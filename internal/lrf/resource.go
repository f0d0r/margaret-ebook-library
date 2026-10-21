package lrf

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/sections"
	"github.com/f0d0r/margaret-ebook-library/internal/util"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

// LrfObjectRef is one 16-byte row of the object table: the object id and its
// location in the file. The fourth dword of a row is reserved and ignored.
type LrfObjectRef struct {
	id     uint32
	offset uint32
	size   uint32
}

// imageTypeCodes maps an image format code to a media type. The same codes
// identify the header thumbnail type and an ImageStream's image format. The
// header value is unreliable (buggy writers), so for the thumbnail it only
// serves as a fallback for the sniff.
var imageTypeCodes = map[uint16]string{
	0x11: mediatype.JPEG,
	0x12: mediatype.PNG,
	0x13: mediatype.BMP,
	0x14: mediatype.GIF,
}

const objectTableEntrySize = 16

// readObjectTable reads the object table from the end of the file, returning
// its entries in file (offset) order.
func (r *LrfReader) readObjectTable(b book.Blob, header *LrfHeader) ([]LrfObjectRef, error) {
	fileSize, err := b.Size()
	if err != nil {
		return nil, fmt.Errorf("%w: read file size: %w", book.ErrCorrupt, err)
	}
	if header.ObjectTableOffset > uint64(fileSize) {
		return nil, fmt.Errorf("%w: object table offset %d beyond file size %d",
			book.ErrCorrupt, header.ObjectTableOffset, fileSize)
	}
	if header.NumberOfObjects > uint64(fileSize)/objectTableEntrySize {
		return nil, fmt.Errorf("%w: object count %d exceeds file size %d",
			book.ErrCorrupt, header.NumberOfObjects, fileSize)
	}
	tableEnd := int64(header.ObjectTableOffset) + int64(header.NumberOfObjects)*objectTableEntrySize
	if tableEnd > fileSize {
		return nil, fmt.Errorf("%w: object table ends at %d beyond file size %d",
			book.ErrCorrupt, tableEnd, fileSize)
	}

	data := make([]byte, int(header.NumberOfObjects)*objectTableEntrySize)
	if err := util.ReadAtFull(b, data, int64(header.ObjectTableOffset)); err != nil {
		return nil, fmt.Errorf("%w: read object table: %w", book.ErrCorrupt, err)
	}

	refs := make([]LrfObjectRef, header.NumberOfObjects)
	for i := range refs {
		entry := data[i*objectTableEntrySize:]
		refs[i] = LrfObjectRef{
			id:     binary.LittleEndian.Uint32(entry),
			offset: binary.LittleEndian.Uint32(entry[4:]),
			size:   binary.LittleEndian.Uint32(entry[8:]),
		}
	}
	return refs, nil
}

func (r *LrfReader) readResources(b book.Blob, header *LrfHeader) (*book.ResourceSet, error) {
	objectTable, err := r.readObjectTable(b, header)
	if err != nil {
		return nil, err
	}
	cover, err := r.readCover(b, header)
	if err != nil {
		return nil, err
	}
	all, readingOrder, err := r.readObjects(b, header, objectTable)
	if err != nil {
		return nil, err
	}
	if cover != nil {
		all = append(all, cover)
	}
	return book.NewResourceSet(all, readingOrder, cover), nil
}

// readCover builds the cover image resource from the header thumbnail, or
// returns nil when the file carries none.
func (r *LrfReader) readCover(b book.Blob, header *LrfHeader) (*book.Resource, error) {
	if header.Version <= 800 || header.ThumbnailSize == 0 {
		return nil, nil
	}
	if header.CompressedInfoSize < 4 {
		return nil, fmt.Errorf("%w: thumbnail without info block", book.ErrCorrupt)
	}

	fileSize, err := b.Size()
	if err != nil {
		return nil, fmt.Errorf("%w: read file size: %w", book.ErrCorrupt, err)
	}
	pos := header.InfoStart + int64(header.CompressedInfoSize) - 4
	size := int64(header.ThumbnailSize)
	if pos < 0 || size > fileSize-pos {
		return nil, fmt.Errorf("%w: thumbnail at %d size %d beyond file size %d",
			book.ErrCorrupt, pos, size, fileSize)
	}

	media, err := thumbnailMedia(b, header, pos, size)
	if err != nil {
		return nil, err
	}
	if media == nil {
		return nil, nil
	}

	href := "cover." + media.Extension
	return &book.Resource{
		Id:           "cover",
		Name:         href,
		Href:         href,
		ResolvedHref: href,
		MediaType:    media.Type,
		Charset:      book.CharsetBinary,
		Properties:   "cover-image",
		Size:         size,
		Open: func() (io.ReadCloser, error) {
			return sections.Open(b, pos, size, r.cfg.MaxResourceSize, 0, href)
		},
	}, nil
}

// thumbnailMedia identifies the thumbnail image format from its first bytes;
// a recognizable magic wins over the header type. It returns nil when neither
// matches.
func thumbnailMedia(b book.Blob, header *LrfHeader, pos, size int64) (*util.Media, error) {
	var buf [16]byte
	n := min(size, int64(len(buf)))
	if err := util.ReadAtFull(b, buf[:n], pos); err != nil {
		return nil, fmt.Errorf("%w: read thumbnail: %w", book.ErrCorrupt, err)
	}
	if media := util.DetectImageMedia(buf[:n]); media != nil {
		return media, nil
	}
	mime, ok := imageTypeCodes[header.ThumbnailType]
	if !ok {
		return nil, nil
	}
	ext, ok := mediatype.Extension(mime)
	if !ok {
		return nil, nil
	}
	return &util.Media{Type: mime, Extension: ext}, nil
}

// readObjects builds the resources of the object table: the synthesized
// content resource first, then the embedded images and fonts. Sound objects
// are skipped: their payload format is undocumented. Binary objects that
// cannot be interpreted are skipped individually rather than failing the
// whole book.
//
// LRF is a single-document format: the reading order holds the content
// resource as its only linear item. Embedded images and fonts are
// manifest-only (they are referenced from the content flow, not spine
// documents), matching the single-document PalmDOC and FB3 readers.
func (r *LrfReader) readObjects(b book.Blob, header *LrfHeader, objectTable []LrfObjectRef) ([]*book.Resource, []book.ReadingOrderItem, error) {
	ix := newObjectIndex(b, objectTable, r.cfg.MaxResourceSize)

	var all []*book.Resource
	var readingOrder []book.ReadingOrderItem
	if res := r.readContent(header, ix); res != nil {
		all = append(all, res)
		readingOrder = append(readingOrder, book.ReadingOrderItem{Resource: res, Linear: true})
	}
	for _, ref := range objectTable {
		typ, err := ix.objectType(ref)
		if err != nil {
			return nil, nil, err
		}
		var res *book.Resource
		switch typ {
		case objectTypeImageStream:
			res = r.imageResource(header, ix, ref)
		case objectTypeFont:
			res = r.fontResource(header, ix, ref)
		}
		if res != nil {
			all = append(all, res)
		}
	}
	return all, readingOrder, nil
}

// imageResource builds the resource for an ImageStream object, or nil when
// the image format is unknown or the object is malformed.
func (r *LrfReader) imageResource(header *LrfHeader, ix *objectIndex, ref LrfObjectRef) *book.Resource {
	info, ok := ix.objectStreamMeta(ref, false)
	if !ok {
		return nil
	}
	mime, ok := imageTypeCodes[uint16(info.flags)&0xFF]
	if !ok {
		return nil
	}
	ext, ok := mediatype.Extension(mime)
	if !ok {
		return nil
	}
	href := fmt.Sprintf("images/%d.%s", ref.id, ext)
	size := int64(0)
	if info.flags&streamFlagCompressed == 0 {
		size = int64(info.size)
	}
	return &book.Resource{
		Id:           fmt.Sprintf("image-%d", ref.id),
		Name:         path.Base(href),
		Href:         href,
		ResolvedHref: href,
		MediaType:    mime,
		Charset:      book.CharsetBinary,
		Size:         size,
		Open: func() (io.ReadCloser, error) {
			data, err := r.objectStreamByID(header, ix, ref.id, true)
			if err != nil {
				return nil, err
			}
			return io.NopCloser(bytes.NewReader(data)), nil
		},
	}
}

// fontResource builds the resource for a Font object, named after the face
// name. It returns nil for malformed objects.
func (r *LrfReader) fontResource(header *LrfHeader, ix *objectIndex, ref LrfObjectRef) *book.Resource {
	info, ok := ix.objectStreamMeta(ref, true)
	if !ok {
		return nil
	}
	name := fontFileName(info.fontFile, info.fontFace, ref.id)
	href := "fonts/" + name
	size := int64(0)
	if info.flags&streamFlagCompressed == 0 {
		size = int64(info.size)
	}
	return &book.Resource{
		Id:           fmt.Sprintf("font-%d", ref.id),
		Name:         name,
		Href:         href,
		ResolvedHref: href,
		MediaType:    mediatype.TTF,
		Charset:      book.CharsetBinary,
		Size:         size,
		Open: func() (io.ReadCloser, error) {
			data, err := r.objectStreamByID(header, ix, ref.id, true)
			if err != nil {
				return nil, err
			}
			return io.NopCloser(bytes.NewReader(data)), nil
		},
	}
}

// fontNames extracts the file and face names of a Font object.
func fontNames(body []byte) (file, face string, err error) {
	sc := newTagScanner(body)
	for {
		id, payload, ok, err := sc.next()
		if err != nil {
			return "", "", err
		}
		if !ok {
			return file, face, nil
		}
		switch id {
		case tagFontFileName:
			if s, ok := stringTagText(payload); ok {
				file = s
			}
		case tagFontFaceName:
			if s, ok := stringTagText(payload); ok {
				face = s
			}
		}
	}
}

// fontFileName derives the embedded file name from the face name, falling
// back to the file name and finally to the object id.
func fontFileName(file, face string, id uint32) string {
	base := strings.TrimSuffix(path.Base(file), path.Ext(file))
	for _, cand := range []string{face, base} {
		if name := sanitizeName(cand); name != "" {
			return name + ".ttf"
		}
	}
	return fmt.Sprintf("font-%d.ttf", id)
}

// sanitizeName maps a font name to a safe file name stem.
func sanitizeName(s string) string {
	var b strings.Builder
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_', c == '.':
			b.WriteRune(c)
		}
	}
	return strings.Trim(b.String(), ".")
}
