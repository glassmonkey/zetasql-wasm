#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WASM_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "Cleaning build artifacts..."

# Remove generated WASM file
rm -f "$WASM_DIR"/*.wasm

# Remove output directory
rm -rf "$WASM_DIR/output"

# Remove Docker image
IMAGE_NAME="zetasql-wasm-builder"
if docker images -q "$IMAGE_NAME" 2> /dev/null | grep -q .; then
  echo "Removing Docker image: $IMAGE_NAME"
  docker rmi "$IMAGE_NAME" || true
fi

# Remove Docker build cache
docker builder prune -f

echo "Clean complete!"
