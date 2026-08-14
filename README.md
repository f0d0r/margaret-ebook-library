# Margaret Ebook Library

[![Build Status](https://github.com/f0d0r/margaret-ebook-library/actions/workflows/build.yml/badge.svg)](https://github.com/f0d0r/margaret-ebook-library/actions/workflows/build.yml)

This project is a Go library for handling ebook files. It currently supports reading metadata from EPUB and MOBI formats.

## Getting Started

1. Clone this repository to your local machine.
2. Run `go mod tidy` to download any dependencies.

## Usage

Import the library:

```go
import "github.com/f0d0r/margaret-ebook-library/pkg/ebook"
```

### Reading metadata from a file path

The simplest way is to pass the file path directly:

```go
metadata, err := ebook.ReadMetadata("books/my-book.epub")
if err != nil {
    // handle the error, e.g. with errors.Is(err, errs.ErrUnsupportedFormat)
}
```

### Reading metadata from an already-open file

Use this when you already have the file open:

```go
f, err := os.Open("books/my-book.mobi")
if err != nil {
    log.Fatal(err)
}
defer f.Close()

metadata, err := ebook.ReadMetadataFromFile(f)
if err != nil {
    log.Fatal(err)
}
```

The caller retains ownership of the file and must keep it open until the
returned metadata (including any cover data) has been consumed.

### Reading metadata from a blob

Blobs are random-access sources, so the same API works for local files,
ebook entries inside an archive, or remote sources. The source position is
never modified and the caller keeps ownership.

```go
metadata, err := ebook.ReadMetadataFromBlob(model.NewPathBlob("books/my-book.epub"))
if err != nil {
    log.Fatal(err)
}
```

### Using the metadata

```go
fmt.Println("Title:", metadata.Title)
fmt.Println("Authors:", strings.Join(metadata.Authors, ", "))
fmt.Println("Language:", metadata.Languages)
fmt.Println("Description:", metadata.Description)
fmt.Println("FileType:", metadata.FileType)

if metadata.Cover != nil {
    data, err := metadata.Cover.Data()
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Cover %q (%s): %d bytes\n", metadata.Cover.Name, metadata.Cover.MediaType, len(data))
}
```

### Overriding the safety limits

By default a config with a 50 MB cover-size limit is used. Pass a custom
[`model.Config`](pkg/model/config.go) to change it:

```go
cfg := model.DefaultConfig()
cfg.MaxCoverSize = 10 * 1024 * 1024 // 10 MB

metadata, err := ebook.ReadMetadata("books/my-book.epub", &cfg)
if err != nil {
    log.Fatal(err)
}
```

### Hashing a book

```go
hash, err := ebook.CalculateFileHash("books/my-book.epub")
if err != nil {
    log.Fatal(err)
}
fmt.Println(hash) // hex-encoded sha256
```