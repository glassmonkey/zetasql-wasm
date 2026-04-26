package resolved_ast

import (
	"fmt"
	"strings"
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
		return fmt.Sprintf(" %d", v.Value().GetValue().GetInt64Value())
	case *ColumnRefNode:
		return " " + v.Column().GetName()
	case *JoinScanNode:
		return " " + v.JoinType().String()
	}
	return ""
}
