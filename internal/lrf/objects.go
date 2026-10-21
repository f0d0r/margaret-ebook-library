package lrf

import (
	"encoding/binary"
	"fmt"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/util"
)

// Object types stored in the object start tag.
const (
	objectTypePageTree    = 0x01
	objectTypePage        = 0x02
	objectTypeBlock       = 0x06
	objectTypeText        = 0x0A
	objectTypeImage       = 0x0C
	objectTypeCanvas      = 0x0D
	objectTypeImageStream = 0x11
	objectTypeFont        = 0x19
	objectTypeBookAttr    = 0x1C
	objectTypeSimpleText  = 0x1D
)

// objectStartSize is the size of the object start tag: F500 plus its 6-byte
// payload (u32 id, u16 type).
const objectStartSize = 8

// objectMetaPrefixSize bounds the header bytes inspected to describe an
// image or font without reading its payload.
const objectMetaPrefixSize = 4096

// objectStreamInfo describes the stream declared by an object body.
type objectStreamInfo struct {
	flags    int
	size     int
	fontFile string
	fontFace string
}

// objectIndex answers object lookups against the object table of a file.
type objectIndex struct {
	blob book.Blob
	byID map[uint32]LrfObjectRef
	max  int64
}

func newObjectIndex(b book.Blob, refs []LrfObjectRef, maxResourceSize int64) *objectIndex {
	byID := make(map[uint32]LrfObjectRef, len(refs))
	for _, ref := range refs {
		byID[ref.id] = ref
	}
	return &objectIndex{blob: b, byID: byID, max: maxResourceSize}
}

// objectType reads and validates the object start tag and returns the
// declared object type.
func (ix *objectIndex) objectType(ref LrfObjectRef) (uint16, error) {
	var buf [objectStartSize]byte
	if err := ix.read(ref.offset, buf[:]); err != nil {
		return 0, fmt.Errorf("object %d: %w", ref.id, err)
	}
	return binary.LittleEndian.Uint16(buf[6:8]), nil
}

// objectBody returns the object type and the tag stream that follows the
// start tag.
func (ix *objectIndex) objectBody(ref LrfObjectRef) (uint16, []byte, error) {
	if ref.size < objectStartSize {
		return 0, nil, fmt.Errorf("%w: object %d smaller than its start tag", book.ErrCorrupt, ref.id)
	}
	if ix.max > 0 && int64(ref.size) > ix.max {
		return 0, nil, book.LimitError(fmt.Sprintf("object %d", ref.id), int64(ref.size), ix.max)
	}
	data := make([]byte, ref.size)
	if err := ix.read(ref.offset, data); err != nil {
		return 0, nil, fmt.Errorf("object %d: %w", ref.id, err)
	}
	return binary.LittleEndian.Uint16(data[6:8]), data[objectStartSize:], nil
}

// objectStreamMeta describes the stream of ref from a bounded prefix of its
// body. It falls back to reading the whole object when the prefix does not
// reach the stream tags.
func (ix *objectIndex) objectStreamMeta(ref LrfObjectRef, wantFonts bool) (objectStreamInfo, bool) {
	if info, ok := ix.prefixStreamMeta(ref, wantFonts); ok {
		return info, true
	}
	_, body, err := ix.objectBody(ref)
	if err != nil {
		return objectStreamInfo{}, false
	}
	flags, payload, ok, err := objectStream(body)
	if err != nil || !ok {
		return objectStreamInfo{}, false
	}
	info := objectStreamInfo{flags: flags, size: len(payload)}
	if wantFonts {
		file, face, err := fontNames(body)
		if err != nil {
			return objectStreamInfo{}, false
		}
		info.fontFile, info.fontFace = file, face
	}
	return info, true
}

// prefixStreamMeta scans the leading tags of an object without consuming the
// stream payload. ok is false when the prefix is inconclusive.
func (ix *objectIndex) prefixStreamMeta(ref LrfObjectRef, wantFonts bool) (objectStreamInfo, bool) {
	if ref.size < objectStartSize {
		return objectStreamInfo{}, false
	}
	if ix.max > 0 && int64(ref.size) > ix.max {
		return objectStreamInfo{}, false
	}
	n := int64(ref.size)
	if n > objectMetaPrefixSize {
		n = objectMetaPrefixSize
	}
	data := make([]byte, n)
	if err := ix.read(ref.offset, data); err != nil {
		return objectStreamInfo{}, false
	}
	body := data[objectStartSize:]
	bodySize := int64(ref.size) - objectStartSize

	var info objectStreamInfo
	haveFlags, haveSize := false, false
	for pos := 0; pos < len(body); {
		if pos+2 > len(body) || body[pos+1] != tagMagic {
			return objectStreamInfo{}, false
		}
		if body[pos] == lowTagStreamStart {
			if !haveFlags || !haveSize || int64(pos)+2+int64(info.size) > bodySize {
				return objectStreamInfo{}, false
			}
			return info, true
		}
		size, ok := tagSize(body, pos)
		if !ok {
			return objectStreamInfo{}, false
		}
		payload := body[pos+2 : pos+size]
		switch 0xF500 + uint32(body[pos]) {
		case tagStreamSize:
			info.size = int(binary.LittleEndian.Uint32(payload))
			haveSize = true
		case tagStreamFlags:
			info.flags = int(binary.LittleEndian.Uint16(payload))
			haveFlags = true
		case tagFontFileName:
			if wantFonts {
				if s, ok := stringTagText(payload); ok {
					info.fontFile = s
				}
			}
		case tagFontFaceName:
			if wantFonts {
				if s, ok := stringTagText(payload); ok {
					info.fontFace = s
				}
			}
		}
		pos += size
	}
	return objectStreamInfo{}, false
}

// bodyByID resolves an object id and returns its type and body.
func (ix *objectIndex) bodyByID(id uint32) (uint16, []byte, error) {
	ref, ok := ix.byID[id]
	if !ok {
		return 0, nil, fmt.Errorf("%w: missing object %d", book.ErrCorrupt, id)
	}
	return ix.objectBody(ref)
}

// typeByID resolves an object id and returns its declared type without
// reading the whole body.
func (ix *objectIndex) typeByID(id uint32) (uint16, error) {
	ref, ok := ix.byID[id]
	if !ok {
		return 0, fmt.Errorf("%w: missing object %d", book.ErrCorrupt, id)
	}
	return ix.objectType(ref)
}

func (ix *objectIndex) read(offset uint32, buf []byte) error {
	if err := util.ReadAtFull(ix.blob, buf, int64(offset)); err != nil {
		return fmt.Errorf("%w: read object: %w", book.ErrCorrupt, err)
	}
	if buf[0] != lowTagObjectStart || buf[1] != tagMagic {
		return fmt.Errorf("%w: bad object start tag at offset %d", book.ErrCorrupt, offset)
	}
	return nil
}
