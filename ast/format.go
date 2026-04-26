package ast

import (
	"fmt"
	"strings"
)

// formatNode returns the canonical multi-line string representation of the
// AST tree rooted at n. Each line is "<2-space indent>*N<Kind><scalar?>",
// where the scalar suffix is type-specific (e.g., "[users]" for an
// IdentifierNode). This is what every concrete XxxNode.String() returns.
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
	case *IdentifierNode:
		return " [" + v.IdString() + "]"
	case *StringLiteralNode:
		return " [" + v.StringValue() + "]"
	case *IntLiteralNode:
		return " [" + v.raw.GetParent().GetImage() + "]"
	case *FloatLiteralNode:
		return " [" + v.raw.GetParent().GetImage() + "]"
	case *BooleanLiteralNode:
		return " [" + v.raw.GetParent().GetImage() + "]"
	case *NullLiteralNode:
		return " [" + v.raw.GetParent().GetImage() + "]"
	case *MaxLiteralNode:
		return " [" + v.raw.GetParent().GetImage() + "]"
	case *SelectNode:
		if v.Distinct() {
			return " [DISTINCT]"
		}
		return ""
	case *OrderingExpressionNode:
		return " [" + v.OrderingSpec().String() + "]"
	case *SetOperationAllOrDistinctNode:
		return " [" + v.Value().String() + "]"
	case *SetOperationTypeNode:
		return " [" + v.Value().String() + "]"
	}
	return ""
}
