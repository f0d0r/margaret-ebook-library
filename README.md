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

The caller retains ownership of the file and must keep it open until
all resource data (including the cover) has been consumed.

### Reading an ebook from a blob

Blobs are random-access sources, so the same API works for local files,
ebook entries inside an archive, or remote sources. The source position is
never modified and the caller keeps ownership. The caller must keep the blob
usable until all resource data (including the cover) has been consumed.

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

if cover, ok := ebook.Resources.CoverImage(); ok {
    rc, err := cover.Open()
    if err != nil {
        log.Fatal(err)
    }
    defer rc.Close()

    // Stream the cover, e.g. write it to a file:
    f, err := os.Create(cover.Name)
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
cover, ok := ebook.Resources.CoverImage()
if !ok {
    log.Fatal("no cover")
}
data, err := cover.Data() // reads the whole cover into a []byte
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Cover %q (%s): %d bytes\n", cover.Name, cover.MediaType, len(data))
```

### Overriding the safety limits

By default the library applies conservative safety limits (100 MB max resource
size, 100 MB max single MOBI record, 256 max EXTH records). Override them
with functional options:

```go
ebook, err := ebook.Read(
    "books/my-book.epub",
    ebook.WithMaxResourceSize(10*1024*1024), // 10 MB
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

### Working with resources

```go
// All manifest resources (images, stylesheets, etc.) in manifest order:
for _, r := range ebook.Resources.All() {
    fmt.Println(r.Id, r.ResolvedHref, r.MediaType, r.Href, r.Properties)
    // Stream lazily without loading everything into memory:
    rc, err := r.Open()
    if err != nil {
        log.Fatal(err)
    }
    defer rc.Close()
    // or: data, err := r.Data()
}

// Spine reading order, including linear="no" items with Linear flag:
for _, it := range ebook.Resources.ReadingOrder() {
    fmt.Println(it.Resource.ResolvedHref, "linear=", it.Linear)
}

// Lookup by manifest ID or resolved href (path.Clean applied):
if r, ok := ebook.Resources.GetByID("cover"); ok {
    fmt.Println("cover:", r.ResolvedHref)
}
if r, ok := ebook.Resources.GetByHref("OEBPS/images/cover.jpg"); ok {
    fmt.Println("found:", r.Id)
}

// Cover via registry:
if cover, ok := ebook.Resources.CoverImage(); ok {
    data, _ := cover.Data() // convenience wrapper around Open()
    _ = data
}
```

### Converting resources (transformers)

Resources are exposed lazily via `Open()`. Use `OpenAs` to convert on the fly
between MIME types via a streaming `Transformer` registry. This is lazy as well:
no data is buffered until you read.

```go
import (
    "context"
    "errors"
    "io"

    "github.com/f0d0r/margaret-ebook-library/pkg/errs"
    "github.com/f0d0r/margaret-ebook-library/pkg/mediatype"
    "github.com/f0d0r/margaret-ebook-library/pkg/model"
)

ctx := context.Background()

// Single resource: XHTML/HTML/MobiHTML -> plain text
// MediaType can include charset params; they are normalized (case-insensitive,
// "; charset=..." stripped).
for _, r := range ebook.Resources.All() {
    rc, err := r.OpenAs(ctx, mediatype.PlainText)
    if err != nil {
        if errors.Is(err, errs.ErrNoTransformer) {
            // no converter for e.g. image/jpeg -> text/plain
            continue
        }
        log.Fatal(err)
    }
    text, err := io.ReadAll(rc)
    rc.Close()
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(text)
}

// Convenience: read fully converted
data, err := ebook.Resources.All()[0].DataAs(ctx, mediatype.PlainText)

// Whole book as one plain-text stream (linear spine only, lazy concatenation):
rc, err := ebook.Resources.OpenReadingOrderAs(ctx, mediatype.PlainText)
if err != nil {
    log.Fatal(err)
}
defer rc.Close()
plain, err := io.ReadAll(rc) // search indexing, LLM input, TTS, CLI reading
if err != nil {
    log.Fatal(err)
}
fmt.Println(string(plain))
```

Constants are compile-time safe: `model.MediaTypePlainText`, `model.MediaTypeXHTML`,
`model.MediaTypeHTML`, `model.MediaTypeMobiHTML`, etc. (aliases of `pkg/mediatype`).

#### Custom transformers and isolated registries

`Transformer` is a single edge `From() -> To()` over MIME types:

```go
type Transformer interface {
    From() string
    To()   string
    Transform(ctx context.Context, r io.Reader) (io.ReadCloser, error)
}
```

`Resource.OpenAs` uses the global `converter.DefaultRegistry` (pre-populated with
`application/xhtml+xml`, `text/html`, `application/x-mobipocket-html` -> `text/plain`
streaming via `html.Tokenizer`). For tests or custom DI, use an isolated registry:

```go
import "github.com/f0d0r/margaret-ebook-library/pkg/converter"

reg := converter.NewRegistry(myTransformer)
rc, err := resource.OpenAsWithRegistry(ctx, reg, mediatype.PlainText)
if err != nil {
    if errors.Is(err, errs.ErrNoTransformer) {
        // no path from resource.MediaType to target
    }
}

// Share logic across multiple source types without per-converter slices:
converter.RegisterAliases(reg, func(from, to string) converter.Transformer {
    return myHTMLToTextTransformer{from: from, to: to}
}, mediatype.PlainText, mediatype.XHTML, mediatype.HTML, mediatype.MobiHTML)

// Multi-hop chaining is resolved via BFS shortest path:
// if you register A->B and B->C, OpenAs(A, C) automatically chains B.
```

Registering is thread-safe via `Registry.Register`.

### Hashing a book

```go
hash, err := ebook.CalculateFileHash("books/my-book.epub")
if err != nil {
    log.Fatal(err)
}
fmt.Println(hash) // hex-encoded sha256
```