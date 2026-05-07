package resolved_ast

import "github.com/glassmonkey/zetasql-wasm/wasm/generated"

// WindowFrameExpr is the typed Go view of ResolvedWindowFrameExprProto:
// one window-frame boundary (PRECEDING / CURRENT ROW / FOLLOWING) with
// its offset expression for the OFFSET_* boundaries.
//
// A flat DTO sibling to the generated WindowFrameExprNode for the same
// proto. Callers that only need the boundary kind and offset expression
// — typical for SQL formatters and analyzers that read window frames
// without walking the AST — use this shape; callers traversing the
// resolved AST as a Node tree continue to use WindowFrameExprNode.
type WindowFrameExpr struct {
	BoundaryType BoundaryType
	Expression   ExprNode
}

func wrapWindowFrameExpr(p *generated.ResolvedWindowFrameExprProto) *WindowFrameExpr {
	if p == nil {
		return nil
	}
	return &WindowFrameExpr{
		BoundaryType: BoundaryType(p.GetBoundaryType()),
		Expression:   wrapExpr(p.GetExpression()),
	}
}

// WrapWindowFrameExpr lifts a *generated.ResolvedWindowFrameExprProto
// into the typed WindowFrameExpr DTO. Returns nil for nil input.
func WrapWindowFrameExpr(p *generated.ResolvedWindowFrameExprProto) *WindowFrameExpr {
	return wrapWindowFrameExpr(p)
}
