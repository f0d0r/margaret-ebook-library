package model

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/f0d0r/margaret-ebook-library/internal/util"
	"github.com/f0d0r/margaret-ebook-library/pkg/converter"
	"github.com/f0d0r/margaret-ebook-library/pkg/mediatype"
)

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

// NewResourceSetFromParts is an internal helper used by internal/resource
// to construct a ResourceSet from pre-deduped parts without re-running the
// dedup logic. It is exported for internal use only.
func NewResourceSetFromParts(all []*Resource, byID map[string]*Resource, byHref map[string]*Resource, readingOrder []ReadingOrderItem, cover *Resource) *ResourceSet {
	return &ResourceSet{
		all:          all,
		byID:         byID,
		byHref:       byHref,
		readingOrder: readingOrder,
		cover:        cover,
	}
}

// NewResourceSet builds a ResourceSet from the given slices.
// all is the full manifest in manifest order; readingOrder is spine order (may be subset of all, with Linear flags).
// cover may be nil or point to one of the resources in all. Maps are built from all using Id and ResolvedHref.
// GetByHref only indexes canonical ResolvedHref; query and fragment are ignored. Relative paths are not resolved.
// Duplicate handling:
//   - Duplicate ResolvedHref (cleanHref) in All(): first wins, All length decreases, second Id aliases to first resource.
//   - Duplicate Id with different ResolvedHref: second resource gets generated Id Id__dupN and remains in All().
func NewResourceSet(all []*Resource, readingOrder []ReadingOrderItem, cover *Resource) *ResourceSet {
	// Filter nils first but keep original order for dedup logic.
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
			hrefKey = util.CleanHref(orig.ResolvedHref)
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
					hrefKey = util.CleanHref(r.ResolvedHref)
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
			if deduped, ok := seenHref[util.CleanHref(res.ResolvedHref)]; ok {
				res = deduped
			}
		}
		// Also handle case where readingOrder references a duplicate Id that was renamed: try to find by original Id? But readingOrder was built from resolved href map first-wins, so it already points to first.
		roCopy = append(roCopy, ReadingOrderItem{Resource: res, Linear: it.Linear})
	}

	// Cover alias: if cover href duplicates an existing href, point to existing.
	coverAliased := cover
	if cover != nil && cover.ResolvedHref != "" {
		if deduped, ok := seenHref[util.CleanHref(cover.ResolvedHref)]; ok {
			coverAliased = deduped
		} else {
			// Cover not in All: ensure it is indexed, but also handle duplicate Id for cover.
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
			if _, exists := byHref[util.CleanHref(cover.ResolvedHref)]; !exists {
				byHref[util.CleanHref(cover.ResolvedHref)] = coverAliased
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

// GetByHref returns the resource with the given resolved href (canonicalized via CleanHref).
// The href should be the resolved, ZIP-absolute form (e.g. "OEBPS/images/cover.jpg").
// Only canonical ResolvedHref is indexed; query and fragment are ignored. Relative references are not resolved here; use a future Resolve method for that.
func (rs *ResourceSet) GetByHref(href string) (*Resource, bool) {
	if rs == nil {
		return nil, false
	}
	key := util.CleanHref(href)
	r, ok := rs.byHref[key]
	return r, ok
}

// All returns all resources in manifest order. The returned slice is a copy.
func (rs *ResourceSet) All() []*Resource {
	if rs == nil {
		return nil
	}
	out := make([]*Resource, len(rs.all))
	copy(out, rs.all)
	return out
}

// ReadingOrder returns the spine-derived reading order, including linear="no" items with Linear==false.
// For MOBI, every item has Linear==true. The returned slice is a copy.
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
	// Collect linear resources.
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

// multiReadCloser lazily opens each resource via OpenAsWithRegistry and
// streams them sequentially.
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
