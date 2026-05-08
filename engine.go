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
	"google.golang.org/protobuf/proto"
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
// Resolved is the resolved (semantically analyzed) AST; Parsed is the
// parser AST that produced it. Pair the two via NewNodeMap to look up
// parser-side nodes from a resolved node.
type AnalyzeOutput struct {
	Resolved resolved_ast.StatementNode
	Parsed   ast.StatementNode
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

// callAnalyze sends an AnalyzeRequest to the WASM bridge and returns the
// resolved response together with the parser AST proto. The bridge frames
// the success payload as
//
//	[uint32 LE: parsed_size][parsed_bytes: AnyASTStatementProto]
//	[uint32 LE: response_size][response_bytes: AnalyzeResponse]
//
// parsed_size is zero (and parsedProto returns nil) for analyzer paths
// that do not yield a statement-level parser AST, e.g. expression analysis.
func (e *Engine) callAnalyze(
	ctx context.Context,
	request *generated.AnalyzeRequest,
	cat *types.SimpleCatalog,
	opts *AnalyzerOptions,
) (*generated.AnalyzeResponse, *generated.AnyASTStatementProto, error) {
	if cat != nil {
		request.SimpleCatalog = cat.ToProto()
	}
	if opts != nil {
		request.Options = opts.toProto()
	}

	requestBytes, err := proto.Marshal(request)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal AnalyzeRequest: %w", err)
	}

	mallocFn := e.module.ExportedFunction("malloc")
	freeFn := e.module.ExportedFunction("free")
	analyzeFn := e.module.ExportedFunction("analyze_statement_proto")
	freeProtoBuffer := e.module.ExportedFunction("free_proto_buffer")

	reqSize := uint64(len(requestBytes))
	results, err := mallocFn.Call(ctx, reqSize)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to allocate memory: %w", err)
	}
	reqPtr := results[0]
	defer func() { _, _ = freeFn.Call(ctx, reqPtr) }()

	if !e.module.Memory().Write(uint32(reqPtr), requestBytes) {
		return nil, nil, fmt.Errorf("failed to write request to WASM memory")
	}

	results, err = analyzeFn.Call(ctx, reqPtr, reqSize)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to call analyze function: %w", err)
	}
	resultPtr := results[0]
	defer func() { _, _ = freeProtoBuffer.Call(ctx, resultPtr) }()

	mem := e.module.Memory()
	sizeBytes, ok := mem.Read(uint32(resultPtr), 4)
	if !ok {
		return nil, nil, fmt.Errorf("failed to read size from WASM memory")
	}
	size := binary.LittleEndian.Uint32(sizeBytes)

	dataBytes, ok := mem.Read(uint32(resultPtr)+4, size)
	if !ok {
		return nil, nil, fmt.Errorf("failed to read data from WASM memory")
	}

	if msg := wasm.ParseResultMessage(dataBytes); msg != "" {
		return nil, nil, &AnalyzeError{Message: msg}
	}

	parsedBytes, respBytes, err := splitAnalyzePayload(dataBytes)
	if err != nil {
		return nil, nil, err
	}

	var parsedProto *generated.AnyASTStatementProto
	if len(parsedBytes) > 0 {
		parsedProto = &generated.AnyASTStatementProto{}
		if err := proto.Unmarshal(parsedBytes, parsedProto); err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal parsed statement: %w", err)
		}
	}

	response := &generated.AnalyzeResponse{}
	if err := proto.Unmarshal(respBytes, response); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal AnalyzeResponse: %w", err)
	}
	return response, parsedProto, nil
}

// splitAnalyzePayload parses the bridge framing
// [uint32 parsed_size][parsed_bytes][uint32 response_size][response_bytes]
// and returns the two slices in payload order. Each section is validated
// against the remaining buffer length so a truncated frame surfaces as a
// clear error rather than a slice panic.
func splitAnalyzePayload(payload []byte) (parsed, response []byte, err error) {
	if len(payload) < 4 {
		return nil, nil, fmt.Errorf("analyze payload too short for parsed length: %d bytes", len(payload))
	}
	parsedSize := binary.LittleEndian.Uint32(payload[:4])
	rest := payload[4:]
	if uint32(len(rest)) < parsedSize {
		return nil, nil, fmt.Errorf("analyze payload truncated: parsed_size=%d remaining=%d", parsedSize, len(rest))
	}
	parsed = rest[:parsedSize]
	rest = rest[parsedSize:]
	if len(rest) < 4 {
		return nil, nil, fmt.Errorf("analyze payload too short for response length: %d bytes remaining", len(rest))
	}
	respSize := binary.LittleEndian.Uint32(rest[:4])
	rest = rest[4:]
	if uint32(len(rest)) < respSize {
		return nil, nil, fmt.Errorf("analyze payload truncated: response_size=%d remaining=%d", respSize, len(rest))
	}
	response = rest[:respSize]
	return parsed, response, nil
}

// buildOutput converts an AnalyzeResponse and an optional parser AST proto
// into a type-safe AnalyzeOutput. parsedProto is nil when the bridge
// returned only the resolved AST (e.g. expression analysis paths).
func buildOutput(response *generated.AnalyzeResponse, parsedProto *generated.AnyASTStatementProto) (*AnalyzeOutput, error) {
	stmtProto := response.GetResolvedStatement()
	if stmtProto == nil {
		return nil, fmt.Errorf("AnalyzeResponse contains no resolved statement")
	}
	stmtBytes, err := proto.Marshal(stmtProto)
	if err != nil {
		return nil, fmt.Errorf("failed to re-marshal resolved statement: %w", err)
	}
	stmt, err := resolved_ast.StatementFromBytes(stmtBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to convert resolved statement: %w", err)
	}
	out := &AnalyzeOutput{Resolved: stmt}
	if parsedProto != nil {
		parsedBytes, err := proto.Marshal(parsedProto)
		if err != nil {
			return nil, fmt.Errorf("failed to re-marshal parsed statement: %w", err)
		}
		parsed, err := ast.StatementFromBytes(parsedBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to convert parsed statement: %w", err)
		}
		out.Parsed = parsed
	}
	return out, nil
}
