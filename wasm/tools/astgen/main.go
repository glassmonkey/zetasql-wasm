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

// primaryInterfaces maps wrapper base names (from AnyAST<Name>Proto)
// to Go interface names declared in ast/node.go.
// This is the ONLY hardcoded configuration. All other mappings
// (abstract detection, category resolution, sub-wrapper interfaces)
// are derived from the proto structure.
var primaryInterfaces = map[string]string{
	"Statement":       "StatementNode",
	"Expression":      "ExpressionNode",
	"QueryExpression": "QueryExpressionNode",
	"TableExpression": "TableExpressionNode",
	"Leaf":            "LeafNode",
}

// reservedMethodNames are Go method names that conflict with the Node interface.
// Fields with these names get suffixed with "Value" (e.g., Kind → KindValue).
var reservedMethodNames = map[string]bool{
	"Kind":        true,
	"NumChildren": true,
	"Child":       true,
}

func main() {
	fd := generated.File_zetasql_parser_parse_tree_proto
	messages := collectMessages(fd)

	// Phase 1: structural analysis
	// Detect oneof wrappers, abstract types, and resolve interface mappings.
	ctx := newAnalysisContext(messages)

	fmt.Printf("Found %d concrete nodes, %d oneof wrappers, %d abstract types\n",
		len(ctx.concreteNodes), len(ctx.oneofWrappers), len(ctx.abstractSet))

	// Phase 2: generate files
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
	interfaceMap  map[string]string // wrapper base name → Go interface type
	concreteNodes []nodeInfo
	oneofWrappers []wrapperInfo
}

func newAnalysisContext(messages []protoreflect.MessageDescriptor) *analysisContext {
	ctx := &analysisContext{
		wrapperSet:  map[string]bool{},
		abstractSet: map[string]bool{},
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
		} else if strings.HasPrefix(name, "AST") && strings.HasSuffix(name, "Proto") && !ctx.abstractSet[name] {
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

// resolveAllInterfaces determines the Go interface type for each wrapper
// by walking the parent chain of its corresponding abstract type.
//
// Rule: For wrapper AnyAST<X>Proto, find abstract AST<X>Proto, then walk
// its parent chain. The first ancestor that matches a primaryInterfaces
// entry determines the interface. If no match, fall back to "Node".
func (ctx *analysisContext) resolveAllInterfaces(msgByName map[string]protoreflect.MessageDescriptor) map[string]string {
	result := make(map[string]string)

	for wrapperName := range ctx.wrapperSet {
		baseName := strings.TrimPrefix(wrapperName, "AnyAST")
		baseName = strings.TrimSuffix(baseName, "Proto")

		// Check if this wrapper itself is a primary interface
		if iface, ok := primaryInterfaces[baseName]; ok {
			result[baseName] = iface
			continue
		}

		// Walk the parent chain of the corresponding abstract type
		abstractName := "AST" + baseName + "Proto"
		if iface := ctx.findInterfaceByParentChain(abstractName, msgByName); iface != "" {
			result[baseName] = iface
		} else {
			result[baseName] = "Node"
		}
	}

	return result
}

// findInterfaceByParentChain walks the Parent field chain of a proto message
// and returns the Go interface name of the first primaryInterface ancestor found.
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

		// Check if this parent's wrapper base name is a primary interface
		// AST<X>Proto → base name X
		if strings.HasPrefix(parentName, "AST") && strings.HasSuffix(parentName, "Proto") {
			parentBase := strings.TrimPrefix(parentName, "AST")
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

// determineCategory resolves the category of a concrete node by walking
// its parent chain and matching against primaryInterfaces.
func (ctx *analysisContext) determineCategory(md protoreflect.MessageDescriptor) string {
	cur := md
	for {
		parentField := cur.Fields().ByName("parent")
		if parentField == nil || parentField.Message() == nil {
			return "node"
		}
		parentName := string(parentField.Message().Name())
		if strings.HasPrefix(parentName, "AST") && strings.HasSuffix(parentName, "Proto") {
			parentBase := strings.TrimPrefix(parentName, "AST")
			parentBase = strings.TrimSuffix(parentBase, "Proto")
			if _, ok := primaryInterfaces[parentBase]; ok {
				// Category = lowercased first char of base name
				return strings.ToLower(parentBase[:1]) + parentBase[1:]
			}
		}
		cur = parentField.Message()
	}
}

// markerMethodsForCategory returns the marker method names that a node
// of the given category must implement to satisfy its interface.
func markerMethodsForCategory(category string) []string {
	switch category {
	case "statement":
		return []string{"statementNode"}
	case "expression":
		return []string{"expressionNode"}
	case "queryExpression":
		return []string{"queryExpressionNode"}
	case "tableExpression":
		return []string{"tableExpressionNode"}
	case "leaf":
		return []string{"expressionNode", "leafNode"}
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

// isOneofWrapper returns true if the message is a pure oneof wrapper:
// exactly one oneof and no fields outside that oneof.
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
	baseName := strings.TrimPrefix(protoName, "AnyAST")
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
		innerBase := strings.TrimPrefix(msgName, "AnyAST")
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

		if fname == "parent" || fname == "parse_location_range" {
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
		fi.GoType = "string"
		fi.WrapCall = fmt.Sprintf("n.raw.Get%s()", goName)
	case protoreflect.Int32Kind, protoreflect.Int64Kind:
		fi.GoType = "int"
		fi.WrapCall = fmt.Sprintf("int(n.raw.Get%s())", goName)
	case protoreflect.EnumKind:
		enumGoName := resolveEnumGoName(fd.Enum())
		fi.GoType = "generated." + enumGoName
		fi.WrapCall = fmt.Sprintf("n.raw.Get%s()", goName)
	case protoreflect.MessageKind:
		msgName := string(fd.Message().Name())
		if ctx.wrapperSet[msgName] {
			// Oneof wrapper → returns interface
			baseName := strings.TrimPrefix(msgName, "AnyAST")
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
			// Abstract node reference — not a child node
			fi.GoType = "any"
			fi.WrapCall = fmt.Sprintf("n.raw.Get%s()", goName)
		} else if strings.HasPrefix(msgName, "AST") && strings.HasSuffix(msgName, "Proto") {
			// Concrete node
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
			fi.GoType = "any"
			fi.WrapCall = fmt.Sprintf("n.raw.Get%s()", goName)
		}
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
	name := strings.TrimPrefix(protoName, "AST")
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
		if _, err := os.Stat(filepath.Join(dir, "wasm", "tools", "astgen")); err == nil {
			return filepath.Join(dir, "ast")
		}
		if _, err := os.Stat(filepath.Join(dir, "ast")); err == nil {
			return filepath.Join(dir, "ast")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "ast"
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
		_ = os.WriteFile(path, buf.Bytes(), 0644)
		return fmt.Errorf("gofmt %s: %w", path, err)
	}

	return os.WriteFile(path, formatted, 0644)
}
