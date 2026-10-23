# Margaret Ebook Library

[![Build Status](https://github.com/f0d0r/margaret-ebook-library/actions/workflows/build.yml/badge.svg)](https://github.com/f0d0r/margaret-ebook-library/actions/workflows/build.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/f0d0r/margaret-ebook-library.svg)](https://pkg.go.dev/github.com/f0d0r/margaret-ebook-library)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.26.8-blue)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Margaret is a format-agnostic Go library that presents every ebook — whether EPUB, MOBI, KF8/AZW3, PalmDOC, eReader, FictionBook (FB2, FB3), LIT, CHM or LRF — through a single, stable `Book` abstraction. Instead of surfacing format-specific internals like EPUB manifests and spines or MOBI PDB records, it hides those details behind normalized metadata and a unified `ResourceSet`. Current format support is EPUB, MOBI, PalmDOC, FB, LIT, eReader, CHM and LRF reading.

## Features

- Single stable `Book` abstraction over all supported formats
- Normalized metadata (title, authors, description, subjects, languages,
  identifiers, publisher, publication date, series)
- Lazy streaming access to resources (no full in-memory load)
- On-the-fly conversion of text resources to plain text (search indexing, LLM input, TTS)
- Content fingerprinting (MinHash / SimHash) for deduplication and similarity
- Conservative safety limits with functional-option overrides

## Supported formats

| Format                        | Read          | Write |
| ----------------------------- | ------------- | ----- |
| EPUB (`.epub`)                 | ✅            | —     |
| KEPUB (`.kepub.epub`)          | ✅ (as EPUB)  | —     |
| MOBI6 (`.mobi`, `.azw`, `.prc`) | ✅  | —     |
| KF8 / MOBI8 (`.mobi`, `.azw`, `.azw3`, `.prc`) | ✅ | —  |
| PalmDOC (`.prc`, `.pdb`) | ✅ (as `PALMDOC`) | — |
| FB2 / FBZ (`.fb2`, `.fbz`, `.fb2.zip`) | ✅   | —     |
| FB3 (`.fb3`) | ✅  | — |
| LIT (`.lit`)       | ✅  | — |
| eReader (`.pdb`) | ✅ (as `EREADER`) | — |
| CHM (`.chm`) | ✅ | — |
| LRF (`.lrf`, Sony BBeB) | ✅ | — |

> **KEPUB status:** a KEPUB is a valid EPUB archive whose content documents
> carry Kobo rendering markup (`div#book-columns`/`div#book-inner` wrappers,
> sentence-level `koboSpan` spans). It reads as `FileType() == EPUB` — the
> container-level differences are rendering-only, so there is no separate
> `KEPUB` file type and no new API surface. Detection is content-based, so
> the `.kepub.epub` filename is irrelevant to `Read`/`ReadFromBlob`.
>
> **DRM status:** DRM-protected books cannot be read and are rejected with
> `ebook.ErrDRM` (matched with `errors.Is`): EPUB/KEPUB files with a
> non-font-obfuscation `META-INF/encryption.xml` entry (Adobe ADEPT) or a
> non-empty `rights.xml` with encrypted spine content (Kobo kdrm), MOBI
> files with DRM records or PalmDOC/MOBI text encryption, and eReader files
> with DRM compression markers. LIT files with a
> passport-locked (`/DRMStorage/Licenses/EUL`) container are rejected outright;
> sealed LIT content stays listed but fails with `ErrDRM` when opened
> (section decryption is out of scope). Font obfuscation
> (`adobe#RC`, `idpf.org/2008/embedding`) and leftover/empty `rights.xml`
> files are not treated as DRM.
>
> **LRF status:** the text flow is synthesized by the reader (UTF-16LE,
> `application/x-lrf-text`) and is the only reading-order item; images and
> fonts are manifest resources, the header thumbnail is the cover, and inline
> plots are dropped from plain text.

## Requirements

- Go 1.26.8 or later (enforced via the `toolchain` directive in `go.mod`).

## Installation

```sh
go get github.com/f0d0r/margaret-ebook-library@latest
```

Full API documentation: https://pkg.go.dev/github.com/f0d0r/margaret-ebook-library

## Quickstart

```go
package main

import (
	"fmt"
	"log"

	ebook "github.com/f0d0r/margaret-ebook-library"
)

func main() {
	b, err := ebook.Read("books/my-book.epub")
	if err != nil {
		log.Fatal(err)
	}

	meta := b.Metadata()
	fmt.Println("Title:", meta.Title)
	fmt.Println("Authors:", meta.Authors)
	fmt.Println("Languages:", meta.Languages)
	fmt.Println("FileType:", b.FileType())
	fmt.Println("Version:", b.Version())
}
```

## Usage

Import the library:

```go
import ebook "github.com/f0d0r/margaret-ebook-library"
```

For advanced use (isolated registries, media-type constants) also import:

```go
import (
    "github.com/f0d0r/margaret-ebook-library/book"
    "github.com/f0d0r/margaret-ebook-library/converter"
    "github.com/f0d0r/margaret-ebook-library/mediatype"
)
```

### Reading an ebook from a file path

The simplest way is to pass the file path directly:

```go
b, err := ebook.Read("books/my-book.epub")
if err != nil {
    // handle the error, e.g. with errors.Is(err, ebook.ErrUnsupportedFormat)
}
```

The snippet above assumes:

```go
import (
	"errors"

	ebook "github.com/f0d0r/margaret-ebook-library"
)
```

### Reading an ebook from an already-open file

Use this when you already have the file open:

```go
f, err := os.Open("books/my-book.mobi")
if err != nil {
    log.Fatal(err)
}
defer f.Close()

b, err := ebook.ReadFromFile(f)
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
b, err := ebook.ReadFromBlob(ebook.NewPathBlob("books/my-book.epub"))
if err != nil {
    log.Fatal(err)
}
// also available: ebook.NewBytesBlob(data), ebook.NewReaderAtBlob(r, size), book.NewFileBlob(f)
```

### Using the ebook

```go
import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	ebook "github.com/f0d0r/margaret-ebook-library"
)
```

```go
meta := b.Metadata()
fmt.Println("Title:", meta.Title)
fmt.Println("Authors:", strings.Join(meta.Authors, ", "))
fmt.Println("Languages:", meta.Languages)
fmt.Println("Description:", meta.Description)
fmt.Println("Subjects:", strings.Join(meta.Subjects, ", "))
fmt.Println("Publisher:", meta.Publisher)
fmt.Println("Publication Date:", meta.PublicationDate)
fmt.Println("Series:", meta.Series)
fmt.Println("Series Index:", meta.SeriesIndex)
for _, id := range meta.Identifiers {
    fmt.Printf("Identifier: %s (Scheme: %s)\n", id.Value, id.Scheme)
}
fmt.Println("FileType:", b.FileType())
fmt.Println("Version:", b.Version())

if cover, ok := b.Resources().CoverImage(); ok {
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
cover, ok := b.Resources().CoverImage()
if !ok {
    log.Fatal("no cover")
}
data, err := cover.Data() // reads the whole cover into a []byte
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Cover %q (%s): %d bytes\n", cover.Name, cover.MediaType, len(data))
```

### Identifiers

Identifiers are exposed as raw scheme/value pairs. Scheme prefixes such as
`urn:isbn:` or `calibre:` are moved to `Identifier.Scheme`; values are
otherwise not validated or normalized.

```go
for _, id := range meta.Identifiers {
    fmt.Printf("%s: %s\n", id.Scheme, id.Value)
}

if isbn, ok := meta.Identifier("ISBN"); ok {
    fmt.Println("ISBN:", isbn)
}
```

`Metadata.Identifier` returns the first match case-insensitively. For EPUB,
`dc:source` identifiers (the source publication the ebook was derived from)
follow the publication's own `dc:identifier` values. FB2 and FB3 follow the
same own-before-source order: FB2 reads `document-info/id`,
`publish-info/isbn`, and `document-info/src-url`; FB3 reads the root UUID,
`document-info/isbn`/`@doi`, `paper-publish-info/isbn`,
`periodical/issn`, and `document-info/@src-url`. LIT reads the embedded
OEB 1.0 OPF's `dc:Identifier` and `dc:Source` values, including their
`scheme` attributes.

### Subjects

`Metadata.Subjects` lists the subject headings or keywords declared by the
e-book metadata, in source order. Semicolon-separated values are split into
separate entries; parts are whitespace-trimmed and empty entries are dropped.
Commas and duplicates are preserved, except for the FB2, FB3 and LRF rules
below, source order is kept, and values are not mapped to any controlled
vocabulary.

For EPUB and KEPUB, each `dc:subject` element is split on semicolons. LIT and
PalmDOC read the OEB 1.x `dc-metadata/Subject` elements the same way. For
MOBI and KF8, EXTH record 105 (subject) is used, including multiple records
and semicolon-separated values. A non-empty KF8 subject list is preferred,
falling back to the MOBI6 EXTH values.

For FB2, `title-info/genre` elements are read in source order together with
the comma-separated `title-info/keywords` value. If `title-info` declares
neither, `src-title-info` is used instead; the two sections are never merged.
FB2 keyword values are split on commas and semicolons. The combined FB2 list
is deduplicated case-insensitively, keeping the first occurrence and spelling.

For FB3, each `fb3-classification/subject` element is used in source order,
with whitespace collapsed per the XSD token type; values are not split.
`fb3-relations/subject` elements describe people and are not subjects, and
`class`, `udc`, and `bbk` are not included. The FB3 list is deduplicated
case-insensitively, keeping the first occurrence and spelling.

For LRF, the repeatable `BookInfo/Category` elements and the single
`BookInfo/Classification` value are read in source order, with each value split
on commas and semicolons. `Info/Keyword` elements are appended whole, without
splitting. `BookInfo/Label` is not used. The combined LRF list is deduplicated
case-insensitively, keeping the first occurrence and spelling.

### Publisher and publication date

For EPUB and LIT, `Metadata.Publisher` is the first non-empty `dc:publisher`
value from the OEB package, with surrounding whitespace trimmed.
`Metadata.PublicationDate` is the raw `dc:date` string: it is not parsed or
validated, so it may contain arbitrary text. Only an EPUB 2 date carrying
`event="publication"` (or `opf:event="publication"`) or a date without an
event is considered; dates carrying any other event (for example `creation`
or `modification`) are ignored. An explicit publication date is preferred;
otherwise the first non-empty unqualified `dc:date` is returned.
`dcterms:modified` is not treated as the publication date.

For MOBI and KF8, `Metadata.Publisher` is the first non-empty EXTH publisher
record (type 101) and `Metadata.PublicationDate` is the raw EXTH publishing
date record (type 106). Both are decoded with the book's text encoding and
whitespace-trimmed; the date is returned as declared, not parsed or
validated. A KF8 EXTH value is preferred when non-empty, otherwise the MOBI6
EXTH is used. PalmDOC books carry both fields in the embedded HTML metadata,
read under the same rules.

For FB2 and FB3, `Metadata.Publisher` and `Metadata.PublicationDate` are
extracted from the publication metadata sections:

- **FB2** (`<publish-info>`): `<publisher>` element text and `<year>` or `<date>`
  element text.
- **FB3** (`<paper-publish-info>`): `publisher` attribute and `year` attribute.

For eReader, `Metadata.Publisher` is the publisher field of the Dropbook
metadata record. For LRF, it is the `BookInfo/Publisher` element. Neither
format exposes a publication date, so `Metadata.PublicationDate` is empty.

For CHM, the native `#SYSTEM` metadata carries no publisher, date or ISBN.
Declared default-topic meta tags (`publisher`, `date`, and Dublin Core forms)
are used when present. Otherwise the publisher is taken from calibre-style
`span.imprint` or labeled table cells, and the ISBN from declared meta tags,
`span.isbn` or labeled table cells. ISBN-specific candidates are reduced to
their digits (ASCII `0`-`9` and `X`) before checksum validation, and the
canonical digits are stored, so surrounding text such as `(pbk.)` is ignored.
Generic Dublin Core identifiers are parsed strictly and are not reduced to
digits. Free-text "pub date" heuristics are not treated as a publication
date.

### Series

`Metadata.Series` is the series name and `Metadata.SeriesIndex` its position.
Both are whitespace-trimmed. The index is returned as the raw declared string:
it is not converted to a number and there is no Calibre-style `1.0` default, so
it is empty when the source declares no position. Only one series is exposed;
the first non-empty series wins.

For EPUB, KEPUB, LIT and PalmDOC, an EPUB 3 `belongs-to-collection` meta with a
`collection-type="series"` refinement is preferred, with the position taken
from its `group-position` refinement. A canonical series without a
`group-position` yields an empty index, even when a `calibre:series_index`
exists. When no canonical series is declared, `calibre:series` and
`calibre:series_index` are used. LIT and PalmDOC apply the same rules to the
OEB 1.x metadata they embed, including `x-metadata` calibre values.

For FB2, the first non-empty direct `title-info/sequence` supplies the `name`
and `number` attributes; `publish-info/sequence` is the fallback.
`src-title-info` and nested sequences are ignored.

For FB3, the first non-empty direct `fb3-description/sequence` is used, with
the name from `sequence/title/main` and the index from `@number`. Nested
sequences are ignored. If there is no such sequence, the first non-empty
`paper-publish-info/sequence` text is used as the name, with an empty index.

MOBI/KF8, eReader, CHM and LRF declare no standard series metadata, so
`Metadata.Series` and `Metadata.SeriesIndex` remain empty for them.

### Overriding the safety limits

By default the library applies conservative safety limits (100 MB max resource
size, 100 MB max single MOBI record, 256 max EXTH records). Override them
with functional options:

```go
b, err := ebook.Read(
    "books/my-book.epub",
    ebook.WithMaxResourceSize(10*1024*1024), // 10 MB
)
if err != nil {
    log.Fatal(err)
}
```

MOBI-specific limits can be adjusted the same way:

```go
b, err := ebook.Read(
    "books/my-book.mobi",
    ebook.WithMaxRecordSize(50*1024*1024), // max single PDB record
    ebook.WithMaxExthRecords(512),         // max EXTH records to parse
)
if err != nil {
    log.Fatal(err)
}
```

### KF8 / MOBI8 fallback behaviour

A joint MOBI6+KF8 file (a `.mobi`/`.azw` carrying both renderings, separated by
a `BOUNDARY` record) is read from its KF8 part. When that KF8 part cannot be
parsed — corrupt index tables, a broken flow table, unreadable markup — the
reader falls back to the intact MOBI6 content, which is the same content the
book exposes to older readers. This is the default so that a damaged KF8 part
never costs the reader the book.

The fallback is configurable. Turn it off for strict behaviour, so a KF8 parse
failure is reported instead of silently serving different content:

```go
b, err := ebook.Read(
    "books/dual.mobi",
    ebook.WithKF8FallbackToMOBI6(false), // report KF8 parse failures
)
```

The parse error does not abort the read: the returned content resource reports
it from `Open` (matched with `errors.Is` against `ebook.ErrCorrupt` or
`ebook.ErrParseFailed`), so metadata and the remaining resources stay usable.
Two cases never fall back: a standalone KF8 file (there is no MOBI6 rendering to
serve), and a DRM-protected file, which is rejected with `ebook.ErrDRM`.

### Working with resources

```go
import (
	"fmt"
	"log"

	ebook "github.com/f0d0r/margaret-ebook-library"
)
```

```go
// All manifest resources (images, stylesheets, etc.) in manifest order:
for _, r := range b.Resources().All() {
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
for _, it := range b.Resources().ReadingOrder() {
    fmt.Println(it.Resource.ResolvedHref, "linear=", it.Linear)
}

// Lookup by manifest ID or resolved href (path.Clean applied):
if r, ok := b.Resources().GetByID("cover"); ok {
    fmt.Println("cover:", r.ResolvedHref)
}
if r, ok := b.Resources().GetByHref("OEBPS/images/cover.jpg"); ok {
    fmt.Println("found:", r.Id)
}

// Cover:
if cover, ok := b.Resources().CoverImage(); ok {
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

    "github.com/f0d0r/margaret-ebook-library/mediatype"
)

ctx := context.Background()

// Single resource: XHTML/HTML/MobiHTML/FB2Body/FB3Body/EreaderPML/LRFText
// -> plain text
// MediaType can include charset params; they are normalized (case-insensitive,
// "; charset=..." stripped).
for _, r := range b.Resources().All() {
    rc, err := r.OpenAs(ctx, mediatype.PlainText)
    if err != nil {
        if errors.Is(err, ebook.ErrNoTransformer) {
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
data, err := b.Resources().All()[0].DataAs(ctx, mediatype.PlainText)

// Whole book as one plain-text stream (linear spine only, lazy concatenation):
rc, err := b.Resources().OpenReadingOrderAs(ctx, mediatype.PlainText)
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

Constants are compile-time safe: `mediatype.PlainText`, `mediatype.XHTML`,
`mediatype.HTML`, `mediatype.MobiHTML`, `mediatype.FB2Body`, `mediatype.FB3Body`,
`mediatype.EreaderPML`, `mediatype.LRFText`, etc.

#### Storage charset and byte-identical reads

`Resource.Open()` (and `Data()`) always serves the stored bytes exactly as
they are: decompressed/decrypted, but **never** transcoded — XML/HTML
declarations, BOMs and the original encoding are preserved.
`Resource.Charset` names that encoding: an explicit IANA label
(`"windows-1251"`, `"utf-16le"`, ...), `"utf-8"` when nothing else is
declared, or `"binary"` for non-textual content (`ebook.CharsetUTF8` /
`ebook.CharsetBinary`). It is never empty on resources returned by `Read`.

`OpenAs`/`DataAs`/`OpenReadingOrderAs` decode to UTF-8 centrally before the
transformer chain, so transformers always see UTF-8. To request a non-UTF-8
output (e.g. a legacy tool expecting Latin-2), use the `...WithCharset`
variants; unknown charset labels return an error:

```go
rc, err := r.OpenAsWithCharset(ctx, mediatype.PlainText, "windows-1250")
if err != nil {
    log.Fatal(err)
}
defer rc.Close()

// Whole book in the requested charset:
rc, err = b.Resources().OpenReadingOrderAsWithCharset(ctx, mediatype.PlainText, "windows-1250")
```

Synthesized resources are the documented exceptions: LIT spine documents are
built by the reader and always UTF-8 (`Charset == ebook.CharsetUTF8`), while
the LRF text flow is a synthesized UTF-16LE stream (`Charset == "utf-16le"`).

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
`application/xhtml+xml`, `text/html`, `application/x-mobipocket-html`, `application/x-fictionbook-body+xml`, `application/x-ereader-pml` -> `text/plain`). For tests or custom DI, use an isolated registry:

```go
import "github.com/f0d0r/margaret-ebook-library/converter"

reg := converter.NewRegistry(myTransformer)
rc, err := resource.OpenAsWithRegistry(ctx, reg, mediatype.PlainText)
if err != nil {
    if errors.Is(err, ebook.ErrNoTransformer) {
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
import "github.com/f0d0r/margaret-ebook-library/tools"

hash, err := tools.CalculateFileHash("books/my-book.epub")
if err != nil {
    log.Fatal(err)
}
fmt.Println(hash) // hex-encoded sha256
```

Or from a blob:

```go
hash, err := tools.CalculateFileHashFromBlob(ebook.NewPathBlob("books/my-book.epub"))
```

### ISBN validation and conversion

`IsValidISBN` accepts ISBN-10 and ISBN-13 identifiers in common forms such as
`urn:isbn:` prefixes, `ISBN` labels, spaces, and hyphens:

```go
if tools.IsValidISBN("urn:isbn:978-0-306-40615-7") {
    // valid ISBN-13
}

isbn, ok := tools.ParseISBN("ISBN-10: 0-306-40615-2")
if ok {
    fmt.Println(isbn, isbn.Kind()) // 0306406152 10

    isbn13, _ := isbn.ToISBN13()
    fmt.Println(isbn13) // 9780306406157

    isbn10, _ := isbn13.ToISBN10()
    fmt.Println(isbn10) // 0306406152
}
```

`ToISBN10` returns `false` for 979-prefixed ISBN-13 values, which have no
ISBN-10 equivalent.

### Content fingerprinting (MinHash / SimHash)

Fingerprints are computed over the normalized plain-text stream and are streaming-friendly — they implement `io.Writer` so they can be attached with `io.TeeReader`/`io.MultiWriter` in a single pass:

```go
import (
    "context"
    "io"

    "github.com/f0d0r/margaret-ebook-library/mediatype"
    "github.com/f0d0r/margaret-ebook-library/tools"
)

ctx := context.Background()

// One-shot helper (injects "\n" between chapters):
fp, err := tools.FingerprintContent(ctx, book,
    tools.WithMinHash(tools.MinHashConfig{NumHashes: 128, ShingleSize: 5}),
    tools.WithSimHash(),
)
fmt.Println(fp.MinHash, fp.SimHash)
fmt.Println(tools.Jaccard(fp.MinHash, other.MinHash))
fmt.Println(tools.Hamming(fp.SimHash, other.SimHash))

// Streaming while copying plain text elsewhere:
rc, handle, err := tools.OpenReadingOrderWithFingerprint(ctx, book.Resources(), mediatype.PlainText,
    tools.WithMinHash(tools.MinHashConfig{NumHashes: 128, ShingleSize: 5}),
    tools.WithSimHash(),
)
defer rc.Close()
io.Copy(io.Discard, rc) // or dst file
fp = handle.Result()
```

`MinHasher` and `SimHasher` also implement `io.Writer` directly:

```go
mh := tools.NewMinHasher(tools.WithShingleSize(5), tools.WithNumHashes(128))
sh := tools.NewSimHasher()

// Fan-out while streaming plain text (single pass):
rc, _ := book.Resources().OpenReadingOrderAs(ctx, mediatype.PlainText)
defer rc.Close()
tee := io.TeeReader(rc, io.MultiWriter(mh, sh))
io.Copy(io.Discard, tee)
fmt.Println(mh.Signature(), sh.Sum64())
```

## Error handling

Sentinel errors are matched with `errors.Is`:

| Error | Meaning |
| ----- | ------- |
| `ebook.ErrUnsupportedFormat` | Unknown or unsupported ebook format |
| `ebook.ErrDRM` | The book is DRM protected and cannot be read |
| `ebook.ErrCorrupt` | The file is malformed or truncated (unreadable container, recovered parse panic) |
| `ebook.ErrParseFailed` | A format-specific structure could not be parsed (headers, indices, metadata) |
| `ebook.ErrLimitExceeded` | A safety limit (resource size, record size, EXTH count) was hit — raise it with a functional option |
| `ebook.ErrNoTransformer` | No conversion path to the requested media type |
| `ebook.ErrUnsupportedMediaType` | The requested media type itself is not supported |

```go
import (
	"errors"

	ebook "github.com/f0d0r/margaret-ebook-library"
)

b, err := ebook.Read(path)
if err != nil {
	if errors.Is(err, ebook.ErrUnsupportedFormat) {
		log.Fatal("unsupported format")
	}
	if errors.Is(err, ebook.ErrLimitExceeded) {
		log.Fatal("safety limit exceeded, retry with a higher limit")
	}
	log.Fatal(err)
}
```

## Development

For contributors working on the library itself:

```sh
git clone https://github.com/f0d0r/margaret-ebook-library.git
cd margaret-ebook-library
go test ./...
```

## License

MIT — see [LICENSE](LICENSE).
