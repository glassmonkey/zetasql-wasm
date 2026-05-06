module github.com/glassmonkey/zetasql-wasm

go 1.26.2

require (
	github.com/glassmonkey/zetasql-wasm/wasm v0.0.0
	github.com/google/go-cmp v0.7.0
	github.com/tetratelabs/wazero v1.8.2
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/Masterminds/semver/v3 v3.3.1 // indirect
	github.com/chzyer/readline v1.5.1 // indirect
	github.com/manifoldco/promptui v0.9.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-tty v0.0.7 // indirect
	github.com/x-motemen/gobump v0.3.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/testify v1.11.1
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/glassmonkey/zetasql-wasm/wasm => ./wasm

tool github.com/x-motemen/gobump/cmd/gobump
