package zetasql

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/glassmonkey/zetasql-wasm/catalog"
	"github.com/glassmonkey/zetasql-wasm/resolved_ast"
	"github.com/glassmonkey/zetasql-wasm/wasm"
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/emscripten"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"google.golang.org/protobuf/proto"
)

// AnalyzeError represents a semantic analysis error returned by ZetaSQL.
type AnalyzeError struct {
	Message string
}

func (e *AnalyzeError) Error() string {
	return e.Message
}

// AnalyzeOutput holds the result of a successful semantic analysis.
type AnalyzeOutput struct {
	statement resolved_ast.StatementNode
}

// ResolvedStatement returns the type-safe resolved AST statement node.
func (o *AnalyzeOutput) ResolvedStatement() resolved_ast.StatementNode {
	return o.statement
}

// Analyzer represents a ZetaSQL analyzer instance backed by WASM.
type Analyzer struct {
	runtime wazero.Runtime
	module  api.Module
}

// NewAnalyzer creates a new ZetaSQL analyzer instance.
func NewAnalyzer(ctx context.Context) (*Analyzer, error) {
	runtime := wazero.NewRuntime(ctx)

	if _, err := wasi_snapshot_preview1.Instantiate(ctx, runtime); err != nil {
		runtime.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate WASI: %w", err)
	}

	compiledModule, err := runtime.CompileModule(ctx, wasm.ZetaSQLWasm)
	if err != nil {
		runtime.Close(ctx)
		return nil, fmt.Errorf("failed to compile WASM module: %w", err)
	}

	builder := runtime.NewHostModuleBuilder("env")

	emscriptenExporter, err := emscripten.NewFunctionExporterForModule(compiledModule)
	if err != nil {
		runtime.Close(ctx)
		return nil, fmt.Errorf("failed to create Emscripten exporter: %w", err)
	}
	emscriptenExporter.ExportFunctions(builder)

	builder.NewFunctionBuilder().WithFunc(func(int32, int32, int32) int32 { return 0 }).Export("emscripten_asm_const_int")
	builder.NewFunctionBuilder().WithFunc(func() int32 { return 0 }).Export("HaveOffsetConverter")

	if _, err := builder.Instantiate(ctx); err != nil {
		runtime.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate env module: %w", err)
	}

	moduleConfig := wazero.NewModuleConfig()
	module, err := runtime.InstantiateModule(ctx, compiledModule, moduleConfig)
	if err != nil {
		runtime.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate WASM module: %w", err)
	}

	// Run C++ global constructors via init_module (equivalent to _initialize in reactor mode).
	// Required before using AnalyzerOptions or any code that depends on abseil global state.
	initFn := module.ExportedFunction("init_module")
	if _, err := initFn.Call(ctx); err != nil {
		runtime.Close(ctx)
		return nil, fmt.Errorf("failed to initialize WASM module: %w", err)
	}

	return &Analyzer{
		runtime: runtime,
		module:  module,
	}, nil
}

// AnalyzeStatement performs semantic analysis on a SQL statement.
// Returns an *AnalyzeError if the SQL is semantically invalid.
func (a *Analyzer) AnalyzeStatement(
	ctx context.Context,
	sql string,
	cat *catalog.SimpleCatalog,
	opts *AnalyzerOptions,
) (*AnalyzeOutput, error) {
	request := &generated.AnalyzeRequest{
		Target: &generated.AnalyzeRequest_SqlStatement{
			SqlStatement: sql,
		},
	}
	response, err := a.callAnalyze(ctx, request, cat, opts)
	if err != nil {
		return nil, err
	}
	return a.buildOutput(response)
}

// AnalyzeNextStatement analyzes the next statement from a multi-statement SQL string.
// Returns the analysis output, whether more statements remain, and any error.
// Call repeatedly with the same ParseResumeLocation until it returns false.
func (a *Analyzer) AnalyzeNextStatement(
	ctx context.Context,
	loc *ParseResumeLocation,
	cat *catalog.SimpleCatalog,
	opts *AnalyzerOptions,
) (*AnalyzeOutput, bool, error) {
	allowResume := true
	request := &generated.AnalyzeRequest{
		Target: &generated.AnalyzeRequest_ParseResumeLocation{
			ParseResumeLocation: &generated.ParseResumeLocationProto{
				Input:        &loc.input,
				BytePosition: &loc.bytePosition,
				AllowResume:  &allowResume,
			},
		},
	}
	response, err := a.callAnalyze(ctx, request, cat, opts)
	if err != nil {
		return nil, false, err
	}

	// Update resume position
	if response.ResumeBytePosition != nil {
		loc.bytePosition = response.GetResumeBytePosition()
	} else {
		// No resume position means we consumed everything
		loc.bytePosition = int32(len(loc.input))
	}

	output, err := a.buildOutput(response)
	if err != nil {
		return nil, false, err
	}
	return output, !loc.AtEnd(), nil
}

// callAnalyze sends an AnalyzeRequest to the WASM bridge and returns the response.
func (a *Analyzer) callAnalyze(
	ctx context.Context,
	request *generated.AnalyzeRequest,
	cat *catalog.SimpleCatalog,
	opts *AnalyzerOptions,
) (*generated.AnalyzeResponse, error) {
	if a.module == nil {
		return nil, fmt.Errorf("analyzer is not initialized")
	}

	if cat != nil {
		request.SimpleCatalog = cat.ToProto()
	}
	if opts != nil {
		request.Options = opts.toProto()
	}

	requestBytes, err := proto.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal AnalyzeRequest: %w", err)
	}

	mallocFn := a.module.ExportedFunction("malloc")
	freeFn := a.module.ExportedFunction("free")
	analyzeFn := a.module.ExportedFunction("analyze_statement_proto")
	freeProtoBuffer := a.module.ExportedFunction("free_proto_buffer")

	reqSize := uint64(len(requestBytes))
	results, err := mallocFn.Call(ctx, reqSize)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate memory: %w", err)
	}
	reqPtr := results[0]
	defer freeFn.Call(ctx, reqPtr)

	if !a.module.Memory().Write(uint32(reqPtr), requestBytes) {
		return nil, fmt.Errorf("failed to write request to WASM memory")
	}

	results, err = analyzeFn.Call(ctx, reqPtr, reqSize)
	if err != nil {
		return nil, fmt.Errorf("failed to call analyze function: %w", err)
	}
	resultPtr := results[0]
	defer freeProtoBuffer.Call(ctx, resultPtr)

	mem := a.module.Memory()
	sizeBytes, ok := mem.Read(uint32(resultPtr), 4)
	if !ok {
		return nil, fmt.Errorf("failed to read size from WASM memory")
	}
	size := binary.LittleEndian.Uint32(sizeBytes)

	dataBytes, ok := mem.Read(uint32(resultPtr)+4, size)
	if !ok {
		return nil, fmt.Errorf("failed to read data from WASM memory")
	}

	dataStr := string(dataBytes)
	if len(dataStr) > 6 && dataStr[:6] == "Error:" {
		return nil, &AnalyzeError{Message: dataStr[7:]}
	}

	response := &generated.AnalyzeResponse{}
	if err := proto.Unmarshal(dataBytes, response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal AnalyzeResponse: %w", err)
	}
	return response, nil
}

// buildOutput converts an AnalyzeResponse to a type-safe AnalyzeOutput.
func (a *Analyzer) buildOutput(response *generated.AnalyzeResponse) (*AnalyzeOutput, error) {
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
	return &AnalyzeOutput{statement: stmt}, nil
}

// Close releases resources used by the analyzer.
func (a *Analyzer) Close(ctx context.Context) error {
	if a.runtime != nil {
		return a.runtime.Close(ctx)
	}
	return nil
}
