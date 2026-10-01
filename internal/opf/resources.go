package opf

import (
	"io"
	"path"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/util"
)

// EntrySource is one manifest item's stored bytes as seen by
// BuildResources. Format readers resolve it from their own container (a ZIP
// entry, a LIT directory record, ...); identity fields (ID, href, name)
// stay with the shared builder so every format names resources the same way.
type EntrySource struct {
	// Size is the stored byte count of the entry.
	Size int64
	// Open streams the entry content lazily. Size limiting is enforced
	// inside, following the format reader's configured limits.
	Open func() (io.ReadCloser, error)
}

// newResource builds one manifest resource from its item, the canonical path
// it resolved to and the located entry. The three BuildResources call sites
// differ only in which lookup path they took, so the naming stays here.
func newResource(item Item, resolved string, src EntrySource) *book.Resource {
	res := book.Resource{
		Id:           item.ID,
		Name:         path.Base(resolved),
		Href:         item.Href,
		ResolvedHref: resolved,
		Properties:   item.Properties,
		MediaType:    item.MediaType,
		Size:         src.Size,
		Open:         src.Open,
	}
	return &res
}

// BuildResources assembles all manifest resources, the spine reading order
// with Linear flags, and the cover from a package. Manifest items without a
// backing entry (lookup reports false) are skipped, mirroring how missing
// ZIP entries are skipped. Cover lookup prefers the cover-image property,
// then the cover meta reference.
func BuildResources(p Package, lookup func(Item) (EntrySource, bool)) (all []*book.Resource, order []book.ReadingOrderItem, cover *book.Resource) {
	resolvedToResource := make(map[string]*book.Resource, len(p.Manifest.Items))
	all = make([]*book.Resource, 0, len(p.Manifest.Items))
	for _, item := range p.Manifest.Items {
		resolved := util.CleanHref(p.ResolvePath(item.Href))
		src, ok := lookup(item)
		if !ok {
			continue
		}
		ptr := newResource(item, resolved, src)
		all = append(all, ptr)
		resolvedToResource[resolved] = ptr
	}

	readingOrder := make([]book.ReadingOrderItem, 0, len(p.Spine.ItemRefs))
	for _, itemRef := range p.Spine.ItemRefs {
		item := p.ItemById(itemRef.IdRef)
		if item == nil {
			continue
		}
		resolved := util.CleanHref(p.ResolvePath(item.Href))
		var resPtr *book.Resource
		if existing, ok := resolvedToResource[resolved]; ok {
			resPtr = existing
		} else {
			src, ok := lookup(*item)
			if !ok {
				continue
			}
			resPtr = newResource(*item, resolved, src)
		}
		linear := itemRef.Linear != "no"
		readingOrder = append(readingOrder, book.ReadingOrderItem{Resource: resPtr, Linear: linear})
	}

	if item := coverItem(p); item != nil {
		coverKey := util.CleanHref(p.ResolvePath(item.Href))
		if existing, ok := resolvedToResource[coverKey]; ok {
			cover = existing
		} else {
			if src, ok := lookup(*item); ok {
				ptr := newResource(*item, coverKey, src)
				all = append(all, ptr)
				resolvedToResource[coverKey] = ptr
				cover = ptr
			}
		}
	}
	return all, readingOrder, cover
}

// coverItem returns the manifest item that represents the cover image, if
// any. Priority: first item with properties="cover-image", then a <meta
// name="cover"> reference.
func coverItem(p Package) *Item {
	if items := p.ItemsByProperty("cover-image"); len(items) > 0 {
		return &items[0]
	}
	if meta := p.MetaByName("cover"); meta != nil {
		return p.ItemById(meta.Content)
	}
	return nil
}
