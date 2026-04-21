package main

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"google.golang.org/protobuf/reflect/protoreflect"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// primaryInterfaces maps wrapper base names (from AnyResolved<Name>Proto)
// to Go interface names declared in resolved_ast/node.go.
var primaryInterfaces = map[string]string{
	"Statement": "StatementNode",
	"Expr":      "ExprNode",
	"Scan":      "ScanNode",
	"Argument":  "ArgumentNode",
}

// reservedMethodNames are Go method names that conflict with the Node interface.
var reservedMethodNames = map[string]bool{
	"Kind":        true,
	"NumChildren": true,
	"Child":       true,
}

func main() {
	fd := generated.File_zetasql_resolved_ast_resolved_ast_proto
	messages := collectMessages(fd)

	ctx := newAnalysisContext(messages)

	fmt.Printf("Found %d concrete nodes, %d oneof wrappers, %d abstract types\n",
		len(ctx.concreteNodes), len(ctx.oneofWrappers), len(ctx.abstractSet))

	outputDir := resolveOutputDir()
	for _, g := range []struct {
		file string
		tmpl string
		data any
	}{
		{"kind_gen.go", "templates/kind.go.tmpl", ctx.concreteNodes},
		{"nodes_gen.go", "templates/nodes.go.tmpl", ctx.concreteNodes},
		{"wrap_gen.go", "templates/wrap.go.tmpl", ctx.oneofWrappers},
	} {
		if err := generateFile(filepath.Join(outputDir, g.file), g.tmpl, g.data); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating %s: %v\n", g.file, err)
			os.Exit(1)
		}
	}

	fmt.Println("Done!")
}

// --- Analysis context ---

type analysisContext struct {
	wrapperSet    map[string]bool   // proto names of oneof wrappers
	abstractSet   map[string]bool   // proto names of abstract base types
	concreteSet   map[string]bool   // proto names of concrete nodes in this file
	interfaceMap  map[string]string // wrapper base name → Go interface type
	concreteNodes []nodeInfo
	oneofWrappers []wrapperInfo
}

func newAnalysisContext(messages []protoreflect.MessageDescriptor) *analysisContext {
	ctx := &analysisContext{
		wrapperSet:  map[string]bool{},
		abstractSet: map[string]bool{},
		concreteSet: map[string]bool{},
	}

	// Step 1: detect oneof wrappers structurally
	msgByName := map[string]protoreflect.MessageDescriptor{}
	for _, msg := range messages {
		name := string(msg.Name())
		msgByName[name] = msg
		if isOneofWrapper(msg) {
			ctx.wrapperSet[name] = true
		}
	}

	// Step 2: derive abstract set — a message X is abstract if "AnyX" exists as a wrapper
	for name := range ctx.wrapperSet {
		abstractName := strings.TrimPrefix(name, "Any")
		ctx.abstractSet[abstractName] = true
	}

	// Step 3: resolve interface for every wrapper by walking parent chains
	ctx.interfaceMap = ctx.resolveAllInterfaces(msgByName)

	// Step 4: classify messages
	for _, msg := range messages {
		name := string(msg.Name())
		if ctx.wrapperSet[name] {
			if w := ctx.classifyWrapper(msg); w != nil {
				ctx.oneofWrappers = append(ctx.oneofWrappers, *w)
			}
		} else if strings.HasPrefix(name, "Resolved") && strings.HasSuffix(name, "Proto") && !ctx.abstractSet[name] {
			ctx.concreteSet[name] = true
			if n := ctx.classifyNode(msg); n != nil {
				ctx.concreteNodes = append(ctx.concreteNodes, *n)
			}
		}
	}

	// Sort for deterministic output
	slices.SortFunc(ctx.concreteNodes, func(a, b nodeInfo) int {
		return strings.Compare(a.GoName, b.GoName)
	})
	slices.SortFunc(ctx.oneofWrappers, func(a, b wrapperInfo) int {
		return strings.Compare(a.GoName, b.GoName)
	})

	return ctx
}

func (ctx *analysisContext) resolveAllInterfaces(msgByName map[string]protoreflect.MessageDescriptor) map[string]string {
	result := make(map[string]string)

	for wrapperName := range ctx.wrapperSet {
		baseName := strings.TrimPrefix(wrapperName, "AnyResolved")
		baseName = strings.TrimSuffix(baseName, "Proto")

		if iface, ok := primaryInterfaces[baseName]; ok {
			result[baseName] = iface
			continue
		}

		abstractName := "Resolved" + baseName + "Proto"
		if iface := ctx.findInterfaceByParentChain(abstractName, msgByName); iface != "" {
			result[baseName] = iface
		} else {
			result[baseName] = "Node"
		}
	}

	return result
}

func (ctx *analysisContext) findInterfaceByParentChain(protoName string, msgByName map[string]protoreflect.MessageDescriptor) string {
	md, ok := msgByName[protoName]
	if !ok {
		return ""
	}

	cur := md
	for {
		parentField := cur.Fields().ByName("parent")
		if parentField == nil || parentField.Message() == nil {
			return ""
		}
		parentName := string(parentField.Message().Name())

		if strings.HasPrefix(parentName, "Resolved") && strings.HasSuffix(parentName, "Proto") {
			parentBase := strings.TrimPrefix(parentName, "Resolved")
			parentBase = strings.TrimSuffix(parentBase, "Proto")
			if iface, ok := primaryInterfaces[parentBase]; ok {
				return iface
			}
		}

		cur = parentField.Message()
	}
}

func (ctx *analysisContext) interfaceFor(baseName string) string {
	if iface, ok := ctx.interfaceMap[baseName]; ok {
		return iface
	}
	return "Node"
}

func (ctx *analysisContext) determineCategory(md protoreflect.MessageDescriptor) string {
	cur := md
	for {
		parentField := cur.Fields().ByName("parent")
		if parentField == nil || parentField.Message() == nil {
			return "node"
		}
		parentName := string(parentField.Message().Name())
		if strings.HasPrefix(parentName, "Resolved") && strings.HasSuffix(parentName, "Proto") {
			parentBase := strings.TrimPrefix(parentName, "Resolved")
			parentBase = strings.TrimSuffix(parentBase, "Proto")
			if _, ok := primaryInterfaces[parentBase]; ok {
				return strings.ToLower(parentBase[:1]) + parentBase[1:]
			}
		}
		cur = parentField.Message()
	}
}

func markerMethodsForCategory(category string) []string {
	switch category {
	case "statement":
		return []string{"statementNode"}
	case "expr":
		return []string{"exprNode"}
	case "scan":
		return []string{"scanNode"}
	case "argument":
		return []string{"argumentNode"}
	default:
		return nil
	}
}

// --- Message classification ---

type nodeInfo struct {
	ProtoName     string
	GoName        string
	KindName      string
	Fields        []fieldInfo
	Category      string
	HasChildNode  bool
	MarkerMethods []string
}

type fieldInfo struct {
	ProtoName string
	GoName    string
	GoType    string
	IsSlice   bool
	IsNode    bool
	WrapCall  string
}

type wrapperInfo struct {
	ProtoName  string
	GoName     string
	FuncName   string
	ReturnType string
	Variants   []variantInfo
}

type variantInfo struct {
	OneofFieldName string
	GoTypeName     string
	ProtoGetter    string
	WrapExpr       string
	IsWrapper      bool
	WrapFunc       string
}

func collectMessages(fd protoreflect.FileDescriptor) []protoreflect.MessageDescriptor {
	var msgs []protoreflect.MessageDescriptor
	mds := fd.Messages()
	for i := range mds.Len() {
		msgs = append(msgs, mds.Get(i))
	}
	return msgs
}

func isOneofWrapper(md protoreflect.MessageDescriptor) bool {
	if md.Oneofs().Len() != 1 {
		return false
	}
	fields := md.Fields()
	for i := range fields.Len() {
		if fields.Get(i).ContainingOneof() == nil {
			return false
		}
	}
	return true
}

func (ctx *analysisContext) classifyWrapper(md protoreflect.MessageDescriptor) *wrapperInfo {
	oo := md.Oneofs().Get(0)
	protoName := string(md.Name())
	baseName := strings.TrimPrefix(protoName, "AnyResolved")
	baseName = strings.TrimSuffix(baseName, "Proto")

	w := &wrapperInfo{
		ProtoName:  protoName,
		GoName:     baseName,
		FuncName:   "wrap" + baseName,
		ReturnType: ctx.interfaceFor(baseName),
	}

	fields := oo.Fields()
	for i := range fields.Len() {
		if v := ctx.classifyVariant(fields.Get(i)); v != nil {
			w.Variants = append(w.Variants, *v)
		}
	}

	return w
}

func (ctx *analysisContext) classifyVariant(fd protoreflect.FieldDescriptor) *variantInfo {
	fieldName := string(fd.Name())
	goFieldName := snakeToPascal(fieldName)
	msgName := string(fd.Message().Name())

	// Skip abstract nodes
	if ctx.abstractSet[msgName] {
		return nil
	}

	v := &variantInfo{
		OneofFieldName: goFieldName,
		ProtoGetter:    "Get" + goFieldName + "()",
	}

	if ctx.wrapperSet[msgName] {
		innerBase := strings.TrimPrefix(msgName, "AnyResolved")
		innerBase = strings.TrimSuffix(innerBase, "Proto")
		v.IsWrapper = true
		v.WrapFunc = "wrap" + innerBase
		v.WrapExpr = "wrap" + innerBase + "(v)"
		v.GoTypeName = ctx.interfaceFor(innerBase)
	} else {
		nodeName := protoToNodeName(msgName)
		v.GoTypeName = "*" + nodeName
		v.WrapExpr = "new" + nodeName + "(v)"
	}

	return v
}

func (ctx *analysisContext) classifyNode(md protoreflect.MessageDescriptor) *nodeInfo {
	protoName := string(md.Name())
	nodeName := protoToNodeName(protoName)
	kindName := "Kind" + strings.TrimSuffix(nodeName, "Node")
	category := ctx.determineCategory(md)

	n := &nodeInfo{
		ProtoName:     protoName,
		GoName:        nodeName,
		KindName:      kindName,
		Category:      category,
		MarkerMethods: markerMethodsForCategory(category),
	}

	fields := md.Fields()
	for i := range fields.Len() {
		f := fields.Get(i)
		fname := string(f.Name())

		if fname == "parent" {
			continue
		}

		fi := ctx.classifyField(f)
		if reservedMethodNames[fi.GoName] {
			fi.GoName = fi.GoName + "Value"
		}
		n.Fields = append(n.Fields, fi)
		if fi.IsNode {
			n.HasChildNode = true
		}
	}

	return n
}

func (ctx *analysisContext) classifyField(fd protoreflect.FieldDescriptor) fieldInfo {
	fname := string(fd.Name())
	goName := snakeToPascal(fname)
	isSlice := fd.IsList()

	fi := fieldInfo{
		ProtoName: fname,
		GoName:    goName,
		IsSlice:   isSlice,
	}

	switch fd.Kind() {
	case protoreflect.BoolKind:
		fi.GoType = "bool"
		fi.WrapCall = fmt.Sprintf("n.raw.Get%s()", goName)
	case protoreflect.StringKind:
		if isSlice {
			fi.GoType = "[]string"
		} else {
			fi.GoType = "string"
		}
		fi.WrapCall = fmt.Sprintf("n.raw.Get%s()", goName)
	case protoreflect.Int32Kind:
		if isSlice {
			fi.GoType = "[]int32"
			fi.WrapCall = fmt.Sprintf("n.raw.Get%s()", goName)
		} else {
			fi.GoType = "int"
			fi.WrapCall = fmt.Sprintf("int(n.raw.Get%s())", goName)
		}
	case protoreflect.Int64Kind:
		if isSlice {
			fi.GoType = "[]int64"
			fi.WrapCall = fmt.Sprintf("n.raw.Get%s()", goName)
		} else {
			fi.GoType = "int"
			fi.WrapCall = fmt.Sprintf("int(n.raw.Get%s())", goName)
		}
	case protoreflect.EnumKind:
		enumGoName := resolveEnumGoName(fd.Enum())
		if isSlice {
			fi.GoType = "[]generated." + enumGoName
		} else {
			fi.GoType = "generated." + enumGoName
		}
		fi.WrapCall = fmt.Sprintf("n.raw.Get%s()", goName)
	case protoreflect.MessageKind:
		msgName := string(fd.Message().Name())
		if ctx.wrapperSet[msgName] {
			baseName := strings.TrimPrefix(msgName, "AnyResolved")
			baseName = strings.TrimSuffix(baseName, "Proto")
			iface := ctx.interfaceFor(baseName)
			fi.IsNode = true
			if isSlice {
				fi.GoType = "[]" + iface
				fi.WrapCall = fmt.Sprintf("wrap%sSlice(n.raw.Get%s())", baseName, goName)
			} else {
				fi.GoType = iface
				fi.WrapCall = fmt.Sprintf("wrap%s(n.raw.Get%s())", baseName, goName)
			}
		} else if ctx.abstractSet[msgName] {
			fi.GoType = "any"
			fi.WrapCall = fmt.Sprintf("n.raw.Get%s()", goName)
		} else if ctx.concreteSet[msgName] {
			nodeName := protoToNodeName(msgName)
			fi.IsNode = true
			if isSlice {
				fi.GoType = "[]*" + nodeName
				fi.WrapCall = fmt.Sprintf("new%sSlice(n.raw.Get%s())", nodeName, goName)
			} else {
				fi.GoType = "*" + nodeName
				fi.WrapCall = fmt.Sprintf("new%s(n.raw.Get%s())", nodeName, goName)
			}
		} else {
			// External helper type (from serialization.proto, etc.)
			if isSlice {
				fi.GoType = "[]*generated." + msgName
			} else {
				fi.GoType = "*generated." + msgName
			}
			fi.WrapCall = fmt.Sprintf("n.raw.Get%s()", goName)
		}
	case protoreflect.BytesKind:
		fi.GoType = "[]byte"
		fi.WrapCall = fmt.Sprintf("n.raw.Get%s()", goName)
	default:
		fi.GoType = "any"
		fi.WrapCall = fmt.Sprintf("n.raw.Get%s()", goName)
	}

	return fi
}

// --- Helpers ---

func resolveEnumGoName(ed protoreflect.EnumDescriptor) string {
	name := string(ed.Name())
	if md, ok := ed.Parent().(protoreflect.MessageDescriptor); ok {
		return string(md.Name()) + "_" + name
	}
	return name
}

func protoToNodeName(protoName string) string {
	name := strings.TrimPrefix(protoName, "Resolved")
	name = strings.TrimSuffix(name, "Proto")
	return name + "Node"
}

func snakeToPascal(s string) string {
	parts := strings.Split(s, "_")
	var b strings.Builder
	for _, p := range parts {
		if len(p) > 0 {
			b.WriteString(strings.ToUpper(p[:1]) + p[1:])
		}
	}
	return b.String()
}

// --- Code generation ---

func resolveOutputDir() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "wasm", "tools", "resolved_astgen")); err == nil {
			return filepath.Join(dir, "resolved_ast")
		}
		if _, err := os.Stat(filepath.Join(dir, "resolved_ast")); err == nil {
			return filepath.Join(dir, "resolved_ast")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "resolved_ast"
}

func generateFile(path, tmplFile string, data any) error {
	content, err := templateFS.ReadFile(tmplFile)
	if err != nil {
		return fmt.Errorf("read template %s: %w", tmplFile, err)
	}

	tmpl, err := template.New(filepath.Base(tmplFile)).Funcs(template.FuncMap{
		"trimPrefix": strings.TrimPrefix,
	}).Parse(string(content))
	if err != nil {
		return fmt.Errorf("parse template %s: %w", tmplFile, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute template %s: %w", tmplFile, err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		// Write unformatted for debugging
		os.WriteFile(path, buf.Bytes(), 0644)
		return fmt.Errorf("gofmt %s: %w", path, err)
	}

	return os.WriteFile(path, formatted, 0644)
}
