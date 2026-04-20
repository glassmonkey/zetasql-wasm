# Example Proto Files for Testing

This directory contains example proto files for testing the code generation tools.

## Directory Structure

```
example/
├── README.md
├── schemas/          # Example proto schemas
│   └── test/
│       └── sample.proto
└── generated/        # Generated Go code (created by tests)
    └── test/
        └── sample.pb.go
```

## Usage

The test scripts (`make test-tools`) use these example files to verify that:
1. `protogen` can successfully generate Go code from proto files
2. The generated code is valid and compiles

## Running Tests Manually

```bash
# Generate Go code from example schemas
go run ../protogen \
  --proto-path schemas \
  --output generated \
  --go-module github.com/glassmonkey/zetasql-wasm/wasm/tools/example/generated \
  --verbose

# Clean generated files
rm -rf generated
```
