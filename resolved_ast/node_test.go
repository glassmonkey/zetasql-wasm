package resolved_ast

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestStatementFromBytes(t *testing.T) {
	// Build a simple QueryStmt proto: SELECT with 1 output column and a SingleRowScan
	outputCol := &generated.ResolvedOutputColumnProto{
		Name: proto.String("col1"),
		Column: &generated.ResolvedColumnProto{
			ColumnId:  proto.Int64(1),
			TableName: proto.String(""),
			Name:      proto.String("col1"),
		},
	}
	query := &generated.AnyResolvedScanProto{
		Node: &generated.AnyResolvedScanProto_ResolvedSingleRowScanNode{
			ResolvedSingleRowScanNode: &generated.ResolvedSingleRowScanProto{},
		},
	}
	stmt := &generated.AnyResolvedStatementProto{
		Node: &generated.AnyResolvedStatementProto_ResolvedQueryStmtNode{
			ResolvedQueryStmtNode: &generated.ResolvedQueryStmtProto{
				OutputColumnList: []*generated.ResolvedOutputColumnProto{outputCol},
				Query:            query,
			},
		},
	}

	data, err := proto.Marshal(stmt)
	require.NoError(t, err)

	node, err := StatementFromBytes(data)
	require.NoError(t, err)

	queryStmt, ok := node.(*QueryStmtNode)
	require.True(t, ok, "expected *QueryStmtNode, got %T", node)

	assert.Equal(t, KindQueryStmt, queryStmt.Kind())

	cols := queryStmt.OutputColumnList()
	require.Len(t, cols, 1)
	assert.Equal(t, "col1", cols[0].Name())

	scan := queryStmt.Query()
	require.NotNil(t, scan)
	assert.Equal(t, KindSingleRowScan, scan.Kind())
}

func TestStatementFromBytes_InvalidBytes(t *testing.T) {
	_, err := StatementFromBytes([]byte{0xff, 0xff})
	assert.Error(t, err)
}

// TestBaseFunctionCall_Implementations verifies that the three resolved
// function-call node types all satisfy BaseFunctionCall. This is a
// compile-time-ish check expressed at runtime via type assertion; if a
// future generator change drops one of the methods, this test fails
// loudly instead of letting downstream callers discover the gap.
func TestBaseFunctionCall_Implementations(t *testing.T) {
	tests := []struct {
		name string
		node Node
	}{
		{
			name: "FunctionCallNode",
			node: &FunctionCallNode{raw: &generated.ResolvedFunctionCallProto{}},
		},
		{
			name: "AggregateFunctionCallNode",
			node: &AggregateFunctionCallNode{raw: &generated.ResolvedAggregateFunctionCallProto{}},
		},
		{
			name: "AnalyticFunctionCallNode",
			node: &AnalyticFunctionCallNode{raw: &generated.ResolvedAnalyticFunctionCallProto{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := tt.node

			// Act
			_, ok := sut.(BaseFunctionCall)

			// Assert
			assert.True(t, ok, "%T does not implement BaseFunctionCall", sut)
		})
	}
}

// TestExprType verifies that ExprType pulls the proto Type off concrete
// expression nodes that carry one, and returns nil for nodes that don't.
func TestExprType(t *testing.T) {
	int64Kind := generated.TypeKind_TYPE_INT64
	int64Proto := &generated.TypeProto{TypeKind: &int64Kind}

	tests := []struct {
		name string
		expr ExprNode
		want *generated.TypeProto
	}{
		{
			name: "LiteralNode with Type returns that type",
			expr: &LiteralNode{raw: &generated.ResolvedLiteralProto{
				Parent: &generated.ResolvedExprProto{Type: int64Proto},
			}},
			want: int64Proto,
		},
		{
			name: "LiteralNode without Type returns nil",
			expr: &LiteralNode{raw: &generated.ResolvedLiteralProto{}},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := tt.expr

			// Act
			got := ExprType(sut)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestScanColumnList verifies that ScanColumnList pulls the column list
// off concrete scan nodes.
func TestScanColumnList(t *testing.T) {
	col := &generated.ResolvedColumnProto{
		ColumnId:  proto.Int64(1),
		TableName: proto.String("t"),
		Name:      proto.String("c"),
	}

	tests := []struct {
		name string
		scan ScanNode
		want []*generated.ResolvedColumnProto
	}{
		{
			name: "TableScanNode with one column",
			scan: &TableScanNode{
				raw: &generated.ResolvedTableScanProto{
					Parent: &generated.ResolvedScanProto{
						ColumnList: []*generated.ResolvedColumnProto{col},
					},
				},
			},
			want: []*generated.ResolvedColumnProto{col},
		},
		{
			name: "SingleRowScanNode has no columns",
			scan: &SingleRowScanNode{
				raw: &generated.ResolvedSingleRowScanProto{},
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := tt.scan

			// Act
			got := ScanColumnList(sut)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

