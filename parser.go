package zetasql

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/emscripten"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

//go:embed wasm/zetasql.wasm
var zetasqlWasm []byte

// init validates the WASM file structure
func init() {
	ctx := context.Background()
	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	// Compile the WASM module to verify it's valid
	compiled, err := runtime.CompileModule(ctx, zetasqlWasm)
	if err != nil {
		panic(fmt.Sprintf("failed to compile ZetaSQL WASM module: %v", err))
	}
	defer compiled.Close(ctx)

	// Verify required functions are exported
	requiredFuncs := []string{"malloc", "free", "parse_statement", "free_string"}
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

// Statement represents a parsed SQL statement
type Statement struct {
	SQL    string // Original SQL string
	Parsed bool   // Whether parsing succeeded
	Error  string // Error message if parsing failed
	AST    string // ZetaSQL AST debug string (if parsing succeeded)
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
	compiledModule, err := runtime.CompileModule(ctx, zetasqlWasm)
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

// ParseStatement parses a SQL statement using ZetaSQL WASM
func (p *Parser) ParseStatement(ctx context.Context, sql string) (*Statement, error) {
	if p.module == nil {
		return nil, fmt.Errorf("parser is not initialized")
	}

	// Get exported functions (already verified in init())
	malloc := p.module.ExportedFunction("malloc")
	free := p.module.ExportedFunction("free")
	parseFunc := p.module.ExportedFunction("parse_statement")
	freeString := p.module.ExportedFunction("free_string")

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

	// Call parse_statement
	results, err = parseFunc.Call(ctx, sqlPtr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SQL: %w", err)
	}
	resultPtr := results[0]
	defer freeString.Call(ctx, resultPtr)

	// Read result string from WASM memory
	mem := p.module.Memory()
	resultBytes, ok := mem.Read(uint32(resultPtr), mem.Size()-uint32(resultPtr))
	if !ok {
		return nil, fmt.Errorf("failed to read result from WASM memory")
	}

	// Find null terminator
	endIdx := 0
	for i, b := range resultBytes {
		if b == 0 {
			endIdx = i
			break
		}
	}
	result := string(resultBytes[:endIdx])

	// Create statement
	stmt := &Statement{
		SQL:    sql,
		Parsed: true,
		Error:  "",
		AST:    result,
	}

	// Check if result is an error
	if len(result) > 6 && result[:6] == "Error:" {
		stmt.Parsed = false
		stmt.Error = result[7:] // Remove "Error: " prefix
		stmt.AST = ""
	}

	return stmt, nil
}

// Close releases resources used by the parser
func (p *Parser) Close(ctx context.Context) error {
	if p.runtime != nil {
		return p.runtime.Close(ctx)
	}
	return nil
}
