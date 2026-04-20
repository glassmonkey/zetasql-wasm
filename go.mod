module github.com/glassmonkey/zetasql-wasm

go 1.25.4

require (
	github.com/glassmonkey/zetasql-wasm/wasm v0.0.0
	github.com/google/go-cmp v0.7.0
	github.com/tetratelabs/wazero v1.8.2
	google.golang.org/protobuf v1.36.11
)

replace github.com/glassmonkey/zetasql-wasm/wasm => ./wasm
