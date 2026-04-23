package zetasql

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/glassmonkey/zetasql-wasm/ast"
	"github.com/glassmonkey/zetasql-wasm/wasm"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/emscripten"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// init validates the WASM file structure
func init() {
	ctx := context.Background()
	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	// Compile the WASM module to verify it's valid
	compiled, err := runtime.CompileModule(ctx, wasm.ZetaSQLWasm)
	if err != nil {
		panic(fmt.Sprintf("failed to compile ZetaSQL WASM module: %v", err))
	}
	defer compiled.Close(ctx)

	// Verify required functions are exported
	requiredFuncs := []string{"init_module", "malloc", "free", "parse_statement_proto", "analyze_statement_proto", "free_proto_buffer"}
	exports := compiled.ExportedFunctions()
	for _, funcName := range requiredFuncs {
		if _, ok := exports[funcName]; !ok {
			panic(fmt.Sprintf("required WASM function '%s' not found in zetasql.wasm", funcName))
		}
	}
}

// Parser represents a ZetaSQL parser instance
type Parser struct {
	runtime wazero.Runtime
	module  api.Module
}

// ParseError represents a SQL parse error returned by ZetaSQL.
type ParseError struct {
	Message string
}

func (e *ParseError) Error() string {
	return e.Message
}

// Statement represents a successfully parsed SQL statement.
type Statement struct {
	sql  string
	root ast.StatementNode
}

// SQL returns the original SQL string.
func (s *Statement) SQL() string { return s.sql }

// RootNode returns the root AST node of the parsed statement.
func (s *Statement) RootNode() ast.StatementNode {
	return s.root
}

// NewParser creates a new ZetaSQL parser instance
func NewParser(ctx context.Context) (*Parser, error) {
	// Create a new WebAssembly runtime
	runtime := wazero.NewRuntime(ctx)

	// Instantiate WASI
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, runtime); err != nil {
		runtime.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate WASI: %w", err)
	}

	// Compile the WASM module
	compiledModule, err := runtime.CompileModule(ctx, wasm.ZetaSQLWasm)
	if err != nil {
		runtime.Close(ctx)
		return nil, fmt.Errorf("failed to compile WASM module: %w", err)
	}

	// Create env module builder
	builder := runtime.NewHostModuleBuilder("env")

	// Add Emscripten functions
	emscriptenExporter, err := emscripten.NewFunctionExporterForModule(compiledModule)
	if err != nil {
		runtime.Close(ctx)
		return nil, fmt.Errorf("failed to create Emscripten exporter: %w", err)
	}
	emscriptenExporter.ExportFunctions(builder)

	// Add missing Emscripten functions that wazero doesn't provide
	builder.NewFunctionBuilder().WithFunc(func(int32, int32, int32) int32 { return 0 }).Export("emscripten_asm_const_int")

	// Add ZetaSQL-specific functions
	builder.NewFunctionBuilder().WithFunc(func() int32 { return 0 }).Export("HaveOffsetConverter")

	// Instantiate env module
	if _, err := builder.Instantiate(ctx); err != nil {
		runtime.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate env module: %w", err)
	}

	// Instantiate the WASM module
	moduleConfig := wazero.NewModuleConfig().WithStartFunctions("_initialize")
	module, err := runtime.InstantiateModule(ctx, compiledModule, moduleConfig)
	if err != nil {
		runtime.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate WASM module: %w", err)
	}

	return &Parser{
		runtime: runtime,
		module:  module,
	}, nil
}

// ParseStatement parses a SQL statement and returns the AST.
// Returns a *ParseError if the SQL is syntactically invalid.
func (p *Parser) ParseStatement(ctx context.Context, sql string) (*Statement, error) {
	if p.module == nil {
		return nil, fmt.Errorf("parser is not initialized")
	}

	// Get exported functions
	malloc := p.module.ExportedFunction("malloc")
	free := p.module.ExportedFunction("free")
	parseProtoFunc := p.module.ExportedFunction("parse_statement_proto")
	freeProtoBuffer := p.module.ExportedFunction("free_proto_buffer")

	// Allocate memory for SQL string in WASM
	sqlBytes := []byte(sql)
	sqlLen := uint64(len(sqlBytes) + 1) // +1 for null terminator

	results, err := malloc.Call(ctx, sqlLen)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate memory: %w", err)
	}
	sqlPtr := results[0]
	defer free.Call(ctx, sqlPtr)

	// Write SQL string to WASM memory
	if !p.module.Memory().Write(uint32(sqlPtr), append(sqlBytes, 0)) {
		return nil, fmt.Errorf("failed to write SQL to WASM memory")
	}

	// Call parse_statement_proto
	results, err = parseProtoFunc.Call(ctx, sqlPtr)
	if err != nil {
		return nil, fmt.Errorf("failed to call parse function: %w", err)
	}
	resultPtr := results[0]
	defer freeProtoBuffer.Call(ctx, resultPtr)

	// Read result from WASM memory
	// Format: [uint32 size][data bytes]
	mem := p.module.Memory()

	// Read size (first 4 bytes)
	sizeBytes, ok := mem.Read(uint32(resultPtr), 4)
	if !ok {
		return nil, fmt.Errorf("failed to read size from WASM memory")
	}
	size := binary.LittleEndian.Uint32(sizeBytes)

	// Read data bytes
	dataBytes, ok := mem.Read(uint32(resultPtr)+4, size)
	if !ok {
		return nil, fmt.Errorf("failed to read data from WASM memory")
	}

	// Check if result is an error string
	dataStr := string(dataBytes)
	if len(dataStr) > 6 && dataStr[:6] == "Error:" {
		return nil, &ParseError{Message: dataStr[7:]}
	}

	// Deserialize proto into AST
	root, err := ast.StatementFromBytes(dataBytes)
	if err != nil {
		return nil, err
	}

	return &Statement{
		sql:  sql,
		root: root,
	}, nil
}

// Close releases resources used by the parser
func (p *Parser) Close(ctx context.Context) error {
	if p.runtime != nil {
		return p.runtime.Close(ctx)
	}
	return nil
}

