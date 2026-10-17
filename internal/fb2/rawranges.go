package fb2

import (
	"fmt"
	"unicode/utf8"

	"github.com/f0d0r/margaret-ebook-library/internal/xml"
)

// rawBodyRanges locates top-level <body> content ranges in raw (untranscoded)
// FB2 bytes, returning [start, end) offsets into data where start is the
// first byte after the <body ...> start tag and end is the '<' of the
// matching </body>.
//
// The scan is ASCII-only, which is exact for UTF-8 and all single-byte
// encodings (markup bytes are ASCII there and can never appear inside
// encoded text). UTF-16 documents must be decoded to UTF-8 first and the
// resulting ranges mapped back with mapUTF8RangesToRaw.
//
// Depth counting mirrors findBodyRanges (encoding/xml based): a <body> whose
// parent is the document root (depth 2) opens a range; truncation (EOF with
// an open body) extends the range to the end of input, like the
// isTruncated branch there. Comments, PIs, CDATA sections and quoted
// attribute values are skipped so markup inside them never affects depth.
// Truncation before any body opens errors, mirroring findBodyRanges'
// unexpected-EOF error. DOCTYPE internal subsets are skipped only
// best-effort (to the first quote-aware '>'); FB2 documents never carry one.
func rawBodyRanges(data []byte) ([][2]int64, error) {
	var out [][2]int64
	depth := 0
	var start int64 = -1
	clean := true

	i := 0
	for i < len(data) {
		if data[i] != '<' {
			i++
			continue
		}
		tagStart := i
		i++
		if i >= len(data) {
			clean = false
			break // truncated mid-tag
		}
		c := data[i]
		switch {
		case c == '/':
			name, next, ok := scanTagName(data, i+1)
			if !ok {
				clean = false
				i = len(data)
				break
			}
			end, ok := skipToTagEnd(data, next)
			i = end
			if !ok {
				clean = false
			}
			if depth == 2 && localPart(name) == elBody && start >= 0 {
				out = append(out, [2]int64{start, int64(tagStart)})
				start = -1
			}
			depth--
			if depth < 0 {
				depth = 0
			}
		case c == '?':
			next, ok := skipUntilMarker(data, i+1, "?>")
			i = next
			if !ok {
				clean = false
			}
		case c == '!':
			switch {
			case hasPrefixAt(data, i, "<!--"):
				next, ok := skipUntilMarker(data, i+4, "-->")
				i = next
				if !ok {
					clean = false
				}
			case hasPrefixAt(data, i, "<![CDATA["):
				next, ok := skipUntilMarker(data, i+9, "]]>")
				i = next
				if !ok {
					clean = false
				}
			default:
				end, ok := skipToTagEnd(data, i+1)
				i = end
				if !ok {
					clean = false
				}
			}
		case xml.IsNameChar(c):
			name, next, ok := scanTagName(data, i)
			if !ok {
				clean = false
				i = len(data)
				break
			}
			end, selfClosing, ok := skipStartTag(data, next)
			i = end
			if !ok {
				clean = false
			}
			depth++
			if depth == 2 && localPart(name) == elBody {
				if selfClosing {
					out = append(out, [2]int64{int64(end), int64(end)})
				} else {
					start = int64(end)
				}
			}
			if selfClosing {
				depth--
			}
		default:
			// Bare '<' followed by a non-name byte: literal text.
			// i already points past '<'; continue from there.
		}
	}
	if start >= 0 {
		// Truncated inside a body: extend to end of input.
		out = append(out, [2]int64{start, int64(len(data))})
		return out, nil
	}
	if !clean || depth != 0 {
		return nil, fmt.Errorf("truncated fb2 document")
	}
	return out, nil
}

// scanTagName reads a tag name starting at pos (first name byte).
// Stray NUL bytes are ignored for name matching (mirroring normalizeFB2's
// NUL stripping) while offsets stay absolute into data. It returns the name
// (possibly prefixed), the offset after it, and false on truncation.
func scanTagName(data []byte, pos int) (string, int, bool) {
	j := pos
	var name []byte
	for j < len(data) && (xml.IsNameChar(data[j]) || data[j] == 0x00) {
		if data[j] != 0x00 {
			name = append(name, data[j])
		}
		j++
	}
	if j >= len(data) {
		return string(name), j, false
	}
	return string(name), j, true
}

// skipToTagEnd skips to (and past) the '>' closing a tag, honoring quotes.
// ok is false on truncation.
func skipToTagEnd(data []byte, pos int) (int, bool) {
	for pos < len(data) {
		switch data[pos] {
		case '>':
			return pos + 1, true
		case '"', '\'':
			next, ok := skipQuoted(data, pos)
			pos = next
			if !ok {
				return pos, false
			}
		default:
			pos++
		}
	}
	return len(data), false
}

// skipStartTag skips attributes to the end of a start tag, reporting the
// offset past '>' (or "/>"), whether it was self-closing, and false on
// truncation.
func skipStartTag(data []byte, pos int) (int, bool, bool) {
	for pos < len(data) {
		switch data[pos] {
		case '>':
			return pos + 1, false, true
		case '"', '\'':
			next, ok := skipQuoted(data, pos)
			pos = next
			if !ok {
				return pos, true, false
			}
		case '/':
			if pos+1 < len(data) && data[pos+1] == '>' {
				return pos + 2, true, true
			}
			pos++
		default:
			pos++
		}
	}
	return len(data), true, false
}

// skipQuoted skips a quoted section starting at the quote byte.
// ok is false on truncation.
func skipQuoted(data []byte, pos int) (int, bool) {
	q := data[pos]
	pos++
	for pos < len(data) {
		if data[pos] == q {
			return pos + 1, true
		}
		pos++
	}
	return len(data), false
}

// skipUntilMarker skips to (and past) marker. ok is false on truncation.
func skipUntilMarker(data []byte, pos int, marker string) (int, bool) {
	m := marker[0]
	for pos < len(data) {
		if data[pos] == m && hasPrefixAt(data, pos, marker) {
			return pos + len(marker), true
		}
		pos++
	}
	return len(data), false
}

func hasPrefixAt(data []byte, pos int, s string) bool {
	if pos+len(s) > len(data) {
		return false
	}
	for i := 0; i < len(s); i++ {
		if data[pos+i] != s[i] {
			return false
		}
	}
	return true
}

// localPart strips an optional namespace prefix.
func localPart(name string) string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == ':' {
			return name[i+1:]
		}
	}
	return name
}

// utf16Layout reports the byte order and the length of the leading BOM of a
// UTF-16 document. A BOM wins; otherwise the ASCII prolog skeleton ('<'
// adjacent to a NUL byte) picks the order; otherwise an explicit
// utf-16le/utf-16be charset label is trusted. ok is false when the byte
// order cannot be determined (plain "utf-16" without BOM or skeleton).
func utf16Layout(raw []byte, charset string) (littleEndian bool, skip int, ok bool) {
	if len(raw) >= 2 {
		switch {
		case raw[0] == 0xFF && raw[1] == 0xFE:
			return true, 2, true
		case raw[0] == 0xFE && raw[1] == 0xFF:
			return false, 2, true
		case raw[0] == '<' && raw[1] == 0x00:
			return true, 0, true
		case raw[0] == 0x00 && raw[1] == '<':
			return false, 0, true
		}
	}
	switch charset {
	case "utf-16le":
		return true, 0, true
	case "utf-16be":
		return false, 0, true
	}
	return false, 0, false
}

// locateRawBodyRanges finds body ranges in the raw FB2 document. For UTF-8
// and single-byte encodings the raw bytes scan directly; for UTF-16 the
// document is decoded once for scanning and the ranges are mapped back to
// raw offsets. wantCount is the body count found on the normalized copy; a
// mismatch fails loudly instead of serving misaligned slices.
func locateRawBodyRanges(raw []byte, charset string, wantCount int) ([][2]int64, error) {
	var ranges [][2]int64
	switch charset {
	case "utf-16", "utf-16le", "utf-16be":
		if len(raw) < 2 {
			return nil, fmt.Errorf("truncated utf-16 document")
		}
		le, skip, ok := utf16Layout(raw, charset)
		if !ok {
			return nil, fmt.Errorf("indeterminate utf-16 byte order")
		}
		decoded, err := utf16ToUTF8(raw[skip:], le)
		if err != nil {
			return nil, err
		}
		scanned, err := rawBodyRanges(decoded)
		if err != nil {
			return nil, err
		}
		mapped, err := mapUTF8RangesToRaw(raw[skip:], le, decoded, scanned)
		if err != nil {
			return nil, err
		}
		ranges = make([][2]int64, len(mapped))
		for i, r := range mapped {
			ranges[i] = [2]int64{r[0] + int64(skip), r[1] + int64(skip)}
		}
	default:
		scanned, err := rawBodyRanges(raw)
		if err != nil {
			return nil, err
		}
		ranges = scanned
	}
	if len(ranges) != wantCount {
		return nil, fmt.Errorf("body location mismatch: raw scan found %d bodies, normalized scan found %d", len(ranges), wantCount)
	}
	return ranges, nil
}

// utf16ToUTF8 decodes raw UTF-16 (littleEndian selects LE/BE, BOM already
// consumed by the caller) to UTF-8. It mirrors normalizeFB2's strictness:
// odd trailing bytes and lone surrogates error.
func utf16ToUTF8(raw []byte, littleEndian bool) ([]byte, error) {
	if len(raw)%2 != 0 {
		return nil, fmt.Errorf("odd-length utf-16 document")
	}
	out := make([]byte, 0, len(raw))
	var buf [utf8.UTFMax]byte
	for i := 0; i < len(raw); i += 2 {
		var u uint16
		if littleEndian {
			u = uint16(raw[i]) | uint16(raw[i+1])<<8
		} else {
			u = uint16(raw[i])<<8 | uint16(raw[i+1])
		}
		r := rune(u)
		if isUTF16HighSurrogate(u) {
			if i+2 >= len(raw) {
				return nil, fmt.Errorf("truncated utf-16 surrogate pair")
			}
			var lo uint16
			if littleEndian {
				lo = uint16(raw[i+2]) | uint16(raw[i+3])<<8
			} else {
				lo = uint16(raw[i+2])<<8 | uint16(raw[i+3])
			}
			if !isUTF16LowSurrogate(lo) {
				return nil, fmt.Errorf("lone utf-16 surrogate")
			}
			r = 0x10000 + (rune(u-0xD800) << 10) + rune(lo-0xDC00)
			i += 2
		} else if isUTF16LowSurrogate(u) {
			return nil, fmt.Errorf("lone utf-16 surrogate")
		}
		n := utf8.EncodeRune(buf[:], r)
		out = append(out, buf[:n]...)
	}
	return out, nil
}

func isUTF16HighSurrogate(u uint16) bool { return u >= 0xD800 && u <= 0xDBFF }
func isUTF16LowSurrogate(u uint16) bool  { return u >= 0xDC00 && u <= 0xDFFF }

// mapUTF8RangesToRaw translates ranges located in utf8copy (a decoding of
// raw) back to raw byte offsets. Endpoints sit at ASCII tag boundaries, so
// walking runes on both sides lands exactly. Walks are O(bodies x size);
// no index array is built, keeping memory flat for large documents.
func mapUTF8RangesToRaw(raw []byte, littleEndian bool, utf8copy []byte, ranges [][2]int64) ([][2]int64, error) {
	out := make([][2]int64, 0, len(ranges))
	for _, r := range ranges {
		lo, err := utf8OffsetToRaw(raw, littleEndian, utf8copy, r[0])
		if err != nil {
			return nil, err
		}
		hi, err := utf8OffsetToRaw(raw, littleEndian, utf8copy, r[1])
		if err != nil {
			return nil, err
		}
		if hi < lo {
			return nil, fmt.Errorf("invalid mapped range")
		}
		out = append(out, [2]int64{lo, hi})
	}
	return out, nil
}

// utf8OffsetToRaw maps a byte offset in utf8copy (must be a rune boundary)
// to the corresponding raw byte offset by counting runes on both sides.
func utf8OffsetToRaw(raw []byte, littleEndian bool, utf8copy []byte, off int64) (int64, error) {
	if off < 0 || off > int64(len(utf8copy)) {
		return 0, fmt.Errorf("offset out of range")
	}
	// Count runes in utf8copy[:off]; off must be a boundary.
	var runes int64
	for pos := int64(0); pos < off; {
		_, size := utf8.DecodeRune(utf8copy[pos:])
		if size <= 0 || pos+int64(size) > off {
			return 0, fmt.Errorf("offset is not a rune boundary")
		}
		pos += int64(size)
		runes++
	}
	// Advance the same number of runes (UTF-16 units) in raw.
	rawPos := 0
	for k := int64(0); k < runes; k++ {
		if rawPos+2 > len(raw) {
			return 0, fmt.Errorf("raw exhausted during mapping")
		}
		var u uint16
		if littleEndian {
			u = uint16(raw[rawPos]) | uint16(raw[rawPos+1])<<8
		} else {
			u = uint16(raw[rawPos])<<8 | uint16(raw[rawPos+1])
		}
		rawPos += 2
		if isUTF16HighSurrogate(u) {
			rawPos += 2
			if rawPos > len(raw) {
				return 0, fmt.Errorf("raw exhausted during mapping")
			}
		}
	}
	return int64(rawPos), nil
}
