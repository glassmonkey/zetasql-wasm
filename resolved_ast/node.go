package resolved_ast

import (
	"fmt"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"google.golang.org/protobuf/proto"
)

//go:generate go run ../wasm/tools/resolved_astgen/main.go

// Node is the interface implemented by all resolved AST nodes.
type Node interface {
	Kind() Kind
	NumChildren() int
	Child(i int) Node
	// String returns the canonical multi-line string representation of the
	// subtree rooted at this node (see formatNode for the exact shape).
	String() string
}

// StatementFromBytes unmarshals serialized proto bytes into a StatementNode.
func StatementFromBytes(data []byte) (StatementNode, error) {
	p := &generated.AnyResolvedStatementProto{}
	if err := proto.Unmarshal(data, p); err != nil {
		return nil, fmt.Errorf("failed to unmarshal proto: %w", err)
	}
	return wrapStatement(p), nil
}

// StatementNode is the interface for resolved statement nodes.
type StatementNode interface {
	Node
	statementNode()
}

// ExprNode is the interface for resolved expression nodes.
type ExprNode interface {
	Node
	exprNode()
}

// ScanNode is the interface for resolved scan nodes.
type ScanNode interface {
	Node
	scanNode()
}

// ArgumentNode is the interface for resolved argument nodes.
type ArgumentNode interface {
	Node
	argumentNode()
}

// WrapExpr converts a serialized AnyResolvedExprProto oneof into the
// matching concrete ExprNode wrapper. Returns nil if the input is nil
// or the oneof is empty / unrecognised. Useful when a parent node
// surfaces an expression as proto (e.g. ResolvedComputedColumnProto.Expr)
// and downstream code needs the typed Go view.
func WrapExpr(p *generated.AnyResolvedExprProto) ExprNode {
	return wrapExpr(p)
}

// WrapScan converts a serialized AnyResolvedScanProto oneof into the
// matching concrete ScanNode wrapper. Returns nil for nil / empty input.
func WrapScan(p *generated.AnyResolvedScanProto) ScanNode {
	return wrapScan(p)
}

// WrapStatement converts a serialized AnyResolvedStatementProto oneof
// into the matching concrete StatementNode wrapper. Returns nil for
// nil / empty input.
func WrapStatement(p *generated.AnyResolvedStatementProto) StatementNode {
	return wrapStatement(p)
}

// WrapArgument converts a serialized AnyResolvedArgumentProto oneof
// into the matching concrete ArgumentNode wrapper. Returns nil for
// nil / empty input.
func WrapArgument(p *generated.AnyResolvedArgumentProto) ArgumentNode {
	return wrapArgument(p)
}

// NewComputedColumnNode wraps a ResolvedComputedColumnProto as a
// ComputedColumnNode. Some parent nodes (AggregateScan.GroupByList etc.)
// expose the underlying proto directly because the proto schema doesn't
// model it as a oneof; this helper lets callers reach the typed wrapper.
func NewComputedColumnNode(raw *generated.ResolvedComputedColumnProto) *ComputedColumnNode {
	return newComputedColumnNode(raw)
}

// NewAnalyticFunctionGroupNode wraps a ResolvedAnalyticFunctionGroupProto.
func NewAnalyticFunctionGroupNode(raw *generated.ResolvedAnalyticFunctionGroupProto) *AnalyticFunctionGroupNode {
	return newAnalyticFunctionGroupNode(raw)
}

// NewWithEntryNode wraps a ResolvedWithEntryProto.
func NewWithEntryNode(raw *generated.ResolvedWithEntryProto) *WithEntryNode {
	return newWithEntryNode(raw)
}
