#!/bin/bash
set -e

# Build ZetaSQL WASM with Emscripten toolchain
# wasm_cc_binary rule automatically applies Emscripten toolchain (no platform flag needed)
# Proto schemas are built as dependencies
echo 'Building ZetaSQL WASM with Bazel + Emscripten (Standalone WASM mode)...'
bazel build //:zetasql

echo ''
echo 'Building generated proto files (resolved_ast.proto, resolved_node_kind.proto)...'
bazel build @zetasql//zetasql/resolved_ast:run_gen_resolved_ast_proto

echo ''
echo 'Building parse_tree.proto (generated from template)...'
bazel build @zetasql//zetasql/parser:gen_protos

echo 'Extracting WASM binary and proto schemas...'
BAZEL_BIN=$(bazel info bazel-bin)
OUTPUT_BASE=$(bazel info output_base)

# Ensure output directory exists and is writable
mkdir -p /home/builder/output
chmod 755 /home/builder/output

cp "$BAZEL_BIN/zetasql/zetasql_bridge.wasm" /home/builder/output/zetasql.wasm

echo ''
echo 'Extracting proto schemas (built as WASM dependencies)...'
extract-protos --output-base "$OUTPUT_BASE" --schemas-dir /home/builder/output/schemas --path-prefix "/zetasql+/zetasql/"

echo ''
echo 'Generating Go code from proto schemas...'
cd /home/builder
protogen --proto-path output/schemas --output output/generated

echo ''
echo 'Build complete!'
echo 'Generated WASI-compatible standalone WASM:'
ls -lh /home/builder/output/zetasql.wasm
echo ''
echo 'Proto schemas:'
find /home/builder/output/schemas -name '*.proto' -type f | wc -l
echo 'proto files extracted'
echo ''
echo 'Generated Go code:'
find /home/builder/output/generated -name '*.pb.go' -type f | wc -l
echo 'Go files generated'
