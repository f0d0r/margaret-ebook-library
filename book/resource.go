package book

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/f0d0r/margaret-ebook-library/converter"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

// Package book defines the core domain types for e-books.
//
// book does not contain concrete transformation logic, but for ergonomic
// convenience its Resource and ResourceSet types provide OpenAs methods
// that delegate to the converter package's DefaultRegistry. Strictly
// speaking this is a domain -> conversion-service coupling that is
// pragmatically accepted for 0.1.0 to keep the consumer API pleasant
// (resource.OpenAs(ctx, mediatype.PlainText)). A future breaking release
// may move conversion to a separate API (e.g. ebook.OpenAs), but for now
// the coupling is intentional and cycle-free (converter depends only on
// mediatype).

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
// It uses converter.DefaultRegistry. If the resource's MediaType already matches
// targetMIME (after normalization), it simply calls Open.
func (r *Resource) OpenAs(ctx context.Context, targetMIME string) (io.ReadCloser, error) {
	return r.openAsWithRegistry(ctx, nil, targetMIME)
}

// OpenAsWithRegistry is like OpenAs but uses the provided registry. If reg is
// nil, converter.DefaultRegistry is used.
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
		return nil, fmt.Errorf("%w: target MIME is empty", ErrUnsupportedMediaType)
	}
	from := mediatype.Normalize(r.MediaType)
	if from == "" {
		return nil, fmt.Errorf("%w: resource media type is empty", ErrUnsupportedMediaType)
	}
	if from == to {
		return r.Open()
	}
	if reg == nil {
		reg = converter.DefaultRegistry
	}
	path, ok := reg.FindPath(from, to)
	if !ok {
		return nil, fmt.Errorf("%w: from %s to %s", ErrNoTransformer, from, to)
	}
	if len(path) == 0 {
		return r.Open()
	}
	rc, err := r.Open()
	if err != nil {
		return nil, err
	}
	current := io.Reader(rc)
	currentRC := rc
	for i, t := range path {
		nextRC, err := t.Transform(ctx, current)
		if err != nil {
			_ = currentRC.Close()
			return nil, fmt.Errorf("transform step %d (%s -> %s): %w", i, t.From(), t.To(), err)
		}
		current = nextRC
		currentRC = nextRC
	}
	return currentRC, nil
}

// ReadingOrderItem represents a single spine entry in reading order.
// Linear is true unless the EPUB spine itemref has linear="no"; for MOBI it is always true.
type ReadingOrderItem struct {
	Resource *Resource
	Linear   bool
}

// ResourceSet holds all embedded resources of an e-book and the spine-derived reading order.
// It provides O(1) lookup by ID and by resolved href.
type ResourceSet struct {
	all          []*Resource
	byID         map[string]*Resource
	byHref       map[string]*Resource
	readingOrder []ReadingOrderItem
	cover        *Resource
}

// NewResourceSet builds a ResourceSet from the given slices.
//
// This constructor is exported for internal parser use and for unit tests
// within the book package. It is NOT part of the stable public surface
// re-exported by the root ebook package: consumers should treat
// ResourceSet as a read-only result of ebook.Read, not as a type they
// construct themselves. Future releases may make it internal without a
// major version bump for direct book importers.
//
// all is the full manifest in manifest order; readingOrder is spine order (may be subset of all, with Linear flags).
// cover may be nil or point to one of the resources in all. Maps are built from all using Id and ResolvedHref.
// GetByHref only indexes canonical ResolvedHref; query and fragment are ignored. Relative paths are not resolved.
// Duplicate handling:
//   - Duplicate ResolvedHref (cleanHref) in All(): first wins, All length decreases, second Id aliases to first resource.
//   - Duplicate Id with different ResolvedHref: second resource gets generated Id Id__dupN and remains in All().
func NewResourceSet(all []*Resource, readingOrder []ReadingOrderItem, cover *Resource) *ResourceSet {
	filteredAll := make([]*Resource, 0, len(all))
	for _, r := range all {
		if r == nil {
			continue
		}
		filteredAll = append(filteredAll, r)
	}
	filteredRO := make([]ReadingOrderItem, 0, len(readingOrder))
	for _, it := range readingOrder {
		if it.Resource == nil {
			continue
		}
		filteredRO = append(filteredRO, it)
	}

	byID := make(map[string]*Resource)
	byHref := make(map[string]*Resource)
	seenHref := make(map[string]*Resource)
	allCopy := make([]*Resource, 0, len(filteredAll))

	for _, orig := range filteredAll {
		hrefKey := ""
		if orig.ResolvedHref != "" {
			hrefKey = cleanHref(orig.ResolvedHref)
		}
		// Duplicate href: alias Id to existing, skip duplicate from All (no data loss, same file).
		if hrefKey != "" {
			if existing, ok := seenHref[hrefKey]; ok {
				if orig.Id != "" {
					if _, exists := byID[orig.Id]; !exists {
						byID[orig.Id] = existing
					}
				}
				continue
			}
		}
		// Handle duplicate Id with different href: generate new Id.
		r := orig
		if r.Id != "" {
			if _, exists := byID[r.Id]; exists {
				var newID string
				for i := 1; ; i++ {
					cand := fmt.Sprintf("%s__dup%d", r.Id, i)
					if _, exists2 := byID[cand]; !exists2 {
						newID = cand
						break
					}
				}
				copyRes := *r
				copyRes.Id = newID
				r = &copyRes
				if hrefKey != "" {
					hrefKey = cleanHref(r.ResolvedHref)
				}
			}
		}
		if hrefKey != "" {
			seenHref[hrefKey] = r
			byHref[hrefKey] = r
		}
		if r.Id != "" {
			byID[r.Id] = r
		}
		allCopy = append(allCopy, r)
	}

	// Canonicalize readingOrder to point to deduped All entries.
	roCopy := make([]ReadingOrderItem, 0, len(filteredRO))
	for _, it := range filteredRO {
		res := it.Resource
		if res.ResolvedHref != "" {
			if deduped, ok := seenHref[cleanHref(res.ResolvedHref)]; ok {
				res = deduped
			}
		}
		roCopy = append(roCopy, ReadingOrderItem{Resource: res, Linear: it.Linear})
	}

	// Cover alias: if cover href duplicates an existing href, point to existing.
	coverAliased := cover
	if cover != nil && cover.ResolvedHref != "" {
		if deduped, ok := seenHref[cleanHref(cover.ResolvedHref)]; ok {
			coverAliased = deduped
		} else {
			if cover.Id != "" {
				if _, exists := byID[cover.Id]; exists {
					var newID string
					for i := 1; ; i++ {
						cand := fmt.Sprintf("%s__dup%d", cover.Id, i)
						if _, exists2 := byID[cand]; !exists2 {
							newID = cand
							break
						}
					}
					copyCover := *cover
					copyCover.Id = newID
					coverAliased = &copyCover
				} else {
					byID[cover.Id] = coverAliased
				}
			}
			if _, exists := byHref[cleanHref(cover.ResolvedHref)]; !exists {
				byHref[cleanHref(cover.ResolvedHref)] = coverAliased
			}
		}
	} else if cover != nil && cover.Id != "" {
		if _, exists := byID[cover.Id]; !exists {
			byID[cover.Id] = coverAliased
		}
	}

	return &ResourceSet{
		all:          allCopy,
		byID:         byID,
		byHref:       byHref,
		readingOrder: roCopy,
		cover:        coverAliased,
	}
}

// GetByID returns the resource with the given manifest ID.
func (rs *ResourceSet) GetByID(id string) (*Resource, bool) {
	if rs == nil {
		return nil, false
	}
	r, ok := rs.byID[id]
	return r, ok
}

// GetByHref returns the resource with the given resolved href (canonicalized via cleanHref).
func (rs *ResourceSet) GetByHref(href string) (*Resource, bool) {
	if rs == nil {
		return nil, false
	}
	key := cleanHref(href)
	r, ok := rs.byHref[key]
	return r, ok
}

// All returns all resources in manifest order. The returned slice is a defensive copy.
func (rs *ResourceSet) All() []*Resource {
	if rs == nil {
		return nil
	}
	out := make([]*Resource, len(rs.all))
	copy(out, rs.all)
	return out
}

// ReadingOrder returns the spine-derived reading order, including linear="no" items with Linear==false.
// The returned slice is a defensive copy.
func (rs *ResourceSet) ReadingOrder() []ReadingOrderItem {
	if rs == nil {
		return nil
	}
	out := make([]ReadingOrderItem, len(rs.readingOrder))
	copy(out, rs.readingOrder)
	return out
}

// CoverImage returns the cover image resource, if any.
func (rs *ResourceSet) CoverImage() (*Resource, bool) {
	if rs == nil || rs.cover == nil {
		return nil, false
	}
	return rs.cover, true
}

// Len returns the number of resources in All().
func (rs *ResourceSet) Len() int {
	if rs == nil {
		return 0
	}
	return len(rs.all)
}

// OpenReadingOrderAs returns a single ReadCloser that lazily concatenates all
// linear reading-order resources converted to targetMIME. Only items with
// Linear==true are included; non-linear items (e.g. EPUB spine linear="no")
// are skipped. The stream is lazy: each chapter is opened and converted only
// when the previous one is exhausted.
func (rs *ResourceSet) OpenReadingOrderAs(ctx context.Context, targetMIME string) (io.ReadCloser, error) {
	return rs.OpenReadingOrderAsWithRegistry(ctx, nil, targetMIME)
}

// OpenReadingOrderAsWithRegistry is like OpenReadingOrderAs but uses the
// provided registry. If reg is nil, converter.DefaultRegistry is used.
func (rs *ResourceSet) OpenReadingOrderAsWithRegistry(ctx context.Context, reg *converter.Registry, targetMIME string) (io.ReadCloser, error) {
	if rs == nil {
		return nil, fmt.Errorf("resource set is nil")
	}
	to := mediatype.Normalize(targetMIME)
	if to == "" {
		return nil, fmt.Errorf("target MIME is empty")
	}
	var linear []*Resource
	for _, it := range rs.readingOrder {
		if it.Linear && it.Resource != nil {
			linear = append(linear, it.Resource)
		}
	}
	if len(linear) == 0 {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}
	if reg == nil {
		reg = converter.DefaultRegistry
	}
	return &multiReadCloser{
		ctx:       ctx,
		resources: linear,
		reg:       reg,
		target:    to,
		index:     0,
	}, nil
}

type multiReadCloser struct {
	ctx       context.Context
	resources []*Resource
	reg       *converter.Registry
	target    string
	index     int
	current   io.ReadCloser
	closed    bool
}

func (m *multiReadCloser) Read(p []byte) (int, error) {
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	if len(p) == 0 {
		return 0, nil
	}
	for {
		if m.current == nil {
			if m.index >= len(m.resources) {
				return 0, io.EOF
			}
			rc, err := m.resources[m.index].OpenAsWithRegistry(m.ctx, m.reg, m.target)
			if err != nil {
				return 0, err
			}
			m.current = rc
		}
		n, err := m.current.Read(p)
		if err == io.EOF {
			_ = m.current.Close()
			m.current = nil
			m.index++
			if n > 0 {
				return n, nil
			}
			continue
		}
		return n, err
	}
}

func (m *multiReadCloser) Close() error {
	if m.closed {
		return nil
	}
	m.closed = true
	if m.current != nil {
		err := m.current.Close()
		m.current = nil
		return err
	}
	return nil
}
