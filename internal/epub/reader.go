package epub

import (
	"archive/zip"
	"encoding/binary"
	"fmt"
	"io"
	"path"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
	"github.com/f0d0r/margaret-ebook-library/internal/util"
	ziputil "github.com/f0d0r/margaret-ebook-library/internal/zip"
)

type EpubReader struct {
	cfg       config.Config
	ocfReader OcfReader
	opfReader OpfReader
}

// NewEpubReader creates a new EPUB reader instance with the given
// safety limits.
func NewEpubReader(cfg config.Config) *EpubReader {
	return &EpubReader{
		cfg:       cfg,
		ocfReader: NewOcfReader(),
		opfReader: NewOpfReader(),
	}
}

func (r *EpubReader) Supports(b book.Blob) bool {
	size, err := b.Size()
	if err != nil || size < 22 {
		return false
	}

	if r.hasValidEpubHeader(b) {
		return true
	}

	zr, err := ziputil.Open(b)
	if err != nil {
		return false
	}

	_, err = r.ocfReader.findContainerFile(zr)
	return err == nil
}

func (r *EpubReader) hasValidEpubHeader(b book.Blob) bool {
	// A valid EPUB starts with a ZIP local file header, followed by the uncompressed "mimetype" file and its 20-byte value.
	buf := make([]byte, 100)
	n, err := b.ReadAt(buf, 0)
	if err != nil || n < 58 {
		return false
	}

	// 1. Verify ZIP signature. Valid ZIP files can start with one of several PK headers.
	if buf[0] != 0x50 || buf[1] != 0x4B {
		return false
	}

	switch {
	case buf[2] == 0x03 && buf[3] == 0x04:
		// ZIP local file header
	case buf[2] == 0x05 && buf[3] == 0x06:
		// ZIP end of central directory (empty archive style)
	case buf[2] == 0x07 && buf[3] == 0x08:
		// ZIP data descriptor / other valid ZIP signature
	default:
		return false
	}

	// 2. Read the file name length and extra field length (Little Endian)
	fileNameLen := binary.LittleEndian.Uint16(buf[26:28])
	extraLen := binary.LittleEndian.Uint16(buf[28:30])

	// The "mimetype" file name is exactly 8 characters long
	if fileNameLen != 8 {
		return false
	}

	// 3. Verify the file name is "mimetype"
	if string(buf[30:38]) != "mimetype" {
		return false
	}

	// 4. Compute and verify the mimetype content location.
	// According to the spec extraLen should be 0, but we tolerate malformed files.
	mimetypeStart := 30 + int(fileNameLen) + int(extraLen)
	mimetypeEnd := mimetypeStart + 20

	if len(buf) < mimetypeEnd {
		return false
	}

	return string(buf[mimetypeStart:mimetypeEnd]) == "application/epub+zip"
}

func (r *EpubReader) Read(b book.Blob) (book.Book, error) {
	zr, err := ziputil.Open(b)
	if err != nil {
		return nil, fmt.Errorf("failed to open epub: %w", err)
	}

	c, err := r.ocfReader.Read(zr)
	if err != nil {
		return nil, fmt.Errorf("failed to read ocf file: %w", err)
	}

	p, err := r.opfReader.Read(zr, c)
	if err != nil {
		return nil, fmt.Errorf("failed to read opf file: %w", err)
	}
	resources, readingOrder, cover := r.readResources(zr, p)

	return &epubBook{
		metadata: book.Metadata{
			Title:       r.opfReader.Title(p),
			Authors:     r.opfReader.Authors(p),
			Description: r.opfReader.Description(p),
			Languages:   r.opfReader.Languages(p),
		},
		resources:  book.NewResourceSet(resources, readingOrder, cover),
		version:    p.Version,
		packageDoc: p,
	}, nil
}

// readResources returns all manifest resources, spine reading order with Linear flag, and cover.
func (r *EpubReader) readResources(zr *zip.Reader, p Package) ([]*book.Resource, []book.ReadingOrderItem, *book.Resource) {
	// All: manifest order, only entries present in ZIP.
	resolvedToResource := make(map[string]*book.Resource, len(p.Manifest.Items))
	all := make([]*book.Resource, 0, len(p.Manifest.Items))
	for _, item := range p.Manifest.Items {
		resolved := p.ResolvePath(item.Href)
		file := ziputil.Find(zr, resolved)
		if file == nil {
			continue
		}
		res := r.resource(item, file)
		res.ResolvedHref = util.CleanHref(res.ResolvedHref)
		ptr := new(book.Resource)
		*ptr = res
		all = append(all, ptr)
		resolvedToResource[util.CleanHref(resolved)] = ptr
	}

	// ReadingOrder: spine order, including linear="no" with flag, reusing pointers from all when possible.
	readingOrder := make([]book.ReadingOrderItem, 0, len(p.Spine.ItemRefs))
	for _, itemRef := range p.Spine.ItemRefs {
		item := r.opfReader.ItemById(p, itemRef.IdRef)
		if item == nil {
			continue
		}
		resolved := p.ResolvePath(item.Href)
		file := ziputil.Find(zr, resolved)
		if file == nil {
			continue
		}
		key := util.CleanHref(resolved)
		var resPtr *book.Resource
		if existing, ok := resolvedToResource[key]; ok {
			resPtr = existing
		} else {
			res := r.resource(*item, file)
			res.ResolvedHref = util.CleanHref(res.ResolvedHref)
			ptr := new(book.Resource)
			*ptr = res
			resPtr = ptr
		}
		linear := itemRef.Linear != "no"
		readingOrder = append(readingOrder, book.ReadingOrderItem{Resource: resPtr, Linear: linear})
	}

	// Cover is derived from all: determine cover item first, then look it up in the already built map.
	// No second Resource creation when the cover is already in all (common case).
	var cover *book.Resource
	if item := r.coverItem(p); item != nil {
		coverKey := util.CleanHref(p.ResolvePath(item.Href))
		if existing, ok := resolvedToResource[coverKey]; ok {
			cover = existing
		} else {
			if file := ziputil.Find(zr, p.ResolvePath(item.Href)); file != nil {
				res := r.resource(*item, file)
				ptr := new(book.Resource)
				*ptr = res
				all = append(all, ptr)
				resolvedToResource[coverKey] = ptr
				cover = ptr
			}
		}
	}
	return all, readingOrder, cover
}

// resource builds a lazily-opened Resource from a manifest item and its
// matching ZIP entry.
func (r *EpubReader) resource(item Item, file *zip.File) book.Resource {
	resolved := util.CleanHref(file.Name)
	return book.Resource{
		Id:           item.ID,
		Name:         path.Base(file.Name),
		Href:         item.Href,
		ResolvedHref: resolved,
		Properties:   item.Properties,
		MediaType:    item.MediaType,
		Size:         int64(file.UncompressedSize64),
		Open: func() (io.ReadCloser, error) {
			maxSize := r.cfg.MaxResourceSize
			if maxSize <= 0 {
				maxSize = config.DefaultConfig().MaxResourceSize
			}
			return ziputil.OpenLimited(file, maxSize)
		},
	}
}

// coverItem returns the manifest item that represents the cover image, if any.
// Priority: first item with properties="cover-image", then <meta name="cover"> reference.
func (r *EpubReader) coverItem(p Package) *Item {
	coverItems := r.opfReader.ItemsByProperty(p, "cover-image")
	if len(coverItems) > 0 {
		return &coverItems[0]
	}
	if meta := r.opfReader.MetaByName(p, "cover"); meta != nil {
		return r.opfReader.ItemById(p, meta.Content)
	}
	return nil
}
