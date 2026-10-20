package lrf

import (
	"encoding/binary"
	"fmt"
	"unicode/utf16"

	"github.com/f0d0r/margaret-ebook-library/book"
)

// tagMagic is the high byte of every LRF tag: IDs are stored low byte first,
// so the tag holding the low byte b is encoded as the byte pair (b, 0xF5).
const tagMagic = 0xF5

// Tag IDs used by the reader.
const (
	tagObjectEnd     = 0xF501
	tagLink          = 0xF503
	tagStreamSize    = 0xF504
	tagStreamStart   = 0xF505
	tagStreamEnd     = 0xF506
	tagPutObj        = 0xF549
	tagStreamFlags   = 0xF554
	tagFontFileName  = 0xF559
	tagPageTreePages = 0xF55C
	tagFontFaceName  = 0xF55D
	tagBookPageTree  = 0xF57B
	tagTextStartPara = 0xF5A1
	tagTextString    = 0xF5CC
	tagTextPlot      = 0xF5D1
	tagTextCR        = 0xF5D2
)

const (
	lowTagObjectStart = 0x00
	lowTagStreamStart = 0x05
	lowTagFontFace    = 0x16
	lowTagTag78       = 0x78
	lowTagStartPara   = 0xA1
	lowTagString      = 0xCC
	lowTagPlot        = 0xD1
	lowTagCR          = 0xD2
)

// stringTags are tag low bytes whose payload is a u16 byte count followed by
// UTF-16LE text.
var stringTags = [256]bool{0x16: true, 0x55: true, 0x59: true, 0x5A: true, 0x5D: true}

// typeOneTags are tag low bytes whose payload is a u16 count followed by that
// many u32 values.
var typeOneTags = [256]bool{0x0B: true, 0x5C: true}

// fixedTagPayload maps tag low bytes to their fixed payload size.
// Variable-size tags are handled by tagSize; low bytes missing
// from this table are not valid tags (except the ones handled specially).
var fixedTagPayload = map[byte]int{
	0x00: 6, 0x01: 0, 0x02: 4, 0x03: 4, 0x04: 4, 0x05: 0, 0x06: 0,
	0x07: 4, 0x08: 4, 0x09: 4, 0x0A: 4,
	0x0D: 2, 0x0E: 2,
	0x11: 2, 0x12: 2, 0x13: 2, 0x14: 2, 0x15: 2,
	0x17: 4, 0x18: 4,
	0x19: 2, 0x1A: 2, 0x1B: 2, 0x1C: 2, 0x1D: 2, 0x1E: 2,
	0x21: 2, 0x22: 2, 0x23: 2, 0x24: 2, 0x25: 2, 0x26: 2, 0x27: 2, 0x28: 2,
	0x29: 6, 0x2A: 2, 0x2B: 2, 0x2C: 2, 0x2D: 4, 0x2E: 2,
	0x31: 2, 0x32: 2, 0x33: 2, 0x34: 4, 0x35: 2, 0x36: 2, 0x37: 4, 0x38: 2, 0x39: 2, 0x3A: 2,
	0x3C: 2, 0x3D: 2, 0x3E: 2,
	0x41: 2, 0x42: 2, 0x44: 4, 0x45: 4, 0x46: 2, 0x47: 2, 0x48: 2,
	0x49: 8, 0x4A: 8, 0x4B: 4, 0x4C: 4, 0x4D: 0, 0x4E: 12,
	0x51: 2, 0x52: 2, 0x53: 4, 0x54: 2,
	0x56: 2, 0x57: 2, 0x58: 2,
	0x5B: 4,
	0x5E: 2,
	0x61: 2, 0x62: 0, 0x63: 0, 0x64: 0, 0x65: 0, 0x66: 0, 0x67: 0, 0x68: 0, 0x69: 0, 0x6A: 0, 0x6B: 0,
	0x6C: 8, 0x6D: 2, 0x6E: 0,
	0x71: 0, 0x72: 0, 0x73: 10,
	0x75: 2, 0x76: 2, 0x77: 2,
	0x79: 2, 0x7A: 2, 0x7B: 4, 0x7C: 4,
	0x81: 0, 0x82: 0,
	0xA1: 4, 0xA2: 0,
	0xA6: 0, 0xA7: 4, 0xA8: 0, 0xA9: 0, 0xAA: 0, 0xAB: 0, 0xAC: 0, 0xAD: 0, 0xAE: 0,
	0xB1: 0, 0xB2: 0, 0xB3: 0, 0xB4: 0, 0xB5: 0, 0xB6: 0, 0xB7: 0, 0xB8: 0, 0xB9: 0,
	0xBA: 0, 0xBB: 0, 0xBC: 0, 0xBD: 0, 0xBE: 0,
	0xC1: 0, 0xC2: 0, 0xC3: 2, 0xC4: 0, 0xC5: 2, 0xC6: 2, 0xC7: 0, 0xC8: 2, 0xC9: 0, 0xCA: 2,
	0xCC: 2,
	0xD1: 12, 0xD2: 0, 0xD4: 2, 0xD6: 0, 0xD7: 14, 0xD8: 4, 0xD9: 8, 0xDA: 2, 0xDB: 2, 0xDC: 2, 0xDD: 2,
	0xF1: 2, 0xF2: 4, 0xF3: 4, 0xF4: 2, 0xF5: 4, 0xF6: 4, 0xF7: 4, 0xF8: 4, 0xF9: 6,
}

// textTagCodes lists the low bytes of tags that may appear inside a Text
// stream. The tokenizer uses it to tell real tags from raw UTF-16LE text
// bytes.
var textTagCodes = [256]bool{
	0x11: true, 0x12: true, 0x13: true, 0x14: true, 0x15: true, 0x16: true, 0x17: true, 0x18: true, 0x19: true,
	0x1A: true, 0x1B: true, 0x1C: true, 0x1D: true, 0x1E: true,
	0x3C: true, 0x3D: true, 0x3E: true,
	0x75: true, 0x76: true, 0x77: true, 0x78: true, 0x79: true, 0x7A: true,
	0x81: true, 0x82: true,
	0xA1: true, 0xA2: true, 0xA7: true, 0xA8: true, 0xA9: true, 0xAA: true, 0xAB: true, 0xAC: true,
	0xAD: true, 0xAE: true,
	0xB1: true, 0xB2: true, 0xB3: true, 0xB4: true, 0xB5: true, 0xB6: true, 0xB7: true, 0xB8: true,
	0xB9: true, 0xBA: true, 0xBB: true, 0xBC: true, 0xBD: true, 0xBE: true,
	0xC1: true, 0xC2: true, 0xC3: true, 0xC4: true, 0xC6: true, 0xC7: true,
	0xCA: true, 0xCC: true, 0xD1: true, 0xD2: true,
	0xDD: true,
	0xF1: true, 0xF2: true,
}

// tagSize returns the total encoded size (ID plus payload) of the tag at
// data[pos]. ok is false when the bytes are not a valid LRF tag, the tag is
// unknown to the format, or the tag is truncated.
func tagSize(data []byte, pos int) (int, bool) {
	if pos < 0 || pos+2 > len(data) || data[pos+1] != tagMagic {
		return 0, false
	}
	low := data[pos]
	switch {
	case stringTags[low]:
		if pos+4 > len(data) {
			return 0, false
		}
		n := int(binary.LittleEndian.Uint16(data[pos+2 : pos+4]))
		if pos+4+n > len(data) {
			return 0, false
		}
		return 4 + n, true
	case typeOneTags[low]:
		if pos+4 > len(data) {
			return 0, false
		}
		n := int(binary.LittleEndian.Uint16(data[pos+2 : pos+4]))
		if pos+4+4*n > len(data) {
			return 0, false
		}
		return 4 + 4*n, true
	case low == lowTagTag78:
		if pos+10 > len(data) || data[pos+6] != lowTagFontFace || data[pos+7] != tagMagic {
			return 0, false
		}
		n := int(binary.LittleEndian.Uint16(data[pos+8 : pos+10]))
		if pos+12+n > len(data) {
			return 0, false
		}
		return 12 + n, true
	}
	n, ok := fixedTagPayload[low]
	if !ok || pos+2+n > len(data) {
		return 0, false
	}
	return 2 + n, true
}

// stringTagText decodes the UTF-16LE text carried by a string tag payload
// (the u16 byte count plus the encoded text).
func stringTagText(payload []byte) (string, bool) {
	if len(payload) < 2 {
		return "", false
	}
	n := int(binary.LittleEndian.Uint16(payload[:2]))
	if n%2 != 0 || 2+n > len(payload) {
		return "", false
	}
	return decodeUTF16LE(payload[2 : 2+n]), true
}

// decodeUTF16LE decodes little-endian UTF-16 bytes, mapping unpaired
// surrogates to the replacement character.
func decodeUTF16LE(b []byte) string {
	units := make([]uint16, len(b)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(b[2*i:])
	}
	return string(utf16.Decode(units))
}

// tagScanner walks the tags of an object body or decoded stream. Stream tags
// (F505) are resolved against the size and flags declared by the preceding
// F504/F554 tags and returned as payloads without further decoding.
type tagScanner struct {
	data      []byte
	pos       int
	haveSize  bool
	size      int
	haveFlags bool
	flags     int
	streamed  bool
}

func newTagScanner(data []byte) *tagScanner {
	return &tagScanner{data: data}
}

// next returns the next tag: its ID, its payload (for F505 the raw stream
// bytes), and ok=false once the data is exhausted. The payload aliases data
// and must not be retained after the scanner's buffer is released.
func (s *tagScanner) next() (uint32, []byte, bool, error) {
	if s.pos >= len(s.data) {
		return 0, nil, false, nil
	}
	if s.pos+2 > len(s.data) || s.data[s.pos+1] != tagMagic {
		return 0, nil, false, fmt.Errorf("%w: bad tag at offset %d", book.ErrCorrupt, s.pos)
	}
	if s.data[s.pos] == lowTagStreamStart {
		if s.streamed {
			return 0, nil, false, fmt.Errorf("%w: multiple streams in object", book.ErrCorrupt)
		}
		if !s.haveFlags {
			return 0, nil, false, fmt.Errorf("%w: stream flags missing before stream", book.ErrCorrupt)
		}
		size := 0
		if s.haveSize {
			size = s.size
		}
		if s.pos+2+size > len(s.data) {
			return 0, nil, false, fmt.Errorf("%w: truncated stream at %d", book.ErrCorrupt, s.pos)
		}
		payload := s.data[s.pos+2 : s.pos+2+size]
		s.pos += 2 + size
		s.streamed = true
		return tagStreamStart, payload, true, nil
	}
	size, ok := tagSize(s.data, s.pos)
	if !ok {
		return 0, nil, false, fmt.Errorf("%w: invalid tag %#04x at %d",
			book.ErrCorrupt, 0xF500+uint32(s.data[s.pos]), s.pos)
	}
	id := 0xF500 + uint32(s.data[s.pos])
	payload := s.data[s.pos+2 : s.pos+size]
	s.pos += size
	switch id {
	case tagStreamSize:
		s.size = int(binary.LittleEndian.Uint32(payload))
		s.haveSize = true
	case tagStreamFlags:
		s.flags = int(binary.LittleEndian.Uint16(payload))
		s.haveFlags = true
	}
	return id, payload, true, nil
}

// findDwordTag returns the u32 payload of the first tag with the given ID.
func findDwordTag(data []byte, tag uint32) (uint32, bool, error) {
	sc := newTagScanner(data)
	for {
		id, payload, ok, err := sc.next()
		if err != nil || !ok {
			return 0, false, err
		}
		if id == tag {
			if len(payload) != 4 {
				return 0, false, fmt.Errorf("%w: tag %#04x has %d bytes, want 4", book.ErrCorrupt, tag, len(payload))
			}
			return binary.LittleEndian.Uint32(payload), true, nil
		}
	}
}

// findDwordTags returns the u32 payloads of all tags with the given ID, in
// order.
func findDwordTags(data []byte, tag uint32) ([]uint32, error) {
	var out []uint32
	sc := newTagScanner(data)
	for {
		id, payload, ok, err := sc.next()
		if err != nil {
			return nil, err
		}
		if !ok {
			return out, nil
		}
		if id == tag {
			if len(payload) != 4 {
				return nil, fmt.Errorf("%w: tag %#04x has %d bytes, want 4", book.ErrCorrupt, tag, len(payload))
			}
			out = append(out, binary.LittleEndian.Uint32(payload))
		}
	}
}

// findPutObjectRefs returns the referenced object ids of all PutObj tags, in
// order. A PutObj payload is <u16 x1><u16 y1><u32 refobj>.
func findPutObjectRefs(data []byte, tag uint32) ([]uint32, error) {
	var out []uint32
	sc := newTagScanner(data)
	for {
		id, payload, ok, err := sc.next()
		if err != nil {
			return nil, err
		}
		if !ok {
			return out, nil
		}
		if id == tag {
			if len(payload) != 8 {
				return nil, fmt.Errorf("%w: tag %#04x has %d bytes, want 8", book.ErrCorrupt, tag, len(payload))
			}
			out = append(out, binary.LittleEndian.Uint32(payload[4:]))
		}
	}
}

// parseTypeOnePayload decodes a type_one tag payload: a u16 count followed by
// count u32 values.
func parseTypeOnePayload(payload []byte) ([]uint32, bool) {
	if len(payload) < 2 {
		return nil, false
	}
	n := int(binary.LittleEndian.Uint16(payload))
	if 2+4*n > len(payload) {
		return nil, false
	}
	out := make([]uint32, n)
	for i := range out {
		out[i] = binary.LittleEndian.Uint32(payload[2+4*i:])
	}
	return out, true
}
