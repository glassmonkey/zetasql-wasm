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

// TestNodeMap_FindParsedNodes_RecoversTablePath demonstrates the value
// emulator-style callers gain: from a resolved TableScan, recover the
// identifier path the user actually typed in the FROM clause. Without
// this round-trip the caller would have to re-walk the catalog to guess
// at the original path. Got is the slice of identifier strings extracted
// from the parser TablePathExpression sharing the resolved TableScan's
// range; want is the literal SQL text.
func TestNodeMap_FindParsedNodes_RecoversTablePath(t *testing.T) {
	// Arrange
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

	// Act
	var tablePath *ast.TablePathExpressionNode
	for _, n := range sut.FindParsedNodes(tableScan) {
		if tp, ok := n.(*ast.TablePathExpressionNode); ok {
			tablePath = tp
			break
		}
	}
	require.NotNil(t, tablePath, "expected a TablePathExpression among parsed nodes")
	require.NotNil(t, tablePath.PathExpr())
	got := identifierStrings(tablePath.PathExpr().Names())

	// Assert
	assert.Equal(t, []string{"users"}, got)
}

// TestNodeMap_FindParsedNodes_RecoversFunctionPath mirrors the table-path
// case for resolved FunctionCall nodes: the user-typed function name path
// (single-segment for builtins, multi-segment for namespaced UDFs) is
// recoverable through the parser FunctionCall. Got is the identifier
// path; want is the function name as the user typed it.
func TestNodeMap_FindParsedNodes_RecoversFunctionPath(t *testing.T) {
	// Arrange
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

	// Act
	var parsedCall *ast.FunctionCallNode
	for _, n := range sut.FindParsedNodes(fnCall) {
		if c, ok := n.(*ast.FunctionCallNode); ok {
			parsedCall = c
			break
		}
	}
	require.NotNil(t, parsedCall, "expected a FunctionCall among parsed nodes")
	require.NotNil(t, parsedCall.Function())
	got := identifierStrings(parsedCall.Function().Names())

	// Assert
	assert.Equal(t, []string{"UPPER"}, got)
}

// TestNodeMap_FindParsedNodes_AnalyzeNextStatement covers the multi-statement
// bridge path. The single-statement path was rewritten to feed parser
// output into AnalyzeStatementFromParserOutputUnowned; the multi-statement
// path got the same rewrite using ParseNextStatement. Without this case a
// regression that nils out.Parsed only on the AnalyzeNextStatement branch
// would slip past the other tests because resolved-AST checks would still
// pass. Got is the recovered table path from the first of two statements.
func TestNodeMap_FindParsedNodes_AnalyzeNextStatement(t *testing.T) {
	// Arrange
	a := newTestAnalyzer(t)
	cat := newUsersCatalog()
	opts := &AnalyzerOptions{}
	pt := ParseLocationRecordFullNodeScope
	opts.ParseLocationRecordType = &pt
	loc := NewParseResumeLocation("SELECT id FROM users; SELECT name FROM users")
	out, more, err := a.AnalyzeNextStatement(t.Context(), loc, cat, opts)
	require.NoError(t, err)
	require.True(t, more, "expected a second statement to remain")
	require.NotNil(t, out.Parsed, "Parsed must be populated on multi-statement path")
	tableScan := findFirstResolved(out.Statement, resolved_ast.KindTableScan)
	require.NotNil(t, tableScan)
	sut := NewNodeMap(out.Statement, out.Parsed)

	// Act
	var tablePath *ast.TablePathExpressionNode
	for _, n := range sut.FindParsedNodes(tableScan) {
		if tp, ok := n.(*ast.TablePathExpressionNode); ok {
			tablePath = tp
			break
		}
	}
	require.NotNil(t, tablePath)
	require.NotNil(t, tablePath.PathExpr())
	got := identifierStrings(tablePath.PathExpr().Names())

	// Assert
	assert.Equal(t, []string{"users"}, got)
}

// identifierStrings projects a parser identifier slice down to the typed
// strings each node reports. Used by the FindParsedNodes tests so the
// final assertion compares two []string values.
func identifierStrings(ids []*ast.IdentifierNode) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.IdString()
	}
	return out
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
