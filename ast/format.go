package ast

import (
	"fmt"
	"strings"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
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
	case *CreateTableStatementNode:
		return createStatementScalar(v.Scope(), v.IsOrReplace(), v.IsIfNotExists())
	}
	return ""
}

// createStatementScalar renders the inherited create-statement flags
// (Scope, IsOrReplace, IsIfNotExists) as a bracketed annotation suitable
// for a node-tree String() suffix. Returns the empty string when every
// flag is at its default so vanilla CREATE TABLE keeps the no-suffix
// output. Multiple flags combine with ", " in the order Scope, OR
// REPLACE, IF NOT EXISTS so the rendering is stable.
func createStatementScalar(scope generated.ASTCreateStatementEnums_Scope, isOrReplace, isIfNotExists bool) string {
	var parts []string
	switch scope {
	case generated.ASTCreateStatementEnums_PRIVATE:
		parts = append(parts, "PRIVATE")
	case generated.ASTCreateStatementEnums_PUBLIC:
		parts = append(parts, "PUBLIC")
	case generated.ASTCreateStatementEnums_TEMPORARY:
		parts = append(parts, "TEMPORARY")
	}
	if isOrReplace {
		parts = append(parts, "OR REPLACE")
	}
	if isIfNotExists {
		parts = append(parts, "IF NOT EXISTS")
	}
	if len(parts) == 0 {
		return ""
	}
	return " [" + strings.Join(parts, ", ") + "]"
}
