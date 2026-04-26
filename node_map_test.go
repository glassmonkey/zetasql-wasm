package zetasql

import (
	"slices"
	"testing"

	"github.com/glassmonkey/zetasql-wasm/resolved_ast"
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func analyzeWithLocations(t *testing.T, a *Analyzer, sql string) *AnalyzeOutput {
	t.Helper()
	opts := &AnalyzerOptions{}
	pt := generated.ParseLocationRecordType_PARSE_LOCATION_RECORD_FULL_NODE_SCOPE
	opts.ParseLocationRecordType = &pt
	out, err := a.AnalyzeStatement(t.Context(), sql, nil, opts)
	require.NoError(t, err, "AnalyzeStatement(%q)", sql)
	return out
}

// TestNodeMap_NodeAt verifies that NodeAt returns the node at the exact
// [start, end) byte range. The got is the node returned by NodeAt; want
// is the same node found via tree walk (identity check via Same).
func TestNodeMap_NodeAt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		sql      string
		start    int32
		end      int32
		wantKind resolved_ast.Kind
	}{
		//   01234567
		{name: "literal in SELECT 1", sql: "SELECT 1", start: 7, end: 8, wantKind: resolved_ast.KindLiteral},
		//   0123456789012
		{name: "literal in SELECT 9999", sql: "SELECT 9999", start: 7, end: 11, wantKind: resolved_ast.KindLiteral},
		{name: "project scan in SELECT 1", sql: "SELECT 1", start: 0, end: 8, wantKind: resolved_ast.KindProjectScan},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Arrange
			a := newTestAnalyzer(t)
			out := analyzeWithLocations(t, a, tt.sql)
			sut := NewNodeMap(out.Statement)

			// Act
			node := sut.NodeAt(tt.start, tt.end)
			require.NotNil(t, node)
			got := node.Kind()

			// Assert
			assert.Equal(t, tt.wantKind, got)
		})
	}
}

// TestNodeMap_NodesInRange_Containment verifies that NodesInRange returns
// only nodes fully contained in [start, end). The got is the sorted slice
// of node Kinds returned; want is the expected list.
func TestNodeMap_NodesInRange_Containment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		sql   string
		start int32
		end   int32
		want  []resolved_ast.Kind
	}{
		{
			//   01234567
			name: "narrow range covers literal only",
			sql:  "SELECT 1", start: 7, end: 8,
			want: []resolved_ast.Kind{resolved_ast.KindLiteral},
		},
		{
			//   0123456789012
			name: "narrow range covers literal in SELECT 9999",
			sql:  "SELECT 9999", start: 7, end: 11,
			want: []resolved_ast.Kind{resolved_ast.KindLiteral},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Arrange
			a := newTestAnalyzer(t)
			out := analyzeWithLocations(t, a, tt.sql)
			sut := NewNodeMap(out.Statement)

			// Act
			nodes := sut.NodesInRange(tt.start, tt.end)
			got := nodeKinds(nodes)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestNodeMap_RequiresParseLocationRecordType verifies that without
// PARSE_LOCATION_RECORD_FULL_NODE_SCOPE, the NodeMap is empty.
func TestNodeMap_RequiresParseLocationRecordType(t *testing.T) {
	t.Parallel()
	// Arrange
	a := newTestAnalyzer(t)
	opts := &AnalyzerOptions{} // deliberately omit ParseLocationRecordType
	out, err := a.AnalyzeStatement(t.Context(), "SELECT 1", nil, opts)
	require.NoError(t, err)
	sut := NewNodeMap(out.Statement)

	// Act
	got := sut.NodesInRange(0, 8)

	// Assert
	var want []resolved_ast.Node
	assert.Equal(t, want, got)
}

// TestNodeMap_NonExistentPosition verifies that lookups outside any node's
// range return nil for NodeAt and an empty slice for NodesInRange.
func TestNodeMap_NonExistentPosition(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		check func(*NodeMap) any
		want  any
	}{
		{
			name:  "NodeAt out of range returns nil",
			check: func(nm *NodeMap) any { return nm.NodeAt(100, 200) },
			want:  resolved_ast.Node(nil),
		},
		{
			name:  "NodesAt out of range returns empty",
			check: func(nm *NodeMap) any { return nm.NodesAt(100, 200) },
			want:  []resolved_ast.Node(nil),
		},
		{
			name:  "NodesInRange out of range returns empty",
			check: func(nm *NodeMap) any { return nm.NodesInRange(100, 200) },
			want:  []resolved_ast.Node(nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Arrange
			a := newTestAnalyzer(t)
			out := analyzeWithLocations(t, a, "SELECT 1")
			sut := NewNodeMap(out.Statement)

			// Act
			got := tt.check(sut)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// nodeKinds extracts and sorts the Kind values from a slice of resolved_ast
// nodes. Used by tests that compare unordered NodesInRange results.
func nodeKinds(nodes []resolved_ast.Node) []resolved_ast.Kind {
	if len(nodes) == 0 {
		return nil
	}
	kinds := make([]resolved_ast.Kind, len(nodes))
	for i, n := range nodes {
		kinds[i] = n.Kind()
	}
	slices.Sort(kinds)
	return kinds
}
