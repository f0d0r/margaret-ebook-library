package lrf

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

const (
	contentID      = "content"
	contentName    = "content.txt"
	contentCharset = "utf-16le"
)

// UTF-16LE encoded tokens inserted for the layout tokens of the text
// stream. A plot (inline image) becomes U+FFFC OBJECT REPLACEMENT CHARACTER:
// its payload carries only layout attributes, and the marker keeps the
// position without leaking binary data into the text flow.
var (
	crUTF16LE   = []byte{0x0A, 0x00}
	paraUTF16LE = []byte{0x0A, 0x00, 0x0A, 0x00}
	plotUTF16LE = []byte{0xFC, 0xFF}
)

// readContent returns the synthesized native text resource, or nil when the
// file declares no root object. The resource reads lazily: the page tree and
// all text streams are walked on Open, so damage deep inside the content
// graph does not prevent opening the book.
func (r *LrfReader) readContent(header *LrfHeader, ix *objectIndex) *book.Resource {
	if header.RootObjectID == 0 {
		return nil
	}
	return &book.Resource{
		Id:           contentID,
		Name:         contentName,
		Href:         contentName,
		ResolvedHref: contentName,
		MediaType:    mediatype.LRFText,
		Charset:      contentCharset,
		Open: func() (io.ReadCloser, error) {
			data, err := r.synthesizeContent(header, ix)
			if err != nil {
				return nil, err
			}
			return io.NopCloser(bytes.NewReader(data)), nil
		},
	}
}

// synthesizeContent walks the content graph from the book root (BookAttr,
// PageTree, Page, Block, Text) and concatenates the decoded text streams in
// reading order.
func (r *LrfReader) synthesizeContent(header *LrfHeader, ix *objectIndex) ([]byte, error) {
	typ, body, err := ix.bodyByID(header.RootObjectID)
	if err != nil {
		return nil, err
	}
	if typ != objectTypeBookAttr {
		return nil, fmt.Errorf("%w: root object %d is not a BookAttr", book.ErrCorrupt, header.RootObjectID)
	}
	pageTreeID, ok, err := findDwordTag(body, tagBookPageTree)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%w: BookAttr has no page tree reference", book.ErrCorrupt)
	}
	pageIDs, err := pageTreeIDs(ix, pageTreeID)
	if err != nil {
		return nil, err
	}

	var out []byte
	for _, pageID := range pageIDs {
		blockIDs, err := r.pageBlocks(header, ix, pageID)
		if err != nil {
			return nil, err
		}
		for _, blockID := range blockIDs {
			text, err := r.blockText(header, ix, blockID)
			if err != nil {
				return nil, err
			}
			if text == nil {
				continue
			}
			out, err = appendText(out, text)
			if err != nil {
				return nil, err
			}
			if max := r.cfg.MaxResourceSize; max > 0 && int64(len(out)) > max {
				return nil, book.LimitError(contentName, int64(len(out)), max)
			}
		}
	}
	return out, nil
}

// pageTreeIDs returns the ordered page object ids of a PageTree.
func pageTreeIDs(ix *objectIndex, id uint32) ([]uint32, error) {
	typ, body, err := ix.bodyByID(id)
	if err != nil {
		return nil, err
	}
	if typ != objectTypePageTree {
		return nil, fmt.Errorf("%w: object %d is not a PageTree", book.ErrCorrupt, id)
	}
	sc := newTagScanner(body)
	for {
		tagID, payload, ok, err := sc.next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		if tagID == tagPageTreePages {
			ids, ok := parseTypeOnePayload(payload)
			if !ok {
				return nil, fmt.Errorf("%w: PageTree %d has a malformed page list", book.ErrCorrupt, id)
			}
			return ids, nil
		}
	}
	return nil, fmt.Errorf("%w: PageTree %d has no page list", book.ErrCorrupt, id)
}

// pageBlocks returns the ordered content object ids linked by a page stream.
// A Canvas (used e.g. for image pages and multi-column layouts) is expanded
// into the Blocks its PutObj tags reference, in stream order.
func (r *LrfReader) pageBlocks(header *LrfHeader, ix *objectIndex, pageID uint32) ([]uint32, error) {
	typ, _, err := ix.bodyByID(pageID)
	if err != nil {
		return nil, err
	}
	if typ != objectTypePage {
		return nil, fmt.Errorf("%w: object %d is not a Page", book.ErrCorrupt, pageID)
	}
	stream, err := r.objectStreamByID(header, ix, pageID, false)
	if err != nil {
		return nil, err
	}
	links, err := findDwordTags(stream, tagLink)
	if err != nil {
		return nil, err
	}
	var out []uint32
	for _, id := range links {
		typ, err := ix.typeByID(id)
		if err != nil {
			return nil, err
		}
		if typ != objectTypeCanvas {
			out = append(out, id)
			continue
		}
		canvas, err := r.objectStreamByID(header, ix, id, false)
		if err != nil {
			return nil, err
		}
		refs, err := findPutObjectRefs(canvas, tagPutObj)
		if err != nil {
			return nil, err
		}
		out = append(out, refs...)
	}
	return out, nil
}

// blockText returns the decoded text stream of a block, or nil for page
// objects that carry no text (image and button blocks, or non-Block objects
// that pageBlocks did not expand).
func (r *LrfReader) blockText(header *LrfHeader, ix *objectIndex, blockID uint32) ([]byte, error) {
	typ, err := ix.typeByID(blockID)
	if err != nil {
		return nil, err
	}
	if typ != objectTypeBlock {
		return nil, nil
	}
	stream, err := r.objectStreamByID(header, ix, blockID, false)
	if err != nil {
		return nil, err
	}
	content, ok, err := findDwordTag(stream, tagLink)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%w: block %d has no content link", book.ErrCorrupt, blockID)
	}
	contentType, _, err := ix.bodyByID(content)
	if err != nil {
		return nil, err
	}
	if contentType != objectTypeText && contentType != objectTypeSimpleText {
		return nil, nil
	}
	return r.objectStreamByID(header, ix, content, false)
}

// appendText tokenizes a decoded Text stream and appends its raw UTF-16LE
// text bytes to out. Layout tags are dropped, paragraph and line breaks
// become separators, and F5D1 plots become U+FFFC markers.
func appendText(out, stream []byte) ([]byte, error) {
	i := 0
	for i < len(stream) {
		j := i
		found := false
		for j+1 < len(stream) {
			if stream[j+1] == tagMagic && textTagCodes[stream[j]] {
				found = true
				break
			}
			j++
		}
		if !found {
			return append(out, stream[i:]...), nil
		}
		out = append(out, stream[i:j]...)
		switch stream[j] {
		case lowTagString:
			if j+4 > len(stream) {
				return nil, fmt.Errorf("%w: truncated text string", book.ErrCorrupt)
			}
			n := int(binary.LittleEndian.Uint16(stream[j+2 : j+4]))
			if j+4+n > len(stream) {
				return nil, fmt.Errorf("%w: truncated text string", book.ErrCorrupt)
			}
			out = append(out, stream[j+4:j+4+n]...)
			i = j + 4 + n
		case lowTagStartPara:
			out = append(out, paraUTF16LE...)
			i = j + 6
		case lowTagCR:
			out = append(out, crUTF16LE...)
			i = j + 2
		case lowTagPlot:
			if j+14 > len(stream) {
				return nil, fmt.Errorf("%w: truncated plot payload", book.ErrCorrupt)
			}
			out = append(out, plotUTF16LE...)
			i = j + 14
		default:
			size, ok := tagSize(stream, j)
			if !ok {
				return nil, fmt.Errorf("%w: invalid tag %#04x in text stream",
					book.ErrCorrupt, 0xF500+uint32(stream[j]))
			}
			i = j + size
		}
	}
	return out, nil
}
