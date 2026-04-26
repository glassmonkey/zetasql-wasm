package ast

import (
	"fmt"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"google.golang.org/protobuf/proto"
)

// Node is the interface implemented by all AST nodes.
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
	p := &generated.AnyASTStatementProto{}
	if err := proto.Unmarshal(data, p); err != nil {
		return nil, fmt.Errorf("failed to unmarshal proto: %w", err)
	}
	return wrapStatement(p), nil
}

// StatementNode is the interface for statement-level AST nodes.
type StatementNode interface {
	Node
	statementNode()
}

// ExpressionNode is the interface for expression-level AST nodes.
type ExpressionNode interface {
	Node
	expressionNode()
}

// QueryExpressionNode is the interface for query expression nodes (SELECT, UNION, etc).
type QueryExpressionNode interface {
	Node
	queryExpressionNode()
}

// TableExpressionNode is the interface for table expression nodes (FROM clause items).
type TableExpressionNode interface {
	Node
	tableExpressionNode()
}

// LeafNode is the interface for leaf expression nodes (literals, identifiers).
type LeafNode interface {
	ExpressionNode
	leafNode()
}
