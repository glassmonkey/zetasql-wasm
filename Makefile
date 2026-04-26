GO_MODULES := . wasm
GOLANGCI_LINT_VERSION := v2.5.0

.PHONY: help test lint lint-install build vet tidy fmt ci

.DEFAULT_GOAL := help

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

test: ## go test ./... -race (all modules)
	@for m in $(GO_MODULES); do \
		echo "==> go test in $$m"; \
		(cd $$m && go test ./... -race) || exit 1; \
	done

lint: ## golangci-lint run (all modules)
	@command -v golangci-lint >/dev/null || { echo "golangci-lint not installed. Run: make lint-install"; exit 1; }
	@for m in $(GO_MODULES); do \
		echo "==> golangci-lint in $$m"; \
		(cd $$m && golangci-lint run) || exit 1; \
	done

lint-install: ## Install pinned golangci-lint
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

build: ## go build ./... (all modules)
	@for m in $(GO_MODULES); do \
		echo "==> go build in $$m"; \
		(cd $$m && go build ./...) || exit 1; \
	done

vet: ## go vet ./... (all modules)
	@for m in $(GO_MODULES); do \
		echo "==> go vet in $$m"; \
		(cd $$m && go vet ./...) || exit 1; \
	done

tidy: ## go mod tidy (all modules)
	@for m in $(GO_MODULES); do \
		echo "==> go mod tidy in $$m"; \
		(cd $$m && go mod tidy) || exit 1; \
	done

fmt: ## gofmt -w -s on the tree
	@gofmt -w -s .

ci: lint test ## Mirror CI locally (lint + test)
