GO_MODULES := . wasm
GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.5.0

.PHONY: help test lint fix

.DEFAULT_GOAL := help

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-6s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

test: ## go test ./... (all modules)
	@for m in $(GO_MODULES); do \
		echo "==> go test in $$m"; \
		(cd $$m && go test ./...) || exit 1; \
	done

lint: ## golangci-lint run (all modules)
	@for m in $(GO_MODULES); do \
		echo "==> golangci-lint in $$m"; \
		(cd $$m && $(GOLANGCI_LINT) run) || exit 1; \
	done

fix: ## Auto-fix: golangci-lint --fix, gofmt, go mod tidy
	@gofmt -w -s .
	@for m in $(GO_MODULES); do \
		echo "==> fix in $$m"; \
		(cd $$m && $(GOLANGCI_LINT) run --fix && go mod tidy) || exit 1; \
	done
