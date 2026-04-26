package resolved_ast

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/stretchr/testify/assert"
)

func ptr[T any](v T) *T { return &v }

// TestNode_String verifies the canonical String() output for representative
// resolved-AST nodes. Each leaf case pins down the per-kind scalar suffix;
// the composite case pins down indentation when a node has children.
func TestNode_String(t *testing.T) {
	int64Kind := generated.ValueProto_Int64Value{Int64Value: 42}
	literalProto := &generated.ResolvedLiteralProto{
		Value: &generated.ValueWithTypeProto{
			Value: &generated.ValueProto{Value: &int64Kind},
		},
	}

	tests := []struct {
		name string
		node Node
		want string
	}{
		{
			name: "OutputColumn emits ' <name>'",
			node: newOutputColumnNode(&generated.ResolvedOutputColumnProto{Name: ptr("col1")}),
			want: "KindOutputColumn col1\n",
		},
		{
			name: "TableScan emits ' <table-name>'",
			node: newTableScanNode(&generated.ResolvedTableScanProto{
				Table: &generated.TableRefProto{Name: ptr("users")},
			}),
			want: "KindTableScan users\n",
		},
		{
			name: "Literal emits ' <int64-value>'",
			node: newLiteralNode(literalProto),
			want: "KindLiteral 42\n",
		},
		{
			name: "ProjectScan with TableScan child indents the child by two spaces",
			node: newProjectScanNode(&generated.ResolvedProjectScanProto{
				InputScan: &generated.AnyResolvedScanProto{
					Node: &generated.AnyResolvedScanProto_ResolvedTableScanNode{
						ResolvedTableScanNode: &generated.ResolvedTableScanProto{
							Table: &generated.TableRefProto{Name: ptr("users")},
						},
					},
				},
			}),
			want: "KindProjectScan\n" +
				"  KindTableScan users\n",
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
