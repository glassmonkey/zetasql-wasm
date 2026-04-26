package ast

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/stretchr/testify/assert"
)

func ptr[T any](v T) *T { return &v }

// TestNode_String verifies the canonical String() output for representative
// node shapes. The leaf cases pin down the per-kind scalar suffix (or lack
// thereof); the composite case pins down indentation across child levels.
func TestNode_String(t *testing.T) {
	tests := []struct {
		name string
		node Node
		want string
	}{
		{
			name: "IntLiteral emits [<image>]",
			node: newIntLiteralNode(&generated.ASTIntLiteralProto{
				Parent: &generated.ASTPrintableLeafProto{Image: ptr("42")},
			}),
			want: "KindIntLiteral [42]\n",
		},
		{
			name: "FloatLiteral emits [<image>]",
			node: newFloatLiteralNode(&generated.ASTFloatLiteralProto{
				Parent: &generated.ASTPrintableLeafProto{Image: ptr("3.14")},
			}),
			want: "KindFloatLiteral [3.14]\n",
		},
		{
			name: "BooleanLiteral emits [<image>]",
			node: newBooleanLiteralNode(&generated.ASTBooleanLiteralProto{
				Parent: &generated.ASTPrintableLeafProto{Image: ptr("true")},
			}),
			want: "KindBooleanLiteral [true]\n",
		},
		{
			name: "NullLiteral emits [<image>]",
			node: newNullLiteralNode(&generated.ASTNullLiteralProto{
				Parent: &generated.ASTPrintableLeafProto{Image: ptr("NULL")},
			}),
			want: "KindNullLiteral [NULL]\n",
		},
		{
			name: "Identifier emits [<id>]",
			node: newIdentifierNode(&generated.ASTIdentifierProto{IdString: ptr("users")}),
			want: "KindIdentifier [users]\n",
		},
		{
			name: "StringLiteral emits [<value>]",
			node: newStringLiteralNode(&generated.ASTStringLiteralProto{StringValue: ptr("hello")}),
			want: "KindStringLiteral [hello]\n",
		},
		{
			name: "QueryStatement with Query child indents the child by two spaces",
			node: newQueryStatementNode(&generated.ASTQueryStatementProto{
				Query: &generated.ASTQueryProto{},
			}),
			want: "KindQueryStatement\n" +
				"  KindQuery\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := tt.node

			// Act
			got := sut.String()

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}
