#!/bin/bash
set -e

# Build ZetaSQL WASM with Emscripten toolchain
# wasm_cc_binary rule automatically applies Emscripten toolchain (no platform flag needed)
# Proto schemas are built as dependencies
echo 'Building ZetaSQL WASM with Bazel + Emscripten (Standalone WASM mode)...'
bazel build //:zetasql

echo 'Extracting WASM binary and proto schemas...'
BAZEL_BIN=$(bazel info bazel-bin)
OUTPUT_BASE=$(bazel info output_base)

mkdir -p /home/builder/output
cp "$BAZEL_BIN/zetasql/zetasql_bridge.wasm" /home/builder/output/zetasql.wasm

echo ''
echo 'Extracting proto schemas (built as WASM dependencies)...'
mkdir -p /home/builder/output/schemas
find "$OUTPUT_BASE" -path '*/zetasql/resolved_ast/*.proto' -type f \
   -exec cp {} /home/builder/output/schemas/ \;

echo ''
echo 'Build complete!'
echo 'Generated WASI-compatible standalone WASM:'
ls -lh /home/builder/output/zetasql.wasm
echo ''
echo 'Proto schemas:'
find /home/builder/output/schemas -name '*.proto' -type f | sort
