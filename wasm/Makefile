.PHONY: build clean test help test-tools

# Default target
.DEFAULT_GOAL := help

# Output files
WASM_OUTPUT := zetasql.wasm

help: ## Show this help message
	@echo "ZetaSQL WASM Build System"
	@echo ""
	@echo "Usage:"
	@echo "  make build         - Build ZetaSQL WASM (includes proto extraction and Go code generation)"
	@echo "  make test          - Test WASI compatibility"
	@echo "  make clean         - Remove build artifacts"
	@echo "  make rebuild       - Clean and build"
	@echo ""
	@echo "Development:"
	@echo "  make test-tools    - Test tools locally with go run (checks dependencies automatically)"
	@echo ""
	@echo "Direct Tool Usage:"
	@echo "  go run ./tools/protogen --help"
	@echo "  go run ./tools/extract-protos --help"
	@echo "  bash script/check-deps.sh"
	@echo ""
	@echo "Note: Tools are automatically executed inside Docker during 'make build'."
	@echo ""

build: ## Build ZetaSQL WASM using Docker
	@bash script/build.sh 2>&1 | tee build.log

test: ## Test WASI compatibility
	@if [ ! -f $(WASM_OUTPUT) ]; then \
		echo "Error: $(WASM_OUTPUT) not found. Run 'make build' first."; \
		exit 1; \
	fi
	@bash script/test-wasi.sh

clean: ## Remove build artifacts
	@bash script/clean.sh

rebuild: clean build ## Clean and rebuild

test-tools: ## Test tools locally with go run
	@bash script/check-deps.sh
	@bash script/test-tools.sh
