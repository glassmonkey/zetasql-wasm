package resolved_ast

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
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
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	node, err := StatementFromBytes(data)
	if err != nil {
		t.Fatalf("StatementFromBytes: %v", err)
	}

	queryStmt, ok := node.(*QueryStmtNode)
	if !ok {
		t.Fatalf("expected *QueryStmtNode, got %T", node)
	}

	if got := queryStmt.Kind(); got != KindQueryStmt {
		t.Errorf("Kind() = %v, want %v", got, KindQueryStmt)
	}

	cols := queryStmt.OutputColumnList()
	if len(cols) != 1 {
		t.Fatalf("OutputColumnList() len = %d, want 1", len(cols))
	}
	if got := cols[0].Name(); got != "col1" {
		t.Errorf("OutputColumnList()[0].Name() = %q, want %q", got, "col1")
	}

	scan := queryStmt.Query()
	if scan == nil {
		t.Fatal("Query() returned nil")
	}
	if got := scan.Kind(); got != KindSingleRowScan {
		t.Errorf("Query().Kind() = %v, want %v", got, KindSingleRowScan)
	}
}

func TestStatementFromBytes_InvalidBytes(t *testing.T) {
	_, err := StatementFromBytes([]byte{0xff, 0xff})
	if err == nil {
		t.Error("expected error for invalid bytes, got nil")
	}
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
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	node, err := NodeFromBytes(data)
	if err != nil {
		t.Fatalf("NodeFromBytes: %v", err)
	}

	if got := node.Kind(); got != KindQueryStmt {
		t.Errorf("Kind() = %v, want %v", got, KindQueryStmt)
	}
}
