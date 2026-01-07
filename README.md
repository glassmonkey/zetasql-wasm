# zetasql-wasm

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Status](https://img.shields.io/badge/Status-WIP-yellow.svg)](https://github.com/glassmonkey/zetasql-wasm)

> **⚠️ Work in Progress**: Currently, only the WASM build is completed. Go SDK implementation is TODO.

WebAssembly (WASM) binding for [ZetaSQL](https://github.com/google/zetasql) in Go using [wazero](https://wazero.io/).

ZetaSQL is Google's SQL parser and analyzer used in BigQuery and Spanner. This library allows you to use ZetaSQL in Go applications without CGO, enabling easy cross-compilation and distribution.

[日本語](./docs/ja/README.md) | **English** (This document)

## Current Status

- ✅ **WASM Build**: ZetaSQL compiled to WebAssembly with Emscripten
- 🚧 **Go SDK**: Implementation in progress (TODO)
- 📝 **Documentation**: Architecture and build process documented

## Planned Features

- 🚧 **CGO-free**: Pure Go implementation using WebAssembly (WASM build completed)
- 🚧 **Easy cross-compilation**: No C++ compiler required (for Go SDK users)
- ✅ **Automatic dependency management**: Renovate Bot keeps Bazel, ZetaSQL, and Go dependencies up-to-date
- 🚧 **Simple API**: Similar to [goccy/go-zetasql](https://github.com/goccy/go-zetasql) (TODO)
- ⚠️ **Performance**: Slightly slower than native CGO (10-30%)

## Installation

> **Coming soon**: Go SDK is not yet available. Only WASM binary is currently built.

```bash
# TODO: Go SDK implementation
# go get github.com/glassmonkey/zetasql-wasm
```

## Quick Start

> **Coming soon**: Go SDK API is under development.

```go
// TODO: Go SDK implementation
// package main
//
// import (
//     "context"
//     "fmt"
//     "log"
//
//     "github.com/glassmonkey/zetasql-wasm"
// )
//
// func main() {
//     ctx := context.Background()
//
//     // Create a parser
//     parser, err := zetasql.NewParser(ctx)
//     if err != nil {
//         log.Fatal(err)
//     }
//     defer parser.Close(ctx)
//
//     // Parse SQL
//     stmt, err := parser.ParseStatement(ctx, "SELECT * FROM users WHERE age > 20")
//     if err != nil {
//         log.Fatal(err)
//     }
//
//     fmt.Printf("Parsed statement: %+v\n", stmt)
// }
```

## Documentation

### English
- 📚 [Architecture Design](./docs/en/architecture.md)
- 📚 [Patch Application Guide](./docs/en/patches.md)
- 📚 [WASM SDK Test Plan](./docs/en/wasm-sdk-test-plan.md)

### 日本語 (Japanese)
- 📚 [アーキテクチャ設計書](./docs/ja/architecture.md)
- 📚 [パッチ適用ガイド](./docs/ja/patches.md)
- 📚 [WASM SDK テスト計画](./docs/ja/wasm-sdk-test-plan.md)

## Architecture

```
┌─────────────┐
│  Go App     │
└──────┬──────┘
       │
       ↓ (Go API)
┌─────────────────┐
│ zetasql-wasm    │
└──────┬──────────┘
       │
       ↓ (wazero)
┌─────────────────┐
│  WASM Runtime   │
└──────┬──────────┘
       │
       ↓ (WASM)
┌─────────────────┐
│ ZetaSQL C++     │
│ (Emscripten)    │
└─────────────────┘
```

## Comparison with goccy/go-zetasql

| Item | goccy/go-zetasql | zetasql-wasm |
|------|------------------|--------------|
| Binding | CGO | WASM (wazero) |
| Cross-compilation | Difficult | Easy |
| Build time | Long (10-30 min) | Long (first time only) |
| Runtime performance | Fast | Slightly slower |
| Binary size | Small | Larger (includes WASM) |
| Dependencies | C++ compiler required | Not required |
| Maintainability | Complex | Simple |

## Building from Source

### Building WASM Module (✅ Available)

```bash
# Clone the repository
git clone https://github.com/glassmonkey/zetasql-wasm.git
cd zetasql-wasm

# Build WASM module
cd wasm
./build.sh

# The WASM binary will be generated at: wasm/zetasql.wasm
```

### Building Go SDK (🚧 TODO)

```bash
# TODO: Go SDK implementation
# cd ..
# go test ./...
# go build ./...
```

For detailed build instructions, see:
- [Architecture Design (Japanese)](./docs/ja/architecture.md)
- [Patch Application Guide (Japanese)](./docs/ja/patches.md)
- [WASM SDK Test Plan (Japanese)](./docs/ja/wasm-sdk-test-plan.md)

## License

Apache 2.0

## Credits

- [ZetaSQL](https://github.com/google/zetasql) - Google's SQL parser and analyzer
- [wazero](https://wazero.io/) - Pure Go WebAssembly runtime
- [goccy/go-zetasql](https://github.com/goccy/go-zetasql) - CGO-based ZetaSQL bindings for Go

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

---

**Maintained by**: [@glassmonkey](https://github.com/glassmonkey)
