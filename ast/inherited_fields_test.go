package ast

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

// TestCreateTableStatementNode_InheritedScalars locks the contract that
// the AST wrapper's parent-chain walk surfaces scalar fields living on
// ASTCreateStatementProto (the depth-2 ancestor of
// ASTCreateTableStatementProto) as accessors on the leaf node.
//
// IsOrReplace, IsIfNotExists, and Scope do not appear in the tree
// String() output (no nodeScalar entry for CreateTableStatementNode),
// so an integration test through Engine.Parse cannot observe them
// — this unit test constructs the proto chain directly via proto
// literals and asserts each accessor returns the expected value
// without going through the WASM analyzer at all.
//
// Triangulated across vanilla / OR REPLACE / IF NOT EXISTS / TEMPORARY
// scope so a regression that breaks one inherited field independently
// surfaces as a slice diff in the bundled flags struct.
func TestCreateTableStatementNode_InheritedScalars(t *testing.T) {
	type flags struct {
		IsOrReplace   bool
		IsIfNotExists bool
		Scope         generated.ASTCreateStatementEnums_Scope
	}

	tests := []struct {
		name string
		raw  *generated.ASTCreateTableStatementProto
		want flags
	}{
		{
			name: "vanilla CREATE TABLE",
			raw:  &generated.ASTCreateTableStatementProto{},
			want: flags{Scope: generated.ASTCreateStatementEnums_DEFAULT_SCOPE},
		},
		{
			name: "OR REPLACE",
			raw: &generated.ASTCreateTableStatementProto{
				Parent: &generated.ASTCreateTableStmtBaseProto{
					Parent: &generated.ASTCreateStatementProto{
						IsOrReplace: proto.Bool(true),
					},
				},
			},
			want: flags{IsOrReplace: true, Scope: generated.ASTCreateStatementEnums_DEFAULT_SCOPE},
		},
		{
			name: "IF NOT EXISTS",
			raw: &generated.ASTCreateTableStatementProto{
				Parent: &generated.ASTCreateTableStmtBaseProto{
					Parent: &generated.ASTCreateStatementProto{
						IsIfNotExists: proto.Bool(true),
					},
				},
			},
			want: flags{IsIfNotExists: true, Scope: generated.ASTCreateStatementEnums_DEFAULT_SCOPE},
		},
		{
			name: "TEMPORARY scope",
			raw: &generated.ASTCreateTableStatementProto{
				Parent: &generated.ASTCreateTableStmtBaseProto{
					Parent: &generated.ASTCreateStatementProto{
						Scope: generated.ASTCreateStatementEnums_TEMPORARY.Enum(),
					},
				},
			},
			want: flags{Scope: generated.ASTCreateStatementEnums_TEMPORARY},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := newCreateTableStatementNode(tt.raw)

			// Act
			got := flags{
				IsOrReplace:   sut.IsOrReplace(),
				IsIfNotExists: sut.IsIfNotExists(),
				Scope:         sut.Scope(),
			}

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}
