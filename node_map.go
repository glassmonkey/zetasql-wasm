package zetasql

import (
	"github.com/glassmonkey/zetasql-wasm/resolved_ast"
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
)

// locatable is satisfied by all generated resolved AST nodes that expose
// their parse location (set when parse_location_record_type is enabled).
type locatable interface {
	ParseLocationRange() *generated.ParseLocationRangeProto
}

type parseLocationKey struct {
	start, end int32
}

// NodeMap maps parse locations to resolved AST nodes, enabling lookup of
// which nodes correspond to a given byte range in the original SQL string.
// Build one by calling NewNodeMap on a statement analyzed with
// PARSE_LOCATION_RECORD_FULL_NODE_SCOPE.
type NodeMap struct {
	nodes map[parseLocationKey][]resolved_ast.Node
}

// NewNodeMap walks the resolved AST and collects every node that has a
// non-nil ParseLocationRange, indexing them by (start, end) byte offsets.
func NewNodeMap(stmt resolved_ast.StatementNode) *NodeMap {
	m := &NodeMap{
		nodes: make(map[parseLocationKey][]resolved_ast.Node),
	}
	resolved_ast.Walk(stmt, func(n resolved_ast.Node) error {
		if loc, ok := n.(locatable); ok {
			r := loc.ParseLocationRange()
			if r != nil && r.Start != nil && r.End != nil {
				key := parseLocationKey{r.GetStart(), r.GetEnd()}
				m.nodes[key] = append(m.nodes[key], n)
			}
		}
		return nil
	})
	return m
}

// NodeAt returns the first node whose parse location exactly matches
// the given [start, end) byte range, or nil if none.
func (m *NodeMap) NodeAt(start, end int32) resolved_ast.Node {
	nodes := m.nodes[parseLocationKey{start, end}]
	if len(nodes) == 0 {
		return nil
	}
	return nodes[0]
}

// NodesAt returns all nodes whose parse location exactly matches
// the given [start, end) byte range.
func (m *NodeMap) NodesAt(start, end int32) []resolved_ast.Node {
	return m.nodes[parseLocationKey{start, end}]
}

// NodesInRange returns all nodes whose parse location is fully contained
// within the given [start, end) byte range.
func (m *NodeMap) NodesInRange(start, end int32) []resolved_ast.Node {
	var result []resolved_ast.Node
	for key, nodes := range m.nodes {
		if key.start >= start && key.end <= end {
			result = append(result, nodes...)
		}
	}
	return result
}
