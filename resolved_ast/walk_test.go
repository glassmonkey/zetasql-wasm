package resolved_ast

import (
	"errors"
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
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
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	// QueryStmt -> OutputColumn, OutputColumn, SingleRowScan
	want := []Kind{KindQueryStmt, KindOutputColumn, KindOutputColumn, KindSingleRowScan}
	if len(kinds) != len(want) {
		t.Fatalf("visited %d nodes, want %d: %v", len(kinds), len(want), kinds)
	}
	for i, k := range kinds {
		if k != want[i] {
			t.Errorf("kinds[%d] = %v, want %v", i, k, want[i])
		}
	}
}

func TestWalk_NilNode(t *testing.T) {
	err := Walk(nil, func(n Node) error {
		t.Error("fn should not be called for nil node")
		return nil
	})
	if err != nil {
		t.Errorf("Walk(nil) = %v, want nil", err)
	}
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
	if !errors.Is(err, errStop) {
		t.Errorf("Walk error = %v, want %v", err, errStop)
	}
	if count != 2 {
		t.Errorf("visited %d nodes before stop, want 2", count)
	}
}

func TestNumChildren_And_Child(t *testing.T) {
	stmt := buildQueryStmt()

	if got := stmt.NumChildren(); got != 3 {
		t.Errorf("NumChildren() = %d, want 3", got)
	}

	// First two children are OutputColumnNodes
	for i := range 2 {
		child := stmt.Child(i)
		if child == nil {
			t.Fatalf("Child(%d) = nil", i)
		}
		if _, ok := child.(*OutputColumnNode); !ok {
			t.Errorf("Child(%d) type = %T, want *OutputColumnNode", i, child)
		}
	}

	// Third child is the scan
	scan := stmt.Child(2)
	if scan == nil {
		t.Fatal("Child(2) = nil")
	}
	if got := scan.Kind(); got != KindSingleRowScan {
		t.Errorf("Child(2).Kind() = %v, want %v", got, KindSingleRowScan)
	}

	// Out of bounds
	if got := stmt.Child(3); got != nil {
		t.Errorf("Child(3) = %v, want nil", got)
	}
}
