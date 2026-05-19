# Makefile for the Go project – library, binary and tests

# Output binary (CLI) location
BINARY=./bin/margaret-ebook-library

# Phony targets (no actual files)
.PHONY: all build lib run test clean help

# Default target: library build + binary build
all: lib build

# -----------------------------------------------------------------
# Help – description of targets
# -----------------------------------------------------------------
help:
	@echo "Makefile targets:"
	@echo "  make all      - library and binary build (default)"
	@echo "  make lib      - build only the library packages"
	@echo "  make build    - CLI binary build into bin/ directory"
	@echo "  make run      - build and run (use ARGS=\"./path/to/book.epub\")"
	@echo "  make test     - run tests"
	@echo "  make clean    - remove bin/ directory and artifacts"
	@echo "  make help     - display this help"

# -----------------------------------------------------------------
# Library (only library packages) build
# -----------------------------------------------------------------
# The `./...` recursively finds all packages, but we exclude `cmd/`
# with `grep -v '^./cmd'` because it only contains the CLI.
lib:
	@echo "📦 Library (module) build..."
	@go list ./... | grep -v '^./cmd' | xargs -n1 go build -v

# -----------------------------------------------------------------
# Binary (CLI) build – the binary is placed in the bin/ directory
# -----------------------------------------------------------------
build: | bin
	@echo "🔧 CLI binary build..."
	go build -o $(BINARY) ./cmd

# -----------------------------------------------------------------
# Run – first build, then execute the binary
# Usage: make run ARGS="./path/to/the/book.epub"
# -----------------------------------------------------------------
ARGS ?=
run: build
	@echo "🚀 Starting CLI..."
	$(BINARY) "$(ARGS)"

# -----------------------------------------------------------------
# Tests – for the entire module (unit + integration)
# -----------------------------------------------------------------
test:
	@echo "🧪 Running tests..."
	go test ./...

# -----------------------------------------------------------------
# Clean – remove the bin/ directory and compiled artifacts
# -----------------------------------------------------------------
clean:
	@echo "🧹 Cleaning..."
	rm -rf ./bin
	# (optional) clean Go cache – only if needed
	# go clean -cache -modcache -i -r

# -----------------------------------------------------------------
# Helper target: create bin directory if it does not exist
# -----------------------------------------------------------------
bin:
	mkdir -p bin
