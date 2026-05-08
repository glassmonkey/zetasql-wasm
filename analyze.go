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
	"google.golang.org/protobuf/proto"
)

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
	if e.module == nil {
		return nil, nil, fmt.Errorf("engine is not initialized")
	}

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
	out := &AnalyzeOutput{Statement: stmt}
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
