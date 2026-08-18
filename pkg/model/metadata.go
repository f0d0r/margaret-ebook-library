package model

import (
	"errors"
	"io"
)

// Metadata represents the extracted core information of an e-book.
// It normalized data across different e-book formats like EPUB, MOBI, etc.
type Metadata struct {
	// Title is the main title of the e-book.
	Title string

	// Authors are the creators or writers of the book.
	Authors []string

	// Description is a brief summary or description of the book.
	Description string

	// Languages is the language(s) of the book.
	Languages []string

	// Cover is the cover image of the book.
	Cover *Resource

	// FileType indicates the original format from which this metadata was parsed.
	FileType FileType
}

// Resource represents an embedded file of an e-book (currently the cover
// image). Its content is exposed lazily through Open so callers can stream it
// without materializing the whole file in memory.
type Resource struct {
	Id        string
	Name      string
	MediaType string

	// Size is the declared uncompressed size of the resource in bytes.
	Size int64

	// Open returns a reader over the resource's content. The returned reader
	// must be closed by the caller. Depending on the format reader's safety
	// limits, the reader rejects streams that exceed the configured maximum
	// size instead of silently truncating them.
	Open func() (io.ReadCloser, error)
}

// Data reads the full resource content into memory. It is a convenience
// wrapper around Open; callers that want to avoid loading the whole resource
// at once should use Open directly.
func (r *Resource) Data() ([]byte, error) {
	if r == nil || r.Open == nil {
		return nil, errors.New("resource is not openable")
	}
	rc, err := r.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	return io.ReadAll(rc)
}
