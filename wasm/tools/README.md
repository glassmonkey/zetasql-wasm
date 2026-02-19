# ZetaSQL WASM Code Generation Tools

This directory contains Go tools for extracting proto schemas from Bazel build output and generating Go code from those schemas.

## Tools

### 1. extract-protos
Extracts proto schema files from Bazel build output.

**Usage:**
```bash
# Direct execution with go run
go run ./tools/extract-protos \
  --output-base /path/to/bazel/output \
  --schemas-dir /path/to/output/schemas \
  --verbose

# Using Makefile (requires OUTPUT_BASE and SCHEMAS_DIR env vars)
OUTPUT_BASE=/path/to/bazel/output \
SCHEMAS_DIR=/path/to/schemas \
make extract-protos
```

**Options:**
- `--output-base`: Bazel output base directory (from `bazel info output_base`)
- `--schemas-dir`: Output directory for extracted proto schemas
- `--verbose`: Enable verbose output

### 2. protogen
Generates Go code from Protocol Buffer files.

**Usage:**
```bash
# Direct execution with go run
go run ./tools/protogen \
  --proto-path schemas \
  --output generated \
  --go-module github.com/glassmonkey/zetasql-wasm/wasm/generated \
  --verbose

# Using Makefile (requires schemas directory)
make protogen
```

**Options:**
- `--proto-path` / `-p`: Path to proto schema directory (default: "schemas")
- `--output` / `-o`: Output directory for generated code (default: "generated")
- `--go-module`: Go module path for generated code (default: "github.com/glassmonkey/zetasql-wasm/wasm/generated")
- `--verbose`: Enable verbose output

## Local Development

### Prerequisites

Check if you have all required dependencies:

```bash
# Using Makefile
make check-deps

# Direct script execution
bash script/check-deps.sh
```

Required dependencies:
- `go` (1.23+)
- `protoc` (with libprotobuf-dev)

**Installation:**
- macOS: `brew install protobuf go`
- Linux: `apt-get install protobuf-compiler libprotobuf-dev golang`

### Testing Tools Locally

Test both tools without building binaries:

```bash
# Using Makefile
make test-tools

# Direct script execution
bash script/test-tools.sh
```

This will:
1. Check dependencies
2. Test `extract-protos --help`
3. Test `protogen --help`
4. If schemas exist, test proto code generation

### Testing Individual Tools

```bash
# Test extract-protos
go run ./tools/extract-protos --help

# Test protogen
go run ./tools/protogen --help
```

## Development Workflow

1. **Check dependencies:**
   ```bash
   make check-deps
   ```

2. **Build WASM (extracts schemas automatically):**
   ```bash
   make build
   ```

3. **Test tools:**
   ```bash
   make test-tools
   ```

4. **Generate Go code from extracted schemas:**
   ```bash
   make protogen
   ```

## Directory Structure

```
tools/
├── README.md                    # This file
├── extract-protos/
│   └── main.go                  # Proto extraction tool
└── protogen/
    └── main.go                  # Go code generation tool
```

## Notes

- These tools are automatically executed inside Docker during `make build`
- For local testing, you can use `go run` without building binaries
- The tools use the same dependencies as the Docker build environment
- Proto schemas are extracted from Bazel's build output
- Generated Go code uses `paths=source_relative` for imports

## Troubleshooting

### "protoc not found"
Install protobuf compiler:
- macOS: `brew install protobuf`
- Linux: `apt-get install protobuf-compiler libprotobuf-dev`

### "google/protobuf/descriptor.proto not found"
Install libprotobuf-dev:
- macOS: Already included in `brew install protobuf`
- Linux: `apt-get install libprotobuf-dev`

### "schemas directory not found"
Run `make build` first to extract schemas from Bazel build output.
