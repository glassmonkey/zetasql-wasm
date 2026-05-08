package zetasql

import (
	"github.com/glassmonkey/zetasql-wasm/ast"
	"github.com/glassmonkey/zetasql-wasm/resolved_ast"
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
)

// locatable is satisfied by all generated AST nodes — both resolved and
// parsed — that expose their parse location.
type locatable interface {
	ParseLocationRange() *generated.ParseLocationRangeProto
}

type parseLocationKey struct {
	start, end int32
}

// NodeMap indexes the resolved and parsed AST of a single statement by
// parse location, enabling forward lookup of resolved nodes from a byte
// range and reverse lookup of the parsed nodes that share that range.
// Build one by calling NewNodeMap on a statement analyzed with
// PARSE_LOCATION_RECORD_FULL_NODE_SCOPE.
type NodeMap struct {
	resolvedByLoc map[parseLocationKey][]resolved_ast.Node
	parsedByLoc   map[parseLocationKey][]ast.Node
}

// NewNodeMap walks both the resolved and parsed AST of the same statement
// and indexes every node that has a non-nil ParseLocationRange by its
// (start, end) byte offsets. The parsed argument may be nil, in which
// case parsed-side lookups return empty results.
func NewNodeMap(resolved resolved_ast.StatementNode, parsed ast.StatementNode) *NodeMap {
	m := &NodeMap{
		resolvedByLoc: make(map[parseLocationKey][]resolved_ast.Node),
		parsedByLoc:   make(map[parseLocationKey][]ast.Node),
	}
	_ = resolved_ast.Walk(resolved, func(n resolved_ast.Node) error {
		if key, ok := locationKey(n); ok {
			m.resolvedByLoc[key] = append(m.resolvedByLoc[key], n)
		}
		return nil
	})
	_ = ast.Walk(parsed, func(n ast.Node) error {
		if key, ok := locationKey(n); ok {
			m.parsedByLoc[key] = append(m.parsedByLoc[key], n)
		}
		return nil
	})
	return m
}

// locationKey extracts the (start, end) parse location key from any node
// that satisfies the locatable interface, returning ok=false when the
// node has no recorded location.
func locationKey(n any) (parseLocationKey, bool) {
	loc, ok := n.(locatable)
	if !ok {
		return parseLocationKey{}, false
	}
	r := loc.ParseLocationRange()
	if r == nil || r.Start == nil || r.End == nil {
		return parseLocationKey{}, false
	}
	return parseLocationKey{r.GetStart(), r.GetEnd()}, true
}

// NodeAt returns the first resolved node whose parse location exactly
// matches the given [start, end) byte range, or nil if none.
func (m *NodeMap) NodeAt(start, end int32) resolved_ast.Node {
	nodes := m.resolvedByLoc[parseLocationKey{start, end}]
	if len(nodes) == 0 {
		return nil
	}
	return nodes[0]
}

// NodesAt returns all resolved nodes whose parse location exactly matches
// the given [start, end) byte range.
func (m *NodeMap) NodesAt(start, end int32) []resolved_ast.Node {
	return m.resolvedByLoc[parseLocationKey{start, end}]
}

// NodesInRange returns all resolved nodes whose parse location is fully
// contained within the given [start, end) byte range.
func (m *NodeMap) NodesInRange(start, end int32) []resolved_ast.Node {
	var result []resolved_ast.Node
	for key, nodes := range m.resolvedByLoc {
		if key.start >= start && key.end <= end {
			result = append(result, nodes...)
		}
	}
	return result
}

// FindParsedNodes returns the parsed AST nodes whose parse location
// matches that of the given resolved node exactly. Multiple nodes can
// share a single (start, end) range when a parser AST chain (for example
// ASTFunctionCall and its enclosing path expression) maps onto the same
// resolved node — callers select among them by Go type. Returns nil when
// no parsed node shares the location, including the cases where the
// resolved node has no recorded location or NewNodeMap was given a nil
// parsed AST.
func (m *NodeMap) FindParsedNodes(resolved resolved_ast.Node) []ast.Node {
	key, ok := locationKey(resolved)
	if !ok {
		return nil
	}
	return m.parsedByLoc[key]
}
