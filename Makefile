# Go Build Variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -w -s -X github.com/the-open-agent/openagent/internal/cli.Version=${VERSION} -X github.com/the-open-agent/openagent/internal/cli.Commit=${COMMIT} -X github.com/the-open-agent/openagent/internal/cli.BuildDate=${BUILD_DATE}

.PHONY: all build build-linux lint lint-mcp test clean help

all: build ## Build the server

build: ## Build server for current OS/ARCH
	go build -ldflags="${LDFLAGS}" -o server .

build-linux: ## Build server for Linux amd64, arm64, riscv64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o server_linux_amd64 .
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="${LDFLAGS}" -o server_linux_arm64 .
	CGO_ENABLED=0 GOOS=linux GOARCH=riscv64 go build -ldflags="${LDFLAGS}" -o server_linux_riscv64 .

lint: ## Run golangci-lint
	golangci-lint run ./...

lint-mcp: ## Run MCP/Tool/Skill execution context lint check
	./scripts/check-mcp-lint.sh

test: ## Run all tests
	go test ./...

clean: ## Remove build artifacts
	rm -f server server_linux_*

help: ## Show this help message
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
