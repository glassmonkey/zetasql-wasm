package zetasql

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/glassmonkey/zetasql-wasm/wasm"
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/emscripten"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"google.golang.org/protobuf/proto"
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
	requiredFuncs := []string{"malloc", "free", "parse_statement", "free_string", "parse_statement_proto", "free_proto_buffer"}
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
	SQL     string                           // Original SQL string
	Parsed  bool                             // Whether parsing succeeded
	Error   string                           // Error message if parsing failed
	AST     string                           // ZetaSQL AST debug string (if parsing succeeded)
	ASTNode *generated.AnyASTStatementProto  // Parsed AST as proto (if parsing succeeded)
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

// ParseStatementProto parses a SQL statement and returns the AST as a proto structure.
// Uses AnalyzeStatement internally to generate a Resolved AST.
// Note: Currently only literal queries (e.g., SELECT 1) work without table definitions.
func (p *Parser) ParseStatementProto(ctx context.Context, sql string) (*Statement, error) {
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
		return nil, fmt.Errorf("failed to parse SQL: %w", err)
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
		return &Statement{
			SQL:     sql,
			Parsed:  false,
			Error:   dataStr[7:], // Remove "Error: " prefix
			AST:     "",
			ASTNode: nil,
		}, nil
	}

	// Deserialize proto
	astNode := &generated.AnyASTStatementProto{}
	if err := proto.Unmarshal(dataBytes, astNode); err != nil {
		return nil, fmt.Errorf("failed to unmarshal proto: %w", err)
	}

	return &Statement{
		SQL:     sql,
		Parsed:  true,
		Error:   "",
		AST:     "", // AST string is not available when using proto output
		ASTNode: astNode,
	}, nil
}

// Close releases resources used by the parser
func (p *Parser) Close(ctx context.Context) error {
	if p.runtime != nil {
		return p.runtime.Close(ctx)
	}
	return nil
}

// DebugTest runs a series of debug tests to identify WASM issues
// Returns a map of test names to results
func (p *Parser) DebugTest(ctx context.Context) (map[string]string, error) {
	if p.module == nil {
		return nil, fmt.Errorf("parser is not initialized")
	}

	results := make(map[string]string)

	debugFuncs := []string{
		"debug_test_basic",
		"debug_test_catalog",
		"debug_test_catalog_with_functions",
		"debug_test_analyzer_options",
		"debug_test_analyze",
	}

	freeString := p.module.ExportedFunction("free_string")

	for _, funcName := range debugFuncs {
		fn := p.module.ExportedFunction(funcName)
		if fn == nil {
			results[funcName] = "NOT EXPORTED (WASM needs rebuild)"
			continue
		}

		callResults, err := fn.Call(ctx)
		if err != nil {
			results[funcName] = fmt.Sprintf("CALL ERROR: %v", err)
			continue
		}

		resultPtr := callResults[0]
		defer freeString.Call(ctx, resultPtr)

		// Read result string from WASM memory
		mem := p.module.Memory()
		resultBytes, ok := mem.Read(uint32(resultPtr), mem.Size()-uint32(resultPtr))
		if !ok {
			results[funcName] = "MEMORY READ ERROR"
			continue
		}

		// Find null terminator
		endIdx := 0
		for i, b := range resultBytes {
			if b == 0 {
				endIdx = i
				break
			}
		}
		results[funcName] = string(resultBytes[:endIdx])
	}

	return results, nil
}
