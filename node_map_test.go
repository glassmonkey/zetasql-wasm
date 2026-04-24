package zetasql

import (
	"context"
	"testing"

	"github.com/glassmonkey/zetasql-wasm/resolved_ast"
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
)

func TestNodeMap_SelectLiteral(t *testing.T) {
	ctx := context.Background()
	analyzer, err := NewAnalyzer(ctx)
	if err != nil {
		t.Fatalf("Failed to create analyzer: %v", err)
	}
	defer analyzer.Close(ctx)

	sql := "SELECT 1"
	opts := NewAnalyzerOptions()
	opts.SetParseLocationRecordType(generated.ParseLocationRecordType_PARSE_LOCATION_RECORD_FULL_NODE_SCOPE)

	output, err := analyzer.AnalyzeStatement(ctx, sql, nil, opts)
	if err != nil {
		t.Fatalf("AnalyzeStatement failed: %v", err)
	}

	nm := NewNodeMap(output.ResolvedStatement())

	// NodesInRange should return nodes with locations
	nodes := nm.NodesInRange(0, int32(len(sql)))
	if got := len(nodes); got == 0 {
		t.Fatal("NodesInRange(0, 8) returned 0 nodes")
	}

	// "SELECT 1" should have a ProjectScan and a Literal with locations
	kindSet := make(map[resolved_ast.Kind]bool)
	for _, n := range nodes {
		kindSet[n.Kind()] = true
		if loc, ok := n.(locatable); ok {
			r := loc.ParseLocationRange()
			if r != nil {
				if r.GetStart() < 0 || r.GetEnd() > int32(len(sql)) {
					t.Errorf("node %v at [%d, %d) is outside [0, %d)", n.Kind(), r.GetStart(), r.GetEnd(), len(sql))
				}
			}
		}
	}
	if !kindSet[resolved_ast.KindProjectScan] {
		t.Errorf("expected KindProjectScan in NodesInRange, got kinds: %v", kindSet)
	}
	if !kindSet[resolved_ast.KindLiteral] {
		t.Errorf("expected KindLiteral in NodesInRange, got kinds: %v", kindSet)
	}

	// NodeAt should find the ProjectScan at the full statement range
	node := nm.NodeAt(0, int32(len(sql)))
	if node == nil {
		t.Fatal("NodeAt(0, 8) = nil")
	}
	if got, want := node.Kind(), resolved_ast.KindProjectScan; got != want {
		t.Errorf("NodeAt(0, 8).Kind() = %v, want %v", got, want)
	}
}

func TestNodeMap_NoLocationWithoutRecordType(t *testing.T) {
	ctx := context.Background()
	analyzer, err := NewAnalyzer(ctx)
	if err != nil {
		t.Fatalf("Failed to create analyzer: %v", err)
	}
	defer analyzer.Close(ctx)

	// Without PARSE_LOCATION_RECORD_FULL_NODE_SCOPE, nodes should not have locations
	opts := NewAnalyzerOptions()
	output, err := analyzer.AnalyzeStatement(ctx, "SELECT 1", nil, opts)
	if err != nil {
		t.Fatalf("AnalyzeStatement failed: %v", err)
	}

	nm := NewNodeMap(output.ResolvedStatement())
	nodes := nm.NodesInRange(0, 8)
	if got, want := len(nodes), 0; got != want {
		t.Errorf("len(NodesInRange) = %d, want %d (no locations without record type)", got, want)
	}
}

func TestNodeMap_NodesAt(t *testing.T) {
	ctx := context.Background()
	analyzer, err := NewAnalyzer(ctx)
	if err != nil {
		t.Fatalf("Failed to create analyzer: %v", err)
	}
	defer analyzer.Close(ctx)

	sql := "SELECT 1"
	opts := NewAnalyzerOptions()
	opts.SetParseLocationRecordType(generated.ParseLocationRecordType_PARSE_LOCATION_RECORD_FULL_NODE_SCOPE)

	output, err := analyzer.AnalyzeStatement(ctx, sql, nil, opts)
	if err != nil {
		t.Fatalf("AnalyzeStatement failed: %v", err)
	}

	nm := NewNodeMap(output.ResolvedStatement())

	// Non-existent position should return nil
	nodes := nm.NodesAt(100, 200)
	if got, want := len(nodes), 0; got != want {
		t.Errorf("NodesAt(100, 200) returned %d nodes, want %d", got, want)
	}

	// NodeAt for non-existent position
	node := nm.NodeAt(100, 200)
	if node != nil {
		t.Errorf("NodeAt(100, 200) = %v, want nil", node.Kind())
	}
}
