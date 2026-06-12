# AGENTS.md - Margaret Ebook Library

## Project Overview
- **Module**: `faun.projects/margaret/margaret-ebook-library`
- **Go Version**: 1.25.0
- **Purpose**: Go library for reading metadata from EPUB and MOBI ebook formats
- **Public API**: `pkg/ebook/api.go` → `ReadMetadata(path)` and `CalculateFileHash(path)`
- **CLI**: `cmd/main.go` - reads metadata from file path argument

## Build Commands
```bash
make lib        # Build library packages only (excludes cmd/)
make build      # Build CLI binary to ./bin/margaret-ebook-library
make all        # Build both library and binary (default)
make clean      # Remove ./bin directory
```

## Test & Quality
```bash
make test       # Run all tests (go test ./...)
```
**CI Pipeline** (`.gitea/workflows/build.yml`):
- `go mod tidy` + `go mod verify`
- `golangci-lint` (via golangci/golangci-lint-action@v6)
- `govulncheck` (security scanning)
- `make build`

## Run CLI
```bash
make run ARGS="./path/to/book.epub"
# or directly:
./bin/margaret-ebook-library ./path/to/book.epub
```

## Project Structure
```
pkg/
  ebook/api.go       # Public API: ReadMetadata(), CalculateFileHash()
  model/metadata.go  # Metadata struct (Title, Authors, Description, Languages, Cover)
  errs/errors.go     # Error types (ErrUnsupportedFormat, etc.)

internal/
  registry/          # Format detection & reader registration
  epub/              # EPUB parser (OCF, OPF, readers)
  mobi/              # MOBI parser (PDB, EXTH, readers)
  converter/         # HTML conversion utilities

cmd/main.go          # CLI entry point
```

## Adding New Format Support
1. Implement `Reader` interface in `internal/<format>/reader.go`
2. Register in `internal/registry/registry.go` via `New()`
3. Add tests in `internal/<format>/reader_test.go`

## Code Style
- Standard Go conventions (`gofmt`, `go vet`)
- Line length: ~100 chars (soft)
- Errors: wrap with `fmt.Errorf("context: %w", err)`
- Public API in `pkg/`, internal in `internal/`
- Defer: use `defer func() { _ = f.Close() }()` instead of `defer f.Close()`
- Getters: use `Title()` instead of `GetTitle()` field access