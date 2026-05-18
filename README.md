# Margaret Ebook Library

This project is a Go library for handling ebook files. It currently supports reading metadata from EPUB and MOBI formats.

## Folder Structure

- **cmd/**: Contains the main executable files.
  - `main.go`: The entry point of your application.

- **internal/**: This directory is for internal packages that are not exposed to other modules.
  - **ebook/**: Contains the core logic for handling ebooks.
    - **epub/** and **mobi/**: Subdirectories for specific ebook formats.
      - `reader.go`: The reader implementation for each format.
    - `metadata.go`: Logic for extracting metadata from ebooks.

- **pkg/**: This directory is for packages that are exposed to other modules.
  - **ebook/**: Contains the public API for handling ebooks.
    - **epub/** and **mobi/**: Subdirectories for specific ebook formats.
      - `reader.go`: The reader implementation for each format.
    - `metadata.go`: Logic for extracting metadata from ebooks.

- **tests/**: This directory contains test files.
  - **unit/**: Contains unit tests.
    - `epub_test.go` and `mobi_test.go`: Unit tests for the ebook readers.
  - **integration/**: Contains integration tests.
    - `epub_integration_test.go` and `mobi_integration_test.go`: Integration tests for the ebook readers.

## Getting Started

1. Clone this repository to your local machine.
2. Run `go mod tidy` to download any dependencies.
3. Build and run the application using `go run cmd/main.go`.

This structure should provide a solid foundation for your ebook library project and allow you to easily add more formats in the future.
