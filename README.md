# Margaret Ebook Library

[![Build Status](https://github.com/f0d0r/margaret-ebook-library/actions/workflows/build.yml/badge.svg)](https://github.com/f0d0r/margaret-ebook-library/actions/workflows/build.yml)

This project is a Go library for handling ebook files. It currently supports reading EPUB and MOBI formats.

## Getting Started

1. Clone this repository to your local machine.
2. Run `go mod tidy` to download any dependencies.

## Usage

Import the library:

```go
import "github.com/f0d0r/margaret-ebook-library/pkg/ebook"
```

### Reading an ebook from a file path

The simplest way is to pass the file path directly:

```go
ebook, err := ebook.Read("books/my-book.epub")
if err != nil {
    // handle the error, e.g. with errors.Is(err, errs.ErrUnsupportedFormat)
}
```

### Reading an ebook from an already-open file

Use this when you already have the file open:

```go
f, err := os.Open("books/my-book.mobi")
if err != nil {
    log.Fatal(err)
}
defer f.Close()

ebook, err := ebook.ReadFromFile(f)
if err != nil {
    log.Fatal(err)
}
```

The caller retains ownership of the file and must keep it open until the
returned ebook (including any cover and content data) has been consumed.

### Reading an ebook from a blob

Blobs are random-access sources, so the same API works for local files,
ebook entries inside an archive, or remote sources. The source position is
never modified and the caller keeps ownership.

```go
ebook, err := ebook.ReadFromBlob(model.NewPathBlob("books/my-book.epub"))
if err != nil {
    log.Fatal(err)
}
```

### Using the ebook

```go
metadata := ebook.Metadata
fmt.Println("Title:", metadata.Title)
fmt.Println("Authors:", strings.Join(metadata.Authors, ", "))
fmt.Println("Language:", metadata.Languages)
fmt.Println("Description:", metadata.Description)
fmt.Println("FileType:", ebook.FileType)
fmt.Println("Version:", ebook.Version)

if metadata.Cover != nil {
    rc, err := metadata.Cover.Open()
    if err != nil {
        log.Fatal(err)
    }
    defer rc.Close()

    // Stream the cover, e.g. write it to a file:
    f, err := os.Create(metadata.Cover.Name)
    if err != nil {
        log.Fatal(err)
    }
    if _, err := io.Copy(f, rc); err != nil {
        log.Fatal(err)
    }
    if err := f.Close(); err != nil {
        log.Fatal(err)
    }
}
```

To load the whole cover into memory instead, use the convenience method:

```go
data, err := ebook.Metadata.Cover.Data() // reads the whole cover into a []byte
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Cover %q (%s): %d bytes\n", ebook.Metadata.Cover.Name, ebook.Metadata.Cover.MediaType, len(data))
```

### Overriding the safety limits

By default the library applies conservative safety limits (50 MB max cover
size, 100 MB max single MOBI record, 256 max EXTH records). Override them
with functional options:

```go
ebook, err := ebook.Read(
    "books/my-book.epub",
    ebook.WithMaxCoverSize(10*1024*1024), // 10 MB
)
if err != nil {
    log.Fatal(err)
}
```

MOBI-specific limits can be adjusted the same way:

```go
ebook, err := ebook.Read(
    "books/my-book.mobi",
    ebook.WithMaxRecordSize(50*1024*1024), // max single PDB record
    ebook.WithMaxExthRecords(512),         // max EXTH records to parse
)
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