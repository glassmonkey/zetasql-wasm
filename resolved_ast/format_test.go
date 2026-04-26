package resolved_ast

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/stretchr/testify/assert"
)

func ptr[T any](v T) *T { return &v }

// newLiteralWith wraps a ValueProto into a LiteralNode the way the analyzer
// produces them. Test-side Setup function — keeps each TestNode_String case
// to a single proto-builder line in the table.
func newLiteralWith(v *generated.ValueProto) Node {
	return newLiteralNode(&generated.ResolvedLiteralProto{
		Value: &generated.ValueWithTypeProto{Value: v},
	})
}

// TestNode_String verifies the canonical String() output for representative
// resolved-AST nodes. Each leaf case pins down the per-kind scalar suffix;
// the composite case pins down indentation when a node has children.
func TestNode_String(t *testing.T) {
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
			name: "Literal int64 prints decimal",
			node: newLiteralWith(&generated.ValueProto{
				Value: &generated.ValueProto_Int64Value{Int64Value: 42},
			}),
			want: "KindLiteral 42\n",
		},
		{
			name: "Literal int64 negative prints with sign",
			node: newLiteralWith(&generated.ValueProto{
				Value: &generated.ValueProto_Int64Value{Int64Value: -7},
			}),
			want: "KindLiteral -7\n",
		},
		{
			name: "Literal string is quoted",
			node: newLiteralWith(&generated.ValueProto{
				Value: &generated.ValueProto_StringValue{StringValue: "hello"},
			}),
			want: `KindLiteral "hello"` + "\n",
		},
		{
			name: "Literal bool prints true/false",
			node: newLiteralWith(&generated.ValueProto{
				Value: &generated.ValueProto_BoolValue{BoolValue: true},
			}),
			want: "KindLiteral true\n",
		},
		{
			name: "Literal double prints with %g",
			node: newLiteralWith(&generated.ValueProto{
				Value: &generated.ValueProto_DoubleValue{DoubleValue: 3.14},
			}),
			want: "KindLiteral 3.14\n",
		},
		{
			name: "Literal empty ARRAY prints tag with no children",
			node: newLiteralWith(&generated.ValueProto{
				Value: &generated.ValueProto_ArrayValue{ArrayValue: &generated.ValueProto_Array{}},
			}),
			want: "KindLiteral <ARRAY>\n",
		},
		{
			name: "Literal ARRAY of int64 expands each element",
			node: newLiteralWith(&generated.ValueProto{
				Value: &generated.ValueProto_ArrayValue{ArrayValue: &generated.ValueProto_Array{
					Element: []*generated.ValueProto{
						{Value: &generated.ValueProto_Int64Value{Int64Value: 1}},
						{Value: &generated.ValueProto_Int64Value{Int64Value: 2}},
						{Value: &generated.ValueProto_Int64Value{Int64Value: 3}},
					},
				}},
			}),
			want: `KindLiteral <ARRAY>
  KindLiteral 1
  KindLiteral 2
  KindLiteral 3
`,
		},
		{
			name: "Literal empty STRUCT prints tag with no children",
			node: newLiteralWith(&generated.ValueProto{
				Value: &generated.ValueProto_StructValue{StructValue: &generated.ValueProto_Struct{}},
			}),
			want: "KindLiteral <STRUCT>\n",
		},
		{
			name: "Literal STRUCT expands each field",
			node: newLiteralWith(&generated.ValueProto{
				Value: &generated.ValueProto_StructValue{StructValue: &generated.ValueProto_Struct{
					Field: []*generated.ValueProto{
						{Value: &generated.ValueProto_Int64Value{Int64Value: 1}},
						{Value: &generated.ValueProto_StringValue{StringValue: "x"}},
					},
				}},
			}),
			want: `KindLiteral <STRUCT>
  KindLiteral 1
  KindLiteral "x"
`,
		},
		{
			name: "Literal nested ARRAY of STRUCTs recurses both layers",
			node: newLiteralWith(&generated.ValueProto{
				Value: &generated.ValueProto_ArrayValue{ArrayValue: &generated.ValueProto_Array{
					Element: []*generated.ValueProto{
						{Value: &generated.ValueProto_StructValue{StructValue: &generated.ValueProto_Struct{
							Field: []*generated.ValueProto{
								{Value: &generated.ValueProto_Int64Value{Int64Value: 10}},
							},
						}}},
					},
				}},
			}),
			want: `KindLiteral <ARRAY>
  KindLiteral <STRUCT>
    KindLiteral 10
`,
		},
		{
			name: "Literal with no value oneof set prints NULL",
			node: newLiteralWith(&generated.ValueProto{}),
			want: "KindLiteral NULL\n",
		},
		{
			name: "Literal with nil ValueProto prints NULL",
			node: newLiteralNode(&generated.ResolvedLiteralProto{
				Value: &generated.ValueWithTypeProto{Value: nil},
			}),
			want: "KindLiteral NULL\n",
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
