#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WASM_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "Building ZetaSQL WASM with WASI compatibility..."
echo ""

# Get current user's UID and GID
USER_ID=$(id -u)
GROUP_ID=$(id -g)

# Create cache and output directories with correct permissions
mkdir -p "$WASM_DIR/.cache/bazel"
mkdir -p "$WASM_DIR/output"
chmod -R 777 "$WASM_DIR/output"  # Ensure Docker can write to output directory

echo "Building with USER_ID=$USER_ID, GROUP_ID=$GROUP_ID"

# Build Docker image
IMAGE_NAME="zetasql-wasm-builder"
echo "Building Docker image: $IMAGE_NAME"
docker build \
  --platform=linux/amd64 \
  --build-arg USER_ID="$USER_ID" \
  --build-arg GROUP_ID="$GROUP_ID" \
  -t "$IMAGE_NAME" \
  -f "$WASM_DIR/Dockerfile" \
  "$WASM_DIR"

echo ""
echo "Running build in container..."

# Run container with volume mounts
# Note: .cache/bazel mount is temporarily disabled due to permission issues
docker run --rm \
  --platform=linux/amd64 \
  -v "$WASM_DIR/assets:/home/builder/workspace:rw" \
  -v "$WASM_DIR/output:/home/builder/output:rw" \
  "$IMAGE_NAME"

# Copy output to wasm directory
if [ -f "$WASM_DIR/output/zetasql.wasm" ]; then
  # Copy WASM file
  rm -f "$WASM_DIR/zetasql.wasm"
  cp "$WASM_DIR/output/zetasql.wasm" "$WASM_DIR/"

  # Copy proto schemas
  if [ -d "$WASM_DIR/output/schemas" ]; then
    echo ""
    echo "Copying proto schemas..."
    rm -rf "$WASM_DIR/schemas"
    cp -r "$WASM_DIR/output/schemas" "$WASM_DIR/"
    echo "Proto schemas copied to $WASM_DIR/schemas/"
    echo "  $(find "$WASM_DIR/schemas" -name '*.proto' -type f | wc -l) proto files"
  fi

  # Copy generated Go code
  if [ -d "$WASM_DIR/output/generated" ]; then
    echo ""
    echo "Copying generated Go code..."
    rm -rf "$WASM_DIR/generated"
    cp -r "$WASM_DIR/output/generated" "$WASM_DIR/"
    echo "Generated Go code copied to $WASM_DIR/generated/"
    echo "  $(find "$WASM_DIR/generated" -name '*.pb.go' -type f | wc -l) Go files"
  fi

  echo ""
  echo "Build complete! Generated files:"
  ls -lh "$WASM_DIR/zetasql.wasm"

  echo ""
  echo "WASI-compatible standalone WASM built successfully!"
else
  echo "Error: Build failed - zetasql.wasm not found"
  exit 1
fi
