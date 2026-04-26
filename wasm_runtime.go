package zetasql

import (
	"github.com/tetratelabs/wazero"
)

// sharedWASMCache amortizes the ~5s ZetaSQL WASM compilation cost
// across every Parser/Analyzer constructor in this process. The
// cache only stores deterministic compilation output, so each
// instance still owns its own wazero.Runtime and module instance.
var sharedWASMCache = wazero.NewCompilationCache()

func sharedRuntimeConfig() wazero.RuntimeConfig {
	return wazero.NewRuntimeConfig().WithCompilationCache(sharedWASMCache)
}
