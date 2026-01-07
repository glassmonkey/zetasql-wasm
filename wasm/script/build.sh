#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WASM_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "Building ZetaSQL WASM with WASI compatibility..."
echo ""

# Get current user's UID and GID
USER_ID=$(id -u)
GROUP_ID=$(id -g)

# Create cache and output directories with correct ownership
mkdir -p "$WASM_DIR/.cache/bazel"
mkdir -p "$WASM_DIR/output"

# On macOS with Docker Desktop, ownership is automatically handled
# Only set ownership on Linux if needed
if [ "$(uname)" != "Darwin" ]; then
  chown -R "$USER_ID:$GROUP_ID" "$WASM_DIR/.cache/bazel" 2>/dev/null || true
  chown -R "$USER_ID:$GROUP_ID" "$WASM_DIR/output" 2>/dev/null || true
fi
chmod -R u+w "$WASM_DIR/output" 2>/dev/null || true

echo "Building with USER_ID=$USER_ID, GROUP_ID=$GROUP_ID"

# Build Docker image (build environment only)
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
docker run --rm \
  --platform=linux/amd64 \
  -v "$WASM_DIR/assets:/home/builder/workspace:rw" \
  -v "$WASM_DIR/.cache/bazel:/home/builder/.cache/bazel:rw" \
  -v "$WASM_DIR/output:/home/builder/output:rw" \
  "$IMAGE_NAME"

# Copy output to wasm directory
if [ -f "$WASM_DIR/output/zetasql.wasm" ]; then
  # Remove existing file if it exists (to avoid permission issues)
  rm -f "$WASM_DIR/zetasql.wasm"
  cp "$WASM_DIR/output/zetasql.wasm" "$WASM_DIR/"
  echo ""
  echo "Build complete! Generated file:"
  ls -lh "$WASM_DIR/zetasql.wasm"
  echo ""
  echo "WASI-compatible standalone WASM built successfully!"
  echo "Run 'make test' to verify WASI compatibility"
else
  echo "Error: Build failed - zetasql.wasm not found"
  exit 1
fi
