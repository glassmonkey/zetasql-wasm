package zetasql

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/glassmonkey/zetasql-wasm/ast"
	"github.com/glassmonkey/zetasql-wasm/wasm"
)

// ParseError represents a SQL parse error returned by ZetaSQL.
type ParseError struct {
	Message string
}

func (e *ParseError) Error() string {
	return e.Message
}

// Statement represents a successfully parsed SQL statement.
type Statement struct {
	SQL  string
	Root ast.StatementNode
}

// Parse parses a SQL statement and returns the AST. Returns a
// *ParseError if the SQL is syntactically invalid.
func (e *Engine) Parse(ctx context.Context, sql string) (*Statement, error) {
	if e.module == nil {
		return nil, fmt.Errorf("engine is not initialized")
	}

	malloc := e.module.ExportedFunction("malloc")
	free := e.module.ExportedFunction("free")
	parseProtoFunc := e.module.ExportedFunction("parse_statement_proto")
	freeProtoBuffer := e.module.ExportedFunction("free_proto_buffer")

	sqlBytes := []byte(sql)
	sqlLen := uint64(len(sqlBytes) + 1)

	results, err := malloc.Call(ctx, sqlLen)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate memory: %w", err)
	}
	sqlPtr := results[0]
	defer func() { _, _ = free.Call(ctx, sqlPtr) }()

	if !e.module.Memory().Write(uint32(sqlPtr), append(sqlBytes, 0)) {
		return nil, fmt.Errorf("failed to write SQL to WASM memory")
	}

	results, err = parseProtoFunc.Call(ctx, sqlPtr)
	if err != nil {
		return nil, fmt.Errorf("failed to call parse function: %w", err)
	}
	resultPtr := results[0]
	defer func() { _, _ = freeProtoBuffer.Call(ctx, resultPtr) }()

	mem := e.module.Memory()

	sizeBytes, ok := mem.Read(uint32(resultPtr), 4)
	if !ok {
		return nil, fmt.Errorf("failed to read size from WASM memory")
	}
	size := binary.LittleEndian.Uint32(sizeBytes)

	dataBytes, ok := mem.Read(uint32(resultPtr)+4, size)
	if !ok {
		return nil, fmt.Errorf("failed to read data from WASM memory")
	}

	if msg := wasm.ParseResultMessage(dataBytes); msg != "" {
		return nil, &ParseError{Message: msg}
	}

	root, err := ast.StatementFromBytes(dataBytes)
	if err != nil {
		return nil, err
	}

	return &Statement{SQL: sql, Root: root}, nil
}
