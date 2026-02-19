#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WASM_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
EXAMPLE_DIR="$WASM_DIR/tools/example"

cd "$WASM_DIR"

echo "=== Cleaning up previous test outputs ==="
# Clean up all test outputs before running tests
rm -rf "$EXAMPLE_DIR/schemas"
rm -rf "$EXAMPLE_DIR/generated"
echo "✅ Cleaned up test outputs"
echo ""

echo "=== Testing extract-protos ==="
echo ""

# Test extract-protos: input/ -> schemas/ (proto files only)
echo "Running extract-protos..."
if go run ./tools/extract-protos \
  --output-base "$EXAMPLE_DIR/input" \
  --schemas-dir "$EXAMPLE_DIR/schemas" \
  --path-prefix "" \
  --verbose; then
  echo "✅ extract-protos successfully extracted proto files"

  # Verify proto files were extracted
  EXPECTED_PROTO_FILES=(
    "$EXAMPLE_DIR/schemas/test/sample.proto"
    "$EXAMPLE_DIR/schemas/api/v1/user.proto"
  )

  for file in "${EXPECTED_PROTO_FILES[@]}"; do
    if [ -f "$file" ]; then
      echo "✅ Extracted proto file exists: ${file#$EXAMPLE_DIR/schemas/}"
    else
      echo "❌ Expected proto file not found: ${file#$EXAMPLE_DIR/schemas/}"
      exit 1
    fi
  done

  # Verify non-proto files were NOT extracted
  EXCLUDED_FILES=(
    "$EXAMPLE_DIR/schemas/test/README.txt"
    "$EXAMPLE_DIR/schemas/test/data.json"
    "$EXAMPLE_DIR/schemas/api/BUILD"
  )

  for file in "${EXCLUDED_FILES[@]}"; do
    if [ -f "$file" ]; then
      echo "❌ Non-proto file was incorrectly extracted: ${file#$EXAMPLE_DIR/schemas/}"
      exit 1
    fi
  done
  echo "✅ Non-proto files were correctly excluded"
else
  echo "❌ extract-protos failed"
  exit 1
fi

echo ""
echo "=== Testing protogen ==="
echo ""

# Test protogen: schemas/ -> generated/
echo "Running protogen..."
if go run ./tools/protogen \
  --proto-path "$EXAMPLE_DIR/schemas" \
  --output "$EXAMPLE_DIR/generated" \
  --go-module "github.com/glassmonkey/zetasql-wasm/wasm/tools/example/generated" \
  --verbose; then
  echo "✅ protogen successfully generated code"

  # Verify generated files exist
  EXPECTED_GENERATED_FILES=(
    "$EXAMPLE_DIR/generated/test/sample.pb.go"
    "$EXAMPLE_DIR/generated/api/v1/user.pb.go"
  )

  for file in "${EXPECTED_GENERATED_FILES[@]}"; do
    if [ -f "$file" ]; then
      LINE_COUNT=$(wc -l < "$file")
      echo "✅ Generated file exists: ${file#$EXAMPLE_DIR/generated/} ($LINE_COUNT lines)"

      if [ "$LINE_COUNT" -le 10 ]; then
        echo "❌ Generated file seems too small ($LINE_COUNT lines)"
        exit 1
      fi
    else
      echo "❌ Expected generated file not found: ${file#$EXAMPLE_DIR/generated/}"
      exit 1
    fi
  done
else
  echo "❌ protogen failed"
  exit 1
fi


echo ""
echo "✅ All tool tests passed!"
