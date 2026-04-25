package zetasql

import (
	"context"
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
)

// TestZetaSQLWasm_ExportsRequiredFunctions guards the contract between the
// embedded WASM binary and the Go bridge: every function the bridge calls
// must be exported by the module. A regenerated WASM that drops an export
// fails this test instead of failing later from inside Parser/Analyzer.
func TestZetaSQLWasm_ExportsRequiredFunctions(t *testing.T) {
	// Arrange
	ctx := context.Background()
	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	compiled, err := runtime.CompileModule(ctx, wasm.ZetaSQLWasm)
	require.NoError(t, err)
	defer compiled.Close(ctx)
	exports := compiled.ExportedFunctions()

	tests := []struct {
		name string
	}{
		{name: "init_module"},
		{name: "malloc"},
		{name: "free"},
		{name: "parse_statement_proto"},
		{name: "analyze_statement_proto"},
		{name: "free_proto_buffer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			_, ok := exports[tt.name]

			// Assert
			assert.True(t, ok, "required WASM export %q is missing", tt.name)
		})
	}
}
