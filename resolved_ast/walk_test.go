package resolved_ast

import (
	"errors"
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// buildQueryStmt builds a QueryStmt with output columns and a SingleRowScan.
func buildQueryStmt() *QueryStmtNode {
	return newQueryStmtNode(&generated.ResolvedQueryStmtProto{
		OutputColumnList: []*generated.ResolvedOutputColumnProto{
			{
				Name: proto.String("a"),
				Column: &generated.ResolvedColumnProto{
					ColumnId:  proto.Int64(1),
					TableName: proto.String("t"),
					Name:      proto.String("a"),
				},
			},
			{
				Name: proto.String("b"),
				Column: &generated.ResolvedColumnProto{
					ColumnId:  proto.Int64(2),
					TableName: proto.String("t"),
					Name:      proto.String("b"),
				},
			},
		},
		Query: &generated.AnyResolvedScanProto{
			Node: &generated.AnyResolvedScanProto_ResolvedSingleRowScanNode{
				ResolvedSingleRowScanNode: &generated.ResolvedSingleRowScanProto{},
			},
		},
	})
}

func TestWalk_VisitsAllNodes(t *testing.T) {
	stmt := buildQueryStmt()

	var kinds []Kind
	err := Walk(stmt, func(n Node) error {
		kinds = append(kinds, n.Kind())
		return nil
	})
	require.NoError(t, err)

	want := []Kind{KindQueryStmt, KindOutputColumn, KindOutputColumn, KindSingleRowScan}
	assert.Equal(t, want, kinds)
}

func TestWalk_NilNode(t *testing.T) {
	err := Walk(nil, func(n Node) error {
		t.Error("fn should not be called for nil node")
		return nil
	})
	assert.NoError(t, err)
}

func TestWalk_EarlyStop(t *testing.T) {
	stmt := buildQueryStmt()
	errStop := errors.New("stop")

	count := 0
	err := Walk(stmt, func(n Node) error {
		count++
		if count == 2 {
			return errStop
		}
		return nil
	})
	assert.ErrorIs(t, err, errStop)
	assert.Equal(t, 2, count)
}

func TestNumChildren_And_Child(t *testing.T) {
	stmt := buildQueryStmt()

	assert.Equal(t, 3, stmt.NumChildren())

	// First two children are OutputColumnNodes
	for i := range 2 {
		child := stmt.Child(i)
		require.NotNil(t, child, "Child(%d)", i)
		assert.IsType(t, &OutputColumnNode{}, child, "Child(%d)", i)
	}

	// Third child is the scan
	scan := stmt.Child(2)
	require.NotNil(t, scan)
	assert.Equal(t, KindSingleRowScan, scan.Kind())

	// Out of bounds
	assert.Nil(t, stmt.Child(3))
}
