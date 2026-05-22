# Margaret Ebook Library

This project is a Go library for handling ebook files. It currently supports reading metadata from EPUB and MOBI formats.

## Folder Structure

- **bin/**: Built executable artifacts.

- **cmd/**: Contains the main application entry point.
  - `main.go`: The entry point of the application.

- **internal/**: Internal packages that are intended for use only inside this module.

- **pkg/**: Public packages exposed to other modules.
  - **ebook/**: Public ebook handling API.
    - `api.go`
  - **errs/**: Defines internal error types.
    - `errors.go`
  - **model/**: Ebook metadata model definitions.
    - `metadata.go`

## Getting Started

1. Clone this repository to your local machine.
2. Run `go mod tidy` to download any dependencies.
3. Build and run the application using `make run`.

