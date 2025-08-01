# Makefile for go-claude-monitor
# Project build and test tools

# Project name
PROJECT_NAME := go-claude-monitor

# Binary directory
BIN_DIR := bin

# Executable name
BINARY := $(BIN_DIR)/$(PROJECT_NAME)

# Go installation directory (GOPATH/bin or GOBIN)
GOBIN := $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(shell go env GOPATH)/bin
endif

# Build flags
BUILD_FLAGS := -ldflags="-s -w"

# Default target
.PHONY: all
all: build

# Build project
.PHONY: build
build:
	@echo "Building $(PROJECT_NAME)..."
	@mkdir -p $(BIN_DIR)
	@go build $(BUILD_FLAGS) -o $(BINARY) ./cmd
	@echo "Build completed: $(BINARY)"

# Install to GOBIN directory
.PHONY: install
install: build
	@echo "Installing $(PROJECT_NAME) to $(GOBIN)..."
	@cp $(BINARY) $(GOBIN)/$(PROJECT_NAME)
	@echo "Installation completed: $(GOBIN)/$(PROJECT_NAME)"

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BIN_DIR)
	@echo "Clean completed"

# Run all tests
.PHONY: test
test:
	@echo "Running all tests..."
	@go test -v ./...

# Generate test coverage report
.PHONY: coverage
coverage:
	@echo "Generating test coverage report..."
	@mkdir -p $(BIN_DIR)
	@go test -coverprofile=$(BIN_DIR)/coverage.out ./...
	@go tool cover -html=$(BIN_DIR)/coverage.out -o $(BIN_DIR)/coverage.html
	@echo "Coverage report generated: $(BIN_DIR)/coverage.html"
	@echo "Coverage summary:"
	@go tool cover -func=$(BIN_DIR)/coverage.out

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Code static analysis
.PHONY: lint
lint:
	@echo "Running linter..."
	@go vet ./...

# Run all checks and tests
.PHONY: check
check: fmt lint test

# Release a specific version (usage: make release v0.0.1)
.PHONY: release
release:
	@if [ -z "$(filter-out $@,$(MAKECMDGOALS))" ]; then \
		echo "Usage: make release v0.0.1"; \
		exit 1; \
	fi
	@VERSION=$(filter-out $@,$(MAKECMDGOALS)); \
	echo "Releasing version $$VERSION"; \
	if ! echo "$$VERSION" | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$$' > /dev/null; then \
		echo "Error: Version must be in format v0.0.1"; \
		exit 1; \
	fi; \
	if [ -n "$$(git status --porcelain)" ]; then \
		echo "Error: Working directory is not clean. Please commit or stash changes."; \
		exit 1; \
	fi; \
	echo "Updating version to $$VERSION"; \
	git add -A; \
	git commit -m "chore: bump version to $$VERSION" || true; \
	git tag -a "$$VERSION" -m "Release $$VERSION"; \
	git push origin main; \
	git push origin "$$VERSION"; \
	echo "Version $$VERSION has been tagged and pushed successfully"

# Internal goreleaser release command
.PHONY: goreleaser-release
goreleaser-release:
	goreleaser release --clean --skip-validate --skip-lint

# Help information
.PHONY: help
help:
	@echo "Available commands:"
	@echo "  build     - Build the project binary to bin/ directory"
	@echo "  install   - Build and install binary to GOBIN directory"
	@echo "  clean     - Remove bin/ directory and build artifacts"
	@echo "  test      - Run all tests"
	@echo "  coverage  - Generate test coverage report"
	@echo "  fmt       - Format code with go fmt"
	@echo "  lint      - Run go vet for static analysis"
	@echo "  check     - Run fmt, lint, and test"
	@echo "  release   - Release a specific version (usage: make release v0.0.1)"
	@echo "  goreleaser-release - Internal goreleaser release command"
	@echo "  help      - Show this help message"

# Allow arguments to be passed to make release
%:
	@: 