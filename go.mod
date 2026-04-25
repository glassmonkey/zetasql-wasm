module github.com/glassmonkey/zetasql-wasm

go 1.25.4

require (
	github.com/glassmonkey/zetasql-wasm/wasm v0.0.0
	github.com/google/go-cmp v0.7.0
	github.com/tetratelabs/wazero v1.8.2
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/testify v1.11.1
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/glassmonkey/zetasql-wasm/wasm => ./wasm
