package ast

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/stretchr/testify/assert"
)

// TestParseLocationRange verifies that generated wrappers expose
// ParseLocationRange by walking the proto Parent chain to ASTNodeProto.
// Cases triangulate three things: a 2-hop chain (Identifier → Expression
// → Node), a 3-hop chain (PathExpression → GeneralizedPathExpression →
// Expression → Node), and a nil-parent path so we can confirm the chain
// short-circuits without panicking when the proto wire omits an
// intermediate parent.
func TestParseLocationRange(t *testing.T) {
	loc := &generated.ParseLocationRangeProto{
		Start: ptr(int32(10)),
		End:   ptr(int32(20)),
	}

	tests := []struct {
		name string
		got  *generated.ParseLocationRangeProto
		want *generated.ParseLocationRangeProto
	}{
		{
			name: "Identifier walks 2 hops to ASTNodeProto",
			got: newIdentifierNode(&generated.ASTIdentifierProto{
				Parent: &generated.ASTExpressionProto{
					Parent: &generated.ASTNodeProto{
						ParseLocationRange: loc,
					},
				},
			}).ParseLocationRange(),
			want: loc,
		},
		{
			name: "PathExpression walks 3 hops to ASTNodeProto",
			got: newPathExpressionNode(&generated.ASTPathExpressionProto{
				Parent: &generated.ASTGeneralizedPathExpressionProto{
					Parent: &generated.ASTExpressionProto{
						Parent: &generated.ASTNodeProto{
							ParseLocationRange: loc,
						},
					},
				},
			}).ParseLocationRange(),
			want: loc,
		},
		{
			name: "Identifier with nil parent chain returns nil without panic",
			got: newIdentifierNode(&generated.ASTIdentifierProto{
				Parent: nil,
			}).ParseLocationRange(),
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.got)
		})
	}
}
