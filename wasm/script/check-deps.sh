#!/bin/bash
set -e

echo "Checking dependencies..."

# Check protoc
if ! command -v protoc >/dev/null 2>&1; then
  echo "❌ protoc not found."
  echo "   Install with: brew install protobuf (macOS) or apt-get install protobuf-compiler libprotobuf-dev (Linux)"
  exit 1
fi

# Check go
if ! command -v go >/dev/null 2>&1; then
  echo "❌ go not found."
  echo "   Install from https://go.dev/dl/"
  exit 1
fi

# Check protoc includes libprotobuf
if ! protoc --version | grep -q "libprotoc"; then
  echo "❌ protoc found but may not include libprotobuf-dev"
  exit 1
fi

echo "✅ All dependencies are installed"
echo "   protoc version: $(protoc --version)"
echo "   go version: $(go version)"
