package chm

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/f0d0r/margaret-ebook-library/book"
	"golang.org/x/text/encoding/htmlindex"
)

// #SYSTEM entry codes carrying book metadata. See chmspec 5.3.3, table 5.2.
const (
	sysTitle        = 3
	sysDefaultTopic = 2
	sysInfo         = 4
	sysCompiledFile = 6
)

// system holds the parsed #SYSTEM entries by code.
type system struct {
	entries map[uint16][][]byte
}

// readSystem parses the #SYSTEM control entry: a DWORD version followed by
// code/length/data entries to EOF. See chmspec 5.3.3.
func (c *container) readSystem() (*system, error) {
	raw, err := c.getFile("/#SYSTEM")
	if err != nil {
		return nil, err
	}
	if len(raw) < 4 {
		return nil, fmt.Errorf("chm: truncated #SYSTEM: %w", book.ErrCorrupt)
	}
	if v := binary.LittleEndian.Uint32(raw[:4]); v != 2 && v != 3 {
		return nil, fmt.Errorf("chm: unsupported #SYSTEM version %d: %w", v, book.ErrCorrupt)
	}
	s := &system{entries: map[uint16][][]byte{}}
	pos := 4
	for pos < len(raw) {
		if pos+4 > len(raw) {
			return nil, fmt.Errorf("chm: truncated #SYSTEM entry: %w", book.ErrCorrupt)
		}
		code := binary.LittleEndian.Uint16(raw[pos:])
		length := int(binary.LittleEndian.Uint16(raw[pos+2:]))
		pos += 4
		if length < 0 || pos+length > len(raw) {
			return nil, fmt.Errorf("chm: #SYSTEM entry overruns: %w", book.ErrCorrupt)
		}
		s.entries[code] = append(s.entries[code], raw[pos:pos+length])
		pos += length
	}
	return s, nil
}

// first returns the first entry payload for code.
func (s *system) first(code uint16) []byte {
	if len(s.entries[code]) == 0 {
		return nil
	}
	return s.entries[code][0]
}

// stringsAt resolves a #STRINGS offset to NUL-terminated text. A string
// crossing a 4K block end is cut without NUL and repeated in full at the
// next block start, so fall through to the next boundary. See chmspec 5.3.6.
func stringsAt(raw []byte, off uint32, charset string) string {
	if uint64(off) >= uint64(len(raw)) {
		return ""
	}
	rest := raw[off:]
	if i := bytes.IndexByte(rest, 0); i >= 0 && off+uint32(i) < (off|0xFFF)+1 {
		return decodeText(rest[:i], charset)
	}
	next := (off | 0xFFF) + 1
	if uint64(next) >= uint64(len(raw)) {
		return decodeText(nulCut(rest), charset)
	}
	return decodeText(nulCut(raw[next:]), charset)
}

// nulCut truncates at the first NUL.
func nulCut(b []byte) []byte {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		return b[:i]
	}
	return b
}

// decodeText decodes codepage bytes, falling back to raw bytes when the
// charset is unknown.
func decodeText(raw []byte, charset string) string {
	raw = bytes.TrimRight(raw, "\x00")
	if len(raw) == 0 {
		return ""
	}
	enc, err := htmlindex.Get(charset)
	if err != nil {
		return string(raw)
	}
	dec, err := enc.NewDecoder().Bytes(raw)
	if err != nil {
		return string(raw)
	}
	return string(dec)
}

// squash collapses whitespace runs to single spaces.
func squash(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
