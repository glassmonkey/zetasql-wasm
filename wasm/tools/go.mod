module github.com/glassmonkey/zetasql-wasm/wasm/tools

go 1.26.2

require (
	github.com/glassmonkey/zetasql-wasm v0.4.0
	github.com/urfave/cli/v3 v3.6.1
	google.golang.org/protobuf v1.36.11
)

replace github.com/glassmonkey/zetasql-wasm => ../..
