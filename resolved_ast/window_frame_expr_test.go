package resolved_ast

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/stretchr/testify/assert"
)

// TestWrapWindowFrameExpr pins the read-side wrap contract: nil-on-nil,
// the BoundaryType field reads through the wrapped enum (PR #29
// pattern), and Expression is delegated to wrapExpr so the OFFSET_*
// boundary's offset expression comes back as a typed ExprNode.
//
// Triangulated across nil, the proto-zero (UNBOUNDED_PRECEDING with no
// expression — what a window frame's start often looks like), CURRENT
// ROW (different boundary, no expression), and OFFSET_PRECEDING (a
// boundary that does carry an expression) so a regression in any one
// of the wirings shows up in the diff.
func TestWrapWindowFrameExpr(t *testing.T) {
	offsetPreceding := generated.ResolvedWindowFrameExprEnums_OFFSET_PRECEDING
	currentRow := generated.ResolvedWindowFrameExprEnums_CURRENT_ROW

	// mkLiteralExpr returns a fresh empty-literal AnyResolvedExprProto on
	// each call. A factory (rather than a function-scope pointer) keeps
	// per-case Arrange independent — no shared mutable proto across cases.
	mkLiteralExpr := func() *generated.AnyResolvedExprProto {
		return &generated.AnyResolvedExprProto{
			Node: &generated.AnyResolvedExprProto_ResolvedLiteralNode{
				ResolvedLiteralNode: &generated.ResolvedLiteralProto{},
			},
		}
	}

	tests := []struct {
		name string
		in   *generated.ResolvedWindowFrameExprProto
		want *WindowFrameExpr
	}{
		{
			name: "nil proto returns nil",
			in:   nil,
			want: nil,
		},
		{
			name: "empty proto yields UNBOUNDED_PRECEDING (zero) and nil Expression",
			in:   &generated.ResolvedWindowFrameExprProto{},
			want: &WindowFrameExpr{BoundaryType: UnboundedPrecedingType},
		},
		{
			name: "CURRENT_ROW boundary, no expression",
			in: &generated.ResolvedWindowFrameExprProto{
				BoundaryType: &currentRow,
			},
			want: &WindowFrameExpr{BoundaryType: CurrentRowType},
		},
		{
			name: "OFFSET_PRECEDING with offset expression",
			in: &generated.ResolvedWindowFrameExprProto{
				BoundaryType: &offsetPreceding,
				Expression:   mkLiteralExpr(),
			},
			want: &WindowFrameExpr{
				BoundaryType: OffsetPrecedingType,
				Expression:   wrapExpr(mkLiteralExpr()),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := tt.in

			// Act
			got := WrapWindowFrameExpr(sut)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}
