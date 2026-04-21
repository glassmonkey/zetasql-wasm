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
}

// StatementFromBytes unmarshals serialized proto bytes into a StatementNode.
func StatementFromBytes(data []byte) (StatementNode, error) {
	p := &generated.AnyResolvedStatementProto{}
	if err := proto.Unmarshal(data, p); err != nil {
		return nil, fmt.Errorf("failed to unmarshal proto: %w", err)
	}
	return wrapStatement(p), nil
}

// NodeFromBytes unmarshals serialized proto bytes into any Node.
func NodeFromBytes(data []byte) (Node, error) {
	p := &generated.AnyResolvedNodeProto{}
	if err := proto.Unmarshal(data, p); err != nil {
		return nil, fmt.Errorf("failed to unmarshal proto: %w", err)
	}
	return wrapNode(p), nil
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
