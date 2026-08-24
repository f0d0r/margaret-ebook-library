package ebook

import (
	"fmt"
	"os"
	"strings"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
	"github.com/f0d0r/margaret-ebook-library/internal/registry"
)

// Re-exported domain types for convenient single-import usage.
// Importing "github.com/f0d0r/margaret-ebook-library" gives access to the
// most common types without needing a second import of "book".
type Book = book.Book
type Metadata = book.Metadata
type Resource = book.Resource
type ResourceSet = book.ResourceSet
type ReadingOrderItem = book.ReadingOrderItem
type FileType = book.FileType
type Blob = book.Blob

const (
	EPUB = book.EPUB
	MOBI = book.MOBI
)

var (
	ErrUnsupportedFormat    = book.ErrUnsupportedFormat
	ErrLimitExceeded        = book.ErrLimitExceeded
	ErrNoTransformer        = book.ErrNoTransformer
	ErrUnsupportedMediaType = book.ErrUnsupportedMediaType
)

var (
	NewFileBlob = book.NewFileBlob
	NewPathBlob = book.NewPathBlob
	// NewBytesBlob and NewReaderAtBlob are advanced Blob factories kept
	// for 0.1.0 for archive/remote and testing use cases. Most consumers
	// only need Read(path) or NewPathBlob/NewFileBlob.
	NewBytesBlob    = book.NewBytesBlob
	NewReaderAtBlob = book.NewReaderAtBlob
)

// Read reads an ebook (metadata and resources) from the specified file path.
//
// It trims leading and trailing whitespace from the provided path and rejects
// empty paths with [ErrUnsupportedFormat]. It selects the appropriate
// reader implementation from the internal registry based on the file format.
// The selected reader is then used to extract and return the ebook.
//
// An optional [Option] can be passed to override the library-wide safety
// limits; when omitted, the defaults of [book.DefaultConfig] are used.
//
// If no suitable reader is found for the format, it returns an error wrapping
// [ErrUnsupportedFormat].
func Read(path string, opts ...Option) (Book, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, ErrUnsupportedFormat
	}
	return read(NewPathBlob(path), path, opts)
}

// ReadFromBlob reads an ebook (metadata and resources) from a random-access
// source such as a local file, an e-book entry inside an archive, or a remote
// source.
//
// It selects the appropriate reader implementation from the internal registry
// based on the file format detected purely from the contents. The selected
// reader is then used to extract and return the ebook.
//
// An optional [Option] can be passed to override the library-wide safety
// limits; when omitted, the defaults of [book.DefaultConfig] are used.
//
// The caller retains ownership of the blob and must keep it usable until
// all resource data (including the cover) has been consumed.
// The file position of the source is never modified.
//
// If no suitable reader is found for the format, it returns an error wrapping
// [ErrUnsupportedFormat].
func ReadFromBlob(b Blob, opts ...Option) (Book, error) {
	return read(b, "", opts)
}

// ReadFromFile reads an ebook (metadata and resources) from an already-open
// file.
//
// It is a convenience wrapper around [ReadFromBlob]. The caller retains
// ownership of the file and must keep it open until all resource data
// (including the cover) has been consumed.
//
// An optional [Option] can be passed to override the library-wide safety
// limits; when omitted, the defaults of [book.DefaultConfig] are used.
func ReadFromFile(f *os.File, opts ...Option) (Book, error) {
	b, err := NewFileBlob(f)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect file %s: %w", f.Name(), err)
	}
	return read(b, f.Name(), opts)
}

func read(b Blob, name string, opts []Option) (Book, error) {
	r := registry.New(resolveOptions(opts))
	reader, err := r.ReaderForBlob(b)
	if err != nil {
		return nil, fmt.Errorf("failed to get reader for %s: %w", name, err)
	}
	ebook, err := reader.Read(b)
	if err != nil {
		return nil, fmt.Errorf("failed to read ebook for %s: %w", name, err)
	}
	return ebook, nil
}

// Config is a convenience alias for the library-wide safety
// limits used internally to configure the format readers.
// Deprecated: use Option instead.
type Config = config.Config
