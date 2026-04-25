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

func TestNodeFromBytes(t *testing.T) {
	// Wrap a statement inside AnyResolvedNodeProto
	nodeProto := &generated.AnyResolvedNodeProto{
		Node: &generated.AnyResolvedNodeProto_ResolvedStatementNode{
			ResolvedStatementNode: &generated.AnyResolvedStatementProto{
				Node: &generated.AnyResolvedStatementProto_ResolvedQueryStmtNode{
					ResolvedQueryStmtNode: &generated.ResolvedQueryStmtProto{},
				},
			},
		},
	}

	data, err := proto.Marshal(nodeProto)
	require.NoError(t, err)

	node, err := NodeFromBytes(data)
	require.NoError(t, err)

	assert.Equal(t, KindQueryStmt, node.Kind())
}
