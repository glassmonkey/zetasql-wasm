package zetasql

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/glassmonkey/zetasql-wasm/ast"
	"github.com/glassmonkey/zetasql-wasm/resolved_ast"
	"github.com/glassmonkey/zetasql-wasm/types"
	"github.com/glassmonkey/zetasql-wasm/wasm"
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/emscripten"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// Engine is the single ZetaSQL runtime instance. It owns the wazero
// runtime and the loaded WASM module, and exposes Parse, Analyze, and
// AnalyzeNext as separate methods on the same engine. There is no
// separate parser-only or analyzer-only type — the underlying WASM
// binary contains both, and instantiating it twice would only double
// the memory cost.
type Engine struct {
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

// AnalyzeError represents a semantic analysis error returned by ZetaSQL.
type AnalyzeError struct {
	Message string
}

func (e *AnalyzeError) Error() string {
	return e.Message
}

// Statement represents a successfully parsed SQL statement.
type Statement struct {
	SQL  string
	Root ast.StatementNode
}

// AnalyzeOutput holds the result of a successful semantic analysis.
// Parsed is the parser AST that produced the resolved Statement; pair the
// two via NewNodeMap to look up parser-side nodes from a resolved node.
type AnalyzeOutput struct {
	Statement resolved_ast.StatementNode
	Parsed    ast.StatementNode
}

// New compiles and instantiates the ZetaSQL WASM module, runs its C++
// global constructors via init_module, and returns a ready-to-use
// engine. The caller owns the returned engine and must invoke Close
// to release the runtime when finished.
func New(ctx context.Context) (*Engine, error) {
	runtime := wazero.NewRuntimeWithConfig(ctx, sharedRuntimeConfig())

	if _, err := wasi_snapshot_preview1.Instantiate(ctx, runtime); err != nil {
		_ = runtime.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate WASI: %w", err)
	}

	compiledModule, err := runtime.CompileModule(ctx, wasm.ZetaSQLWasm)
	if err != nil {
		_ = runtime.Close(ctx)
		return nil, fmt.Errorf("failed to compile WASM module: %w", err)
	}

	builder := runtime.NewHostModuleBuilder("env")
	emscriptenExporter, err := emscripten.NewFunctionExporterForModule(compiledModule)
	if err != nil {
		_ = runtime.Close(ctx)
		return nil, fmt.Errorf("failed to create Emscripten exporter: %w", err)
	}
	emscriptenExporter.ExportFunctions(builder)

	builder.NewFunctionBuilder().WithFunc(func(int32, int32, int32) int32 { return 0 }).Export("emscripten_asm_const_int")
	builder.NewFunctionBuilder().WithFunc(func() int32 { return 0 }).Export("HaveOffsetConverter")

	if _, err := builder.Instantiate(ctx); err != nil {
		_ = runtime.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate env module: %w", err)
	}

	module, err := runtime.InstantiateModule(ctx, compiledModule, wazero.NewModuleConfig())
	if err != nil {
		_ = runtime.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate WASM module: %w", err)
	}

	// Run C++ global constructors. Required before any code that depends
	// on abseil global state (e.g. AnalyzerOptions) is exercised.
	if _, err := module.ExportedFunction("init_module").Call(ctx); err != nil {
		_ = runtime.Close(ctx)
		return nil, fmt.Errorf("failed to initialize WASM module: %w", err)
	}

	return &Engine{runtime: runtime, module: module}, nil
}

// Close releases the wazero runtime that backs the engine. After Close
// the engine must not be used again.
func (e *Engine) Close(ctx context.Context) error {
	if e.runtime != nil {
		return e.runtime.Close(ctx)
	}
	return nil
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

// Analyze performs semantic analysis on a SQL statement. Returns an
// *AnalyzeError if the SQL is semantically invalid.
func (e *Engine) Analyze(
	ctx context.Context,
	sql string,
	cat *types.SimpleCatalog,
	opts *AnalyzerOptions,
) (*AnalyzeOutput, error) {
	request := &generated.AnalyzeRequest{
		Target: &generated.AnalyzeRequest_SqlStatement{
			SqlStatement: sql,
		},
	}
	response, parsedProto, err := e.callAnalyze(ctx, request, cat, opts)
	if err != nil {
		return nil, err
	}
	return buildOutput(response, parsedProto)
}

// AnalyzeNext analyzes the next statement from a multi-statement SQL
// string. Returns the analysis output, whether more statements remain,
// and any error. Call repeatedly with the same ParseResumeLocation until
// it returns false.
func (e *Engine) AnalyzeNext(
	ctx context.Context,
	loc *ParseResumeLocation,
	cat *types.SimpleCatalog,
	opts *AnalyzerOptions,
) (*AnalyzeOutput, bool, error) {
	allowResume := true
	request := &generated.AnalyzeRequest{
		Target: &generated.AnalyzeRequest_ParseResumeLocation{
			ParseResumeLocation: &generated.ParseResumeLocationProto{
				Input:        &loc.Input,
				BytePosition: &loc.BytePosition,
				AllowResume:  &allowResume,
			},
		},
	}
	response, parsedProto, err := e.callAnalyze(ctx, request, cat, opts)
	if err != nil {
		return nil, false, err
	}

	if response.ResumeBytePosition != nil {
		loc.BytePosition = response.GetResumeBytePosition()
	} else {
		loc.BytePosition = int32(len(loc.Input))
	}

	output, err := buildOutput(response, parsedProto)
	if err != nil {
		return nil, false, err
	}
	more := int(loc.BytePosition) < len(loc.Input)
	return output, more, nil
}
