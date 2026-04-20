package ast

import (
	"fmt"
	"strings"
)

// DebugString returns a human-readable tree representation of the AST.
// Each node is printed with its Kind name, and children are indented.
func DebugString(n Node) string {
	var b strings.Builder
	debugString(&b, n, 0)
	return b.String()
}

func debugString(b *strings.Builder, n Node, depth int) {
	if n == nil {
		return
	}

	indent := strings.Repeat("  ", depth)
	fmt.Fprintf(b, "%s%s", indent, n.Kind())

	// Print scalar fields (non-node fields) inline
	if extras := scalarSummary(n); extras != "" {
		fmt.Fprintf(b, " %s", extras)
	}
	b.WriteByte('\n')

	for i := range n.NumChildren() {
		debugString(b, n.Child(i), depth+1)
	}
}

// scalarSummary returns a short summary of non-node fields for known node types.
func scalarSummary(n Node) string {
	switch v := n.(type) {
	case *IdentifierNode:
		return fmt.Sprintf("[%s]", v.IdString())
	case *IntLiteralNode:
		// IntLiteral has no direct fields (image is in parent chain)
		return ""
	case *StringLiteralNode:
		return fmt.Sprintf("[%s]", v.StringValue())
	case *SelectNode:
		if v.Distinct() {
			return "[DISTINCT]"
		}
	}
	return ""
}
