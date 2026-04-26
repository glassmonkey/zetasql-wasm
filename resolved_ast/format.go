package resolved_ast

import (
	"fmt"
	"strings"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
)

// formatNode returns the canonical multi-line string representation of the
// resolved AST tree rooted at n. Each line is "<2-space indent>*N<Kind><scalar?>",
// where the scalar suffix is type-specific (e.g., " users" for a
// TableScanNode's table name). This is what every concrete XxxNode.String()
// returns.
func formatNode(n Node) string {
	var b strings.Builder
	writeNode(&b, n, 0)
	return b.String()
}

func writeNode(b *strings.Builder, n Node, depth int) {
	if n == nil {
		return
	}
	fmt.Fprintf(b, "%s%s%s\n", strings.Repeat("  ", depth), n.Kind(), nodeScalar(n))
	for i := range n.NumChildren() {
		writeNode(b, n.Child(i), depth+1)
	}
}

func nodeScalar(n Node) string {
	switch v := n.(type) {
	case *OutputColumnNode:
		return " " + v.Name()
	case *TableScanNode:
		return " " + v.Table().GetName()
	case *FunctionCallNode:
		return " " + v.Function().GetName()
	case *LiteralNode:
		return literalScalar(v.Value().GetValue())
	case *ColumnRefNode:
		return " " + v.Column().GetName()
	case *JoinScanNode:
		return " " + v.JoinType().String()
	}
	return ""
}

// literalScalar formats a ValueProto into a printable token. Each scalar
// kind prints its underlying Go value; composite kinds (ARRAY, STRUCT)
// print a tag instead of expanding because the resolved-AST walker treats
// a LiteralNode as a leaf — recursing into the proto would mix two
// different traversal mechanisms in the same output.
//
// A nil ValueProto, or a ValueProto with no oneof set, prints " NULL".
// Unknown kinds fall through to "" so a future proto update is visible
// as a missing scalar suffix without breaking the caller.
func literalScalar(v *generated.ValueProto) string {
	if v == nil {
		return " NULL"
	}
	switch x := v.GetValue().(type) {
	case *generated.ValueProto_Int64Value:
		return fmt.Sprintf(" %d", x.Int64Value)
	case *generated.ValueProto_Int32Value:
		return fmt.Sprintf(" %d", x.Int32Value)
	case *generated.ValueProto_Uint64Value:
		return fmt.Sprintf(" %d", x.Uint64Value)
	case *generated.ValueProto_Uint32Value:
		return fmt.Sprintf(" %d", x.Uint32Value)
	case *generated.ValueProto_BoolValue:
		return fmt.Sprintf(" %t", x.BoolValue)
	case *generated.ValueProto_DoubleValue:
		return fmt.Sprintf(" %g", x.DoubleValue)
	case *generated.ValueProto_FloatValue:
		return fmt.Sprintf(" %g", x.FloatValue)
	case *generated.ValueProto_StringValue:
		return fmt.Sprintf(" %q", x.StringValue)
	case *generated.ValueProto_BytesValue:
		return fmt.Sprintf(" b%q", x.BytesValue)
	case *generated.ValueProto_ArrayValue:
		return " <ARRAY>"
	case *generated.ValueProto_StructValue:
		return " <STRUCT>"
	case nil:
		return " NULL"
	}
	return ""
}
