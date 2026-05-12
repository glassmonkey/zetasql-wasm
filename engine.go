package zetasql

import (
	"context"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"

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
	request := &generated.ParseRequest{
		Target:  &generated.ParseRequest_SqlStatement{SqlStatement: sql},
		Options: parseRequestLanguageOptions(),
	}
	requestBytes, err := proto.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ParseRequest: %w", err)
	}

	mallocFn := e.module.ExportedFunction("malloc")
	freeFn := e.module.ExportedFunction("free")
	parseProtoFunc := e.module.ExportedFunction("parse_statement_proto")
	freeProtoBuffer := e.module.ExportedFunction("free_proto_buffer")

	reqSize := uint64(len(requestBytes))
	results, err := mallocFn.Call(ctx, reqSize)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate memory: %w", err)
	}
	reqPtr := results[0]
	defer func() { _, _ = freeFn.Call(ctx, reqPtr) }()

	if !e.module.Memory().Write(uint32(reqPtr), requestBytes) {
		return nil, fmt.Errorf("failed to write request to WASM memory")
	}

	results, err = parseProtoFunc.Call(ctx, reqPtr, reqSize)
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
//
// Before reaching the C++ resolver the SQL is run through the
// source-rewriting passes in the ast package (currently:
// RewriteNamedTimezoneLiterals, which adapts named-IANA TIMESTAMP
// literals to the numeric-offset form the bundled WASM accepts).
// Skipping the extra parse on inputs that cannot trigger any pass
// keeps the common case free of overhead — see preprocessForAnalyze.
func (e *Engine) Analyze(
	ctx context.Context,
	sql string,
	cat *types.SimpleCatalog,
	opts *AnalyzerOptions,
) (*AnalyzeOutput, error) {
	// resolverSQL is what the C++ resolver actually sees and what its
	// parse-location offsets point into; the source-rewriting pass may
	// shift positions relative to the caller's sql, so pass this same
	// string to rejectInvalidLiteralCasts below.
	resolverSQL := e.preprocessForAnalyze(ctx, sql)
	request := &generated.AnalyzeRequest{
		Target: &generated.AnalyzeRequest_SqlStatement{
			SqlStatement: resolverSQL,
		},
	}
	response, parsedProto, err := e.callAnalyze(ctx, request, cat, opts)
	if err != nil {
		return nil, err
	}
	output, err := buildOutput(response, parsedProto)
	if err != nil {
		return nil, err
	}
	if opts != nil && opts.RejectInvalidLiteralCasts {
		if err := rejectInvalidLiteralCasts(output, resolverSQL); err != nil {
			return nil, err
		}
	}
	return output, nil
}

// preprocessForAnalyze runs the ast package's source-rewriting passes
// on sql so the C++ resolver gets input it can handle. A cheap text
// gate ("/" must appear) skips the extra parser invocation when the
// SQL cannot contain a named-zone literal — the only pass currently
// gated this way. Parse failures fall through with the original sql
// so the downstream callAnalyze can surface its native diagnostic.
func (e *Engine) preprocessForAnalyze(ctx context.Context, sql string) string {
	if !strings.Contains(sql, "/") {
		return sql
	}
	stmt, err := e.Parse(ctx, sql)
	if err != nil {
		return sql
	}
	return ast.RewriteNamedTimezoneLiterals(stmt.Root, sql)
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
	if opts != nil && opts.RejectInvalidLiteralCasts {
		if err := rejectInvalidLiteralCasts(output, loc.Input); err != nil {
			return nil, false, err
		}
	}
	more := int(loc.BytePosition) < len(loc.Input)
	return output, more, nil
}

// ParseNext parses the next statement from a multi-statement SQL string,
// using the given ParseResumeLocation to track position. Returns the
// parsed statement, whether more statements remain, and any error. Call
// repeatedly with the same ParseResumeLocation until it returns false.
// Symmetric to AnalyzeNext but skips semantic analysis.
func (e *Engine) ParseNext(
	ctx context.Context,
	loc *ParseResumeLocation,
) (*Statement, bool, error) {
	request := &generated.ParseRequest{
		Target: &generated.ParseRequest_ParseResumeLocation{
			ParseResumeLocation: &generated.ParseResumeLocationProto{
				Input:        &loc.Input,
				BytePosition: &loc.BytePosition,
			},
		},
		Options: parseRequestLanguageOptions(),
	}
	requestBytes, err := proto.Marshal(request)
	if err != nil {
		return nil, false, fmt.Errorf("failed to marshal ParseRequest: %w", err)
	}

	mallocFn := e.module.ExportedFunction("malloc")
	freeFn := e.module.ExportedFunction("free")
	parseNextFn := e.module.ExportedFunction("parse_next_statement_proto")
	freeProtoBuffer := e.module.ExportedFunction("free_proto_buffer")

	reqSize := uint64(len(requestBytes))
	results, err := mallocFn.Call(ctx, reqSize)
	if err != nil {
		return nil, false, fmt.Errorf("failed to allocate memory: %w", err)
	}
	reqPtr := results[0]
	defer func() { _, _ = freeFn.Call(ctx, reqPtr) }()

	if !e.module.Memory().Write(uint32(reqPtr), requestBytes) {
		return nil, false, fmt.Errorf("failed to write request to WASM memory")
	}

	results, err = parseNextFn.Call(ctx, reqPtr, reqSize)
	if err != nil {
		return nil, false, fmt.Errorf("failed to call parse_next_statement_proto: %w", err)
	}
	resultPtr := results[0]
	defer func() { _, _ = freeProtoBuffer.Call(ctx, resultPtr) }()

	mem := e.module.Memory()
	sizeBytes, ok := mem.Read(uint32(resultPtr), 4)
	if !ok {
		return nil, false, fmt.Errorf("failed to read size from WASM memory")
	}
	size := binary.LittleEndian.Uint32(sizeBytes)

	dataBytes, ok := mem.Read(uint32(resultPtr)+4, size)
	if !ok {
		return nil, false, fmt.Errorf("failed to read data from WASM memory")
	}

	if msg := wasm.ParseResultMessage(dataBytes); msg != "" {
		return nil, false, &ParseError{Message: msg}
	}

	response := &generated.ParseResponse{}
	if err := proto.Unmarshal(dataBytes, response); err != nil {
		return nil, false, fmt.Errorf("failed to unmarshal ParseResponse: %w", err)
	}

	parsedProto := response.GetParsedStatement()
	if parsedProto == nil {
		return nil, false, fmt.Errorf("ParseResponse contains no parsed_statement")
	}

	if response.ResumeBytePosition != nil {
		loc.BytePosition = response.GetResumeBytePosition()
	} else {
		loc.BytePosition = int32(len(loc.Input))
	}

	parsedBytes, err := proto.Marshal(parsedProto)
	if err != nil {
		return nil, false, fmt.Errorf("failed to re-marshal parsed statement: %w", err)
	}
	root, err := ast.StatementFromBytes(parsedBytes)
	if err != nil {
		return nil, false, err
	}

	more := int(loc.BytePosition) < len(loc.Input)
	return &Statement{SQL: loc.Input, Root: root}, more, nil
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
	// Synthesize a default catalog when the caller passes nil so the rest
	// of the pipeline always sees a real *types.SimpleCatalog and the WASM
	// bridge takes a single proto-driven path.
	if cat == nil {
		cat = types.NewSimpleCatalog("default")
	}

	// One LanguageOptions drives both builtin loading and analyzer
	// behavior so the catalog and analyzer can never see diverging
	// feature sets (e.g. LAST_DAY loaded but DATETIME rejected). The
	// populated LanguageOptionsProto on BuiltinFunctionOptions is a
	// hard requirement, not tidiness: leaving language_options unset
	// triggered a deterministic WASM OOB inside the C++ analyzer on
	// Linux x64 wazero — observed as a trap at .$39380 escalating to a
	// host SIGSEGV in runtime.memmove after roughly eleven iterations.
	// macOS wazero was unaffected and the embedded WASM has its name
	// section stripped, so .$39380 cannot be resolved without a debug
	// rebuild. If this re-emerges after a wazero / WASM / zetasql
	// upgrade, retry with language_options unset to confirm whether the
	// same trigger is back before digging further.
	optsProto, langProto := buildAnalyzeRequestOptions(opts)
	catProto := cat.ToProto()
	catProto.BuiltinFunctionOptions = &generated.ZetaSQLBuiltinFunctionOptionsProto{
		LanguageOptions: langProto,
	}
	request.SimpleCatalog = catProto
	request.Options = optsProto

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

// parseRequestLanguageOptions returns the LanguageOptionsProto attached
// to ParseRequest so the WASM bridge can forward it to ParserOptions.
// Engine.Parse and Engine.ParseNext do not take an AnalyzerOptions
// argument, but the parser still has to know which BigQuery-shaped
// constructs to accept (QUALIFY, IS DISTINCT FROM, ...). Reusing the
// same default contract Engine.Analyze applies (via
// buildAnalyzeRequestOptions) keeps the parse-only and analyze paths
// from drifting.
func parseRequestLanguageOptions() *generated.LanguageOptionsProto {
	_, language := buildAnalyzeRequestOptions(nil)
	return language
}

// buildAnalyzeRequestOptions returns the AnalyzerOptionsProto Engine.Analyze
// hands to the WASM bridge plus the LanguageOptionsProto used to load the
// catalog's builtin functions. The two protos share a single LanguageOptions
// instance so the analyzer and the catalog never see diverging feature sets.
//
// The caller's opts is read but never modified: each clone hands back an
// independent instance, then the BigQuery default-contract features are
// layered on that copy.
func buildAnalyzeRequestOptions(opts *AnalyzerOptions) (*generated.AnalyzerOptionsProto, *generated.LanguageOptionsProto) {
	// Step 1: build the effective LanguageOptions. Clone the caller's
	// Language (or start fresh when nil), then layer the BigQuery
	// default contract on top.
	language := NewLanguageOptions()
	if opts != nil && opts.Language != nil {
		language = opts.Language.clone()
	}
	language.enableBigQueryExtensions()

	// Step 2: build the effective AnalyzerOptions. Clone the caller's
	// opts (or start fresh when nil) so other fields — query parameters,
	// parameter mode, parse-location record type — survive untouched,
	// then attach the LanguageOptions from Step 1.
	analyzer := NewAnalyzerOptions()
	if opts != nil {
		analyzer = opts.Clone()
	}
	analyzer.Language = language

	// RejectInvalidLiteralCasts needs the C++ analyzer to attach parse
	// locations to resolved nodes so the [at L:C] suffix on the
	// returned *types.CastValueError can be populated. Auto-promote
	// ParseLocationRecordType to FullNodeScope when the gate is on
	// and the caller did not pick one; an explicit non-nil value is
	// left alone so callers can still opt down to CodeSearch or None
	// if they take responsibility for the consequences.
	if analyzer.RejectInvalidLiteralCasts && analyzer.ParseLocationRecordType == nil {
		p := ParseLocationRecordFullNodeScope
		analyzer.ParseLocationRecordType = &p
	}

	// Step 3: serialize once. Both return values point at the same
	// nested LanguageOptionsProto so the catalog and analyzer sides of
	// the request cannot drift.
	analyzerProto := analyzer.toProto()
	return analyzerProto, analyzerProto.GetLanguageOptions()
}

// rejectInvalidLiteralCasts walks the resolved AST in out and
// returns a *types.CastValueError for the first ResolvedCast whose
// source is a STRING literal that cannot be parsed as the target
// type. Returns nil when no such cast is present.
//
// sql is the SQL string the C++ resolver actually parsed (i.e. the
// post-preprocess version for Engine.Analyze; ParseResumeLocation
// Input for Engine.AnalyzeNext). The literal's ParseLocationRange
// offsets index into this string and feed Line / Col on the returned
// error so the surface error carries BigQuery's [at L:C] suffix.
//
// Reached only when AnalyzerOptions.RejectInvalidLiteralCasts is
// true, matching BigQuery's analyze-time-reject behavior on top of
// upstream ZetaSQL's defer-to-runtime resolution. SAFE_CAST
// (ReturnNullOnError) is skipped because its contract is to return
// NULL on failure rather than error. Currently handles INT64
// targets only; other numeric targets will be added as call sites
// require them.
func rejectInvalidLiteralCasts(out *AnalyzeOutput, sql string) error {
	return resolved_ast.Walk(out.Resolved, func(n resolved_ast.Node) error {
		cast, ok := n.(*resolved_ast.CastNode)
		if !ok || cast.ReturnNullOnError() {
			return nil
		}
		lit, ok := cast.Expr().(*resolved_ast.LiteralNode)
		if !ok {
			return nil
		}
		src := types.WrapLiteralValue(lit.Value())
		if src == nil {
			return nil
		}
		s, ok := src.AsString()
		if !ok {
			return nil
		}
		target := types.WrapType(cast.Type())
		if target == nil || target.Kind() != types.Int64 {
			return nil
		}
		if _, err := castStringToInt64(s); err == nil {
			return nil
		}
		line, col := byteOffsetToLineColumn(sql, int(lit.ParseLocationRange().GetStart()))
		return &types.CastValueError{
			Value:  s,
			ToType: types.Int64,
			Line:   line,
			Col:    col,
		}
	})
}

// byteOffsetToLineColumn turns a 0-indexed byte offset into the
// 1-indexed line/column pair used by BigQuery's [at L:C] error
// suffix. Out-of-range offsets clamp to (1, 1) so the caller still
// gets a positive position rather than a slice-index panic.
func byteOffsetToLineColumn(sql string, offset int) (int, int) {
	if offset < 0 || offset > len(sql) {
		return 1, 1
	}
	line := 1
	lineStart := 0
	for i := 0; i < offset; i++ {
		if sql[i] == '\n' {
			line++
			lineStart = i + 1
		}
	}
	return line, offset - lineStart + 1
}

// castStringToInt64 mirrors BigQuery's CAST(string AS INT64)
// behavior: empty string folds to 0, a "0x"-containing image is
// reparsed in base 0 so hex literals like "0x87a" succeed, and
// everything else goes through base 10. Kept private because the
// only caller is rejectInvalidLiteralCasts; this is not a runtime
// evaluator surface, just the contract the strict-cast gate uses
// to decide which literals are unfoldable.
func castStringToInt64(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	base := 10
	if strings.Contains(strings.ToLower(s), "0x") {
		base = 0
	}
	return strconv.ParseInt(s, base, 64)
}
