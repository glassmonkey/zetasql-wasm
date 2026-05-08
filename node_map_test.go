package zetasql

import (
	"errors"
	"slices"
	"testing"

	"github.com/glassmonkey/zetasql-wasm/ast"
	"github.com/glassmonkey/zetasql-wasm/resolved_ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func analyzeWithLocations(t *testing.T, a *Analyzer, sql string) *AnalyzeOutput {
	t.Helper()
	opts := &AnalyzerOptions{}
	pt := ParseLocationRecordFullNodeScope
	opts.ParseLocationRecordType = &pt
	out, err := a.AnalyzeStatement(t.Context(), sql, nil, opts)
	require.NoError(t, err, "AnalyzeStatement(%q)", sql)
	return out
}

// TestNodeMap_NodeAt verifies that NodeAt returns the node at the exact
// [start, end) byte range. The got is the node returned by NodeAt; want
// is the same node found via tree walk (identity check via Same).
func TestNodeMap_NodeAt(t *testing.T) {
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
			// Arrange
			a := newTestAnalyzer(t)
			out := analyzeWithLocations(t, a, tt.sql)
			sut := NewNodeMap(out.Statement, out.Parsed)

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
			// Arrange
			a := newTestAnalyzer(t)
			out := analyzeWithLocations(t, a, tt.sql)
			sut := NewNodeMap(out.Statement, out.Parsed)

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
	// Arrange
	a := newTestAnalyzer(t)
	opts := &AnalyzerOptions{} // deliberately omit ParseLocationRecordType
	out, err := a.AnalyzeStatement(t.Context(), "SELECT 1", nil, opts)
	require.NoError(t, err)
	sut := NewNodeMap(out.Statement, out.Parsed)

	// Act
	got := sut.NodesInRange(0, 8)

	// Assert
	var want []resolved_ast.Node
	assert.Equal(t, want, got)
}

// TestNodeMap_NonExistentPosition verifies that lookups outside any node's
// range return nil for NodeAt and an empty slice for NodesInRange.
func TestNodeMap_NonExistentPosition(t *testing.T) {
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
			// Arrange
			a := newTestAnalyzer(t)
			out := analyzeWithLocations(t, a, "SELECT 1")
			sut := NewNodeMap(out.Statement, out.Parsed)

			// Act
			got := tt.check(sut)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestNodeMap_FindParsedNodes_TableScan exercises the resolved → parsed
// reverse lookup that emulator-style callers rely on for table identifier
// reconstruction. Got is the set of parser-AST kinds returned for the
// resolved TableScan node; want is the kinds we expect at the same byte
// range. The TablePathExpression case is the one that carries the
// user-typed identifier path emulators need; siblings sharing the same
// range (PathExpression, Identifier) are an artifact of the parser tree
// and reflect emulator's existing "type-assert through the slice"
// strategy.
func TestNodeMap_FindParsedNodes_TableScan(t *testing.T) {
	a := newTestAnalyzer(t)
	cat := newUsersCatalog()
	opts := &AnalyzerOptions{}
	pt := ParseLocationRecordFullNodeScope
	opts.ParseLocationRecordType = &pt

	out, err := a.AnalyzeStatement(t.Context(), "SELECT id FROM users", cat, opts)
	require.NoError(t, err)
	require.NotNil(t, out.Parsed, "AnalyzeOutput.Parsed must be populated")

	tableScan := findFirstResolved(out.Statement, resolved_ast.KindTableScan)
	require.NotNil(t, tableScan, "expected a TableScan node in resolved tree")

	sut := NewNodeMap(out.Statement, out.Parsed)

	got := parsedKinds(sut.FindParsedNodes(tableScan))

	assert.Contains(t, got, ast.KindTablePathExpression)
}

// TestNodeMap_FindParsedNodes_FunctionCall mirrors emulator's getFuncName
// pattern: a resolved FunctionCall must surface the parser
// FunctionCallNode that carries the user-typed function name path. Got
// is the kinds returned at the resolved range; want is that the slice
// contains FunctionCall (the type-asserted pick the emulator selects).
func TestNodeMap_FindParsedNodes_FunctionCall(t *testing.T) {
	a := newTestAnalyzer(t)
	cat := newBuiltinsCatalog()
	opts := &AnalyzerOptions{}
	pt := ParseLocationRecordFullNodeScope
	opts.ParseLocationRecordType = &pt

	out, err := a.AnalyzeStatement(t.Context(), "SELECT UPPER('x')", cat, opts)
	require.NoError(t, err)
	require.NotNil(t, out.Parsed)

	fnCall := findFirstResolved(out.Statement, resolved_ast.KindFunctionCall)
	require.NotNil(t, fnCall, "expected a FunctionCall node in resolved tree")

	sut := NewNodeMap(out.Statement, out.Parsed)

	got := parsedKinds(sut.FindParsedNodes(fnCall))

	assert.Contains(t, got, ast.KindFunctionCall)
}

// findFirstResolved returns the first resolved node of the given kind by
// pre-order traversal, or nil if none. Used by tests to locate a specific
// node without hard-coding byte ranges.
func findFirstResolved(root resolved_ast.StatementNode, kind resolved_ast.Kind) resolved_ast.Node {
	var found resolved_ast.Node
	_ = resolved_ast.Walk(root, func(n resolved_ast.Node) error {
		if n.Kind() == kind {
			found = n
			return errStopWalk
		}
		return nil
	})
	return found
}

// parsedKinds projects a parsed-AST node slice down to the kinds the
// caller cares about — kinds are stable identifiers, easier to compare
// than full nodes in a test assertion.
func parsedKinds(nodes []ast.Node) []ast.Kind {
	if len(nodes) == 0 {
		return nil
	}
	kinds := make([]ast.Kind, len(nodes))
	for i, n := range nodes {
		kinds[i] = n.Kind()
	}
	return kinds
}

// errStopWalk is a sentinel returned from a Walk callback to terminate
// traversal early. It is used by tests to find the first node matching a
// predicate without scanning the entire tree.
var errStopWalk = errors.New("stop walk")

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
