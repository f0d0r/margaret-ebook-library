package model

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/f0d0r/margaret-ebook-library/pkg/converter"
	"github.com/f0d0r/margaret-ebook-library/pkg/errs"
	"github.com/f0d0r/margaret-ebook-library/pkg/mediatype"
)

type Ebook struct {
	Metadata  Metadata
	Resources *ResourceSet
	FileType  FileType

	// Version is the format-specific version declared by the e-book: the OPF
	// package version (e.g. "3.0") for EPUB, the MOBI header version (e.g.
	// "6", "8", or "6/8" for dual MOBI/KF8 files) for MOBI.
	Version string
}

// Metadata represents the extracted core information of an e-book.
// It normalizes data across different e-book formats like EPUB, MOBI, etc.
type Metadata struct {
	// Title is the main title of the e-book.
	Title string

	// Authors are the creators or writers of the book.
	Authors []string

	// Description is a brief summary or description of the book.
	Description string

	// Languages is the language(s) of the book.
	Languages []string
}

// Resource represents an embedded file of an e-book (e.g. a chapter,
// image, stylesheet, or the cover image). Its content is exposed lazily
// through Open so callers can stream it without materializing the whole
// file in memory.
type Resource struct {
	Id           string
	Name         string
	MediaType    string
	Href         string
	ResolvedHref string
	Properties   string

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

// OpenAs opens the resource and converts its content to targetMIME if needed.
// It uses the global converter.DefaultRegistry. If the resource's MediaType
// already matches targetMIME (after normalization), it simply calls Open.
func (r *Resource) OpenAs(ctx context.Context, targetMIME string) (io.ReadCloser, error) {
	return r.openAsWithRegistry(ctx, nil, targetMIME)
}

// OpenAsWithRegistry is like OpenAs but uses the provided registry. If reg is
// nil, the global DefaultRegistry is used. This allows isolated registries in
// tests and custom DI setups.
func (r *Resource) OpenAsWithRegistry(ctx context.Context, reg *converter.Registry, targetMIME string) (io.ReadCloser, error) {
	return r.openAsWithRegistry(ctx, reg, targetMIME)
}

// DataAs reads the full resource content converted to targetMIME.
func (r *Resource) DataAs(ctx context.Context, targetMIME string) ([]byte, error) {
	rc, err := r.OpenAs(ctx, targetMIME)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	return io.ReadAll(rc)
}

// DataAsWithRegistry is like DataAs but uses the provided registry.
func (r *Resource) DataAsWithRegistry(ctx context.Context, reg *converter.Registry, targetMIME string) ([]byte, error) {
	rc, err := r.OpenAsWithRegistry(ctx, reg, targetMIME)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	return io.ReadAll(rc)
}

func (r *Resource) openAsWithRegistry(ctx context.Context, reg *converter.Registry, targetMIME string) (io.ReadCloser, error) {
	if r == nil {
		return nil, errors.New("resource is nil")
	}
	if r.Open == nil {
		return nil, errors.New("resource is not openable")
	}
	to := mediatype.Normalize(targetMIME)
	if to == "" {
		return nil, fmt.Errorf("%w: target MIME is empty", errs.ErrUnsupportedMediaType)
	}
	from := mediatype.Normalize(r.MediaType)
	if from == "" {
		return nil, fmt.Errorf("%w: resource media type is empty", errs.ErrUnsupportedMediaType)
	}
	if from == to {
		return r.Open()
	}
	if reg == nil {
		reg = converter.DefaultRegistry
	}
	path, ok := reg.FindPath(from, to)
	if !ok {
		return nil, fmt.Errorf("%w: from %s to %s", errs.ErrNoTransformer, from, to)
	}
	if len(path) == 0 {
		// Identity case already handled, but FindPath may return empty for identity.
		return r.Open()
	}
	rc, err := r.Open()
	if err != nil {
		return nil, err
	}
	current := io.Reader(rc)
	// Keep track of the current ReadCloser for cleanup on error.
	currentRC := rc
	for i, t := range path {
		nextRC, err := t.Transform(ctx, current)
		if err != nil {
			_ = currentRC.Close()
			return nil, fmt.Errorf("transform step %d (%s -> %s): %w", i, t.From(), t.To(), err)
		}
		// Next iteration will read from nextRC; currentRC will be closed when nextRC is closed
		// via the transformer's closer chain, but if the transformer does not wrap the closer,
		// we rely on the transformer to close it. The outermost RC is returned.
		current = nextRC
		currentRC = nextRC
	}
	// currentRC is the final transformed reader; it already chains closes.
	return currentRC, nil
}
