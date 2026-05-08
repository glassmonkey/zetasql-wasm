package zetasql

import (
	"context"
	"fmt"

	"github.com/glassmonkey/zetasql-wasm/wasm"
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
