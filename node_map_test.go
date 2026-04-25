package zetasql

import (
	"context"
	"testing"

	"github.com/glassmonkey/zetasql-wasm/resolved_ast"
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/google/go-cmp/cmp"
)

func analyzeWithLocations(t *testing.T, a *Analyzer, sql string) *AnalyzeOutput {
	t.Helper()
	ctx := context.Background()
	opts := NewAnalyzerOptions()
	opts.SetParseLocationRecordType(generated.ParseLocationRecordType_PARSE_LOCATION_RECORD_FULL_NODE_SCOPE)
	out, err := a.AnalyzeStatement(ctx, sql, nil, opts)
	if err != nil {
		t.Fatalf("AnalyzeStatement(%q): %v", sql, err)
	}
	return out
}

// TestNodeMap_ExactPositions verifies that NodeMap indexes nodes at their
// exact byte positions in the source SQL, enabling precise source mapping.
// Two distinct SQL strings triangulate that positions are computed per-query,
// not hardcoded.
func TestNodeMap_ExactPositions(t *testing.T) {
	a := newTestAnalyzer(t)

	type nodePayload struct {
		Kind         resolved_ast.Kind
		LiteralInt64 int64
	}

	tests := []struct {
		name         string
		sql          string
		scanStart    int32
		scanEnd      int32
		wantScanKind resolved_ast.Kind
		litStart     int32
		litEnd       int32
		wantLiteral  nodePayload
	}{
		{
			//          01234567
			name:         "SELECT 1",
			sql:          "SELECT 1",
			scanStart:    0,
			scanEnd:      8,
			wantScanKind: resolved_ast.KindProjectScan,
			litStart:     7,
			litEnd:       8,
			wantLiteral:  nodePayload{Kind: resolved_ast.KindLiteral, LiteralInt64: 1},
		},
		{
			//          0123456789012
			name:         "SELECT 9999",
			sql:          "SELECT 9999",
			scanStart:    0,
			scanEnd:      11,
			wantScanKind: resolved_ast.KindProjectScan,
			litStart:     7,
			litEnd:       11,
			wantLiteral:  nodePayload{Kind: resolved_ast.KindLiteral, LiteralInt64: 9999},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := analyzeWithLocations(t, a, tt.sql)
			nm := NewNodeMap(out.ResolvedStatement())

			scanNode := nm.NodeAt(tt.scanStart, tt.scanEnd)
			if scanNode == nil {
				t.Fatalf("NodeAt(%d, %d) = nil", tt.scanStart, tt.scanEnd)
			}
			if got, want := scanNode.Kind(), tt.wantScanKind; got != want {
				t.Errorf("scan node kind = %v, want %v", got, want)
			}

			litNode := nm.NodeAt(tt.litStart, tt.litEnd)
			if litNode == nil {
				t.Fatalf("NodeAt(%d, %d) = nil", tt.litStart, tt.litEnd)
			}
			literal := litNode.(*resolved_ast.LiteralNode)
			got := nodePayload{
				Kind:         litNode.Kind(),
				LiteralInt64: literal.Value().GetValue().GetInt64Value(),
			}
			if diff := cmp.Diff(tt.wantLiteral, got); diff != "" {
				t.Errorf("literal node (-want +got):\n%s", diff)
			}
		})
	}
}

// TestNodeMap_NodesInRange_Containment verifies that NodesInRange only returns
// nodes whose [start, end) is fully contained within the query range.
// Two SQL strings with different literal positions triangulate that containment
// logic is position-aware, not fixed.
func TestNodeMap_NodesInRange_Containment(t *testing.T) {
	a := newTestAnalyzer(t)

	tests := []struct {
		name            string
		sql             string
		fullStart       int32
		fullEnd         int32
		wantFullKinds   []resolved_ast.Kind
		narrowStart     int32
		narrowEnd       int32
		wantNarrowKinds []resolved_ast.Kind
	}{
		{
			//          01234567
			name:      "SELECT 1",
			sql:       "SELECT 1",
			fullStart: 0, fullEnd: 8,
			wantFullKinds:   []resolved_ast.Kind{resolved_ast.KindProjectScan, resolved_ast.KindLiteral},
			narrowStart:     7, narrowEnd: 8,
			wantNarrowKinds: []resolved_ast.Kind{resolved_ast.KindLiteral},
		},
		{
			//          0123456789012
			name:      "SELECT 9999",
			sql:       "SELECT 9999",
			fullStart: 0, fullEnd: 11,
			wantFullKinds:   []resolved_ast.Kind{resolved_ast.KindProjectScan, resolved_ast.KindLiteral},
			narrowStart:     7, narrowEnd: 11,
			wantNarrowKinds: []resolved_ast.Kind{resolved_ast.KindLiteral},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := analyzeWithLocations(t, a, tt.sql)
			nm := NewNodeMap(out.ResolvedStatement())

			// Full range should contain all expected node kinds
			allNodes := nm.NodesInRange(tt.fullStart, tt.fullEnd)
			gotFullKinds := make(map[resolved_ast.Kind]bool)
			for _, n := range allNodes {
				gotFullKinds[n.Kind()] = true
			}
			for _, wantKind := range tt.wantFullKinds {
				if !gotFullKinds[wantKind] {
					t.Errorf("NodesInRange(%d, %d) missing %v", tt.fullStart, tt.fullEnd, wantKind)
				}
			}

			// Narrow range should only contain the literal
			narrowNodes := nm.NodesInRange(tt.narrowStart, tt.narrowEnd)
			var gotNarrowKinds []resolved_ast.Kind
			for _, n := range narrowNodes {
				gotNarrowKinds = append(gotNarrowKinds, n.Kind())
			}
			if diff := cmp.Diff(tt.wantNarrowKinds, gotNarrowKinds); diff != "" {
				t.Errorf("NodesInRange(%d, %d) kinds (-want +got):\n%s", tt.narrowStart, tt.narrowEnd, diff)
			}
		})
	}
}

// TestNodeMap_RequiresParseLocationRecordType verifies that without
// PARSE_LOCATION_RECORD_FULL_NODE_SCOPE, the NodeMap is empty because
// the analyzer does not record parse locations.
func TestNodeMap_RequiresParseLocationRecordType(t *testing.T) {
	a := newTestAnalyzer(t)
	ctx := context.Background()

	opts := NewAnalyzerOptions() // deliberately omit SetParseLocationRecordType
	out, err := a.AnalyzeStatement(ctx, "SELECT 1", nil, opts)
	if err != nil {
		t.Fatalf("AnalyzeStatement failed: %v", err)
	}

	nm := NewNodeMap(out.ResolvedStatement())

	type payload struct {
		NodesInRangeCount int
		NodeAtNil         bool
	}
	got := payload{
		NodesInRangeCount: len(nm.NodesInRange(0, 8)),
		NodeAtNil:         nm.NodeAt(0, 8) == nil,
	}
	want := payload{NodesInRangeCount: 0, NodeAtNil: true}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

// TestNodeMap_NonExistentPosition verifies that lookups at positions
// where no node exists return nil / empty.
func TestNodeMap_NonExistentPosition(t *testing.T) {
	a := newTestAnalyzer(t)
	out := analyzeWithLocations(t, a, "SELECT 1")
	nm := NewNodeMap(out.ResolvedStatement())

	type payload struct {
		NodeAtNil         bool
		NodesAtCount      int
		NodesInRangeCount int
	}
	got := payload{
		NodeAtNil:         nm.NodeAt(100, 200) == nil,
		NodesAtCount:      len(nm.NodesAt(100, 200)),
		NodesInRangeCount: len(nm.NodesInRange(100, 200)),
	}
	want := payload{NodeAtNil: true, NodesAtCount: 0, NodesInRangeCount: 0}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}
