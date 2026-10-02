package mobi

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/util"
)

// isPalmDocDb reports whether the PDB container holds a PalmDOC book
// (type TEXt, creator REAd), case-insensitively like calibre which
// uppercases the 8-byte ident and tests for TEXTREAD.
func isPalmDocDb(pdbDb *PdbDb) bool {
	if pdbDb == nil {
		return false
	}
	return strings.ToUpper(pdbDb.Type+pdbDb.Creator) == pdbIdentPalmDoc
}

// readPalmDocHeader parses the 16-byte PalmDOC header from record 0.
// It mirrors calibre's BookHeader ancient path (headers.py): cp1252 codec,
// zero extra flags, MOBI version 1. The MOBI field layout must not be
// applied here: past byte 16 the record already carries text.
func readPalmDocHeader(pdbDb *PdbDb) (*Mobi, error) {
	if len(pdbDb.PdbRecords) == 0 {
		return nil, fmt.Errorf("no records in PDB database")
	}
	data, err := pdbDb.PdbRecords[0].Data()
	if err != nil {
		return nil, fmt.Errorf("failed to read Record 0: %w", err)
	}
	if len(data) < PALM_DOC_HEADER_SIZE {
		return nil, fmt.Errorf("pdb record 0 is too short for PalmDOC header")
	}

	mobi := newMobiDefaults()
	mobi.readBaseHeader(data)
	mobi.Type = MobiTypePalmDoc
	mobi.TextEncoding = CP1252
	mobi.MobiVersion = 1
	mobi.Codec = "cp1252"
	mobi.name = pdbDb.Name
	mobi.recordCount = pdbDb.NumberOfRecords
	return mobi, nil
}

// palmDocSniffSize bounds how many decompressed bytes the HTML sniff
// inspects. The OEB 1.0 payloads open with <HTML><HEAD> so a small window
// is enough; calibre uses the first 300 bytes.
const palmDocSniffSize = 4096

// isPalmDocHTML reports whether decompressed PalmDOC text looks like HTML.
// Only the media type decision uses it: the payload bytes are preserved
// as-is either way. It trims the leading 0x0E padding, NULs, BOM and
// whitespace, then scans case-insensitively for an opening html/head/body
// tag with a proper delimiter, following the FB2 FictionBook sniff pattern.
func isPalmDocHTML(data []byte) bool {
	trimmed := trimPalmDocPrefix(data)
	if len(trimmed) == 0 {
		return false
	}
	if len(trimmed) > palmDocSniffSize {
		trimmed = trimmed[:palmDocSniffSize]
	}
	for _, tag := range []string{"html", "head", "body"} {
		if util.ScanOpenTag(trimmed, tag, true, false) {
			return true
		}
	}
	return false
}

// trimPalmDocPrefix drops PalmDOC leading padding for sniffing only:
// 0x0E record padding, NULs, UTF-8 BOM and ASCII whitespace.
func trimPalmDocPrefix(data []byte) []byte {
	data = bytes.TrimLeft(data, "\x0e\x00")
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	return bytes.TrimLeft(data, " \t\n\r\f\v")
}

// palmDocPrefixCap bounds the decompressed prefix used for the HTML sniff
// and the OEB metadata. Decompression stops early once a complete
// <metadata>...</metadata> fragment is present, so typical books only cost
// a record or two; the cap is a safety bound and Read stays cheap while
// the full content remains lazily-opened.
const palmDocPrefixCap = 32 * 1024

// palmDocPrefixText decompresses record 0 past the header plus following
// records until a complete metadata fragment is present or cap bytes are
// gathered. It is best-effort for sniffing: callers ignore the error and
// fall back to plain text, the full error surfaces from Open via
// extractPalmDocText.
func palmDocPrefixText(pdbDb *PdbDb, mobi *Mobi) ([]byte, error) {
	switch mobi.Compression {
	case CompressionNone, 0, CompressionPalmDOC, CompressionPalmDOCAlt:
	default:
		return nil, fmt.Errorf("unsupported PalmDOC compression type %d", mobi.Compression)
	}
	if len(pdbDb.PdbRecords) == 0 {
		return nil, fmt.Errorf("no records in PDB database")
	}
	var out []byte
	for i := range pdbDb.PdbRecords {
		data, err := pdbDb.PdbRecords[i].Data()
		if err != nil {
			return out, err
		}
		section := data
		if i == 0 {
			if len(data) < PALM_DOC_HEADER_SIZE {
				return out, fmt.Errorf("pdb record 0 is too short for PalmDOC header")
			}
			section = data[PALM_DOC_HEADER_SIZE:]
		}
		// Unlimited per-record budget with truncation below: a large first
		// record still yields a usable prefix instead of an empty one.
		dec, err := decompressTextSection(section, mobi.Compression, nil, -1, i)
		if err != nil {
			return out, err
		}
		if int64(len(out)+len(dec)) > palmDocPrefixCap {
			out = append(out, dec[:palmDocPrefixCap-len(out)]...)
			return out, nil
		}
		out = append(out, dec...)
		if hasMetadataEnd(out) {
			return out, nil
		}
	}
	return out, nil
}

// hasMetadataEnd reports whether out already holds a complete
// <metadata>...</metadata> fragment, letting the prefix stop early
// instead of decompressing up to the cap.
func hasMetadataEnd(out []byte) bool {
	return containsFold(out, []byte("</metadata>"))
}

// containsFold reports whether needle occurs in haystack,
// matched case-insensitively without allocating a lowered copy.
func containsFold(haystack, needle []byte) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if !foldEq(haystack[i], needle[0]) {
			continue
		}
		match := true
		for j := 1; j < len(needle); j++ {
			if !foldEq(haystack[i+j], needle[j]) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func foldEq(a, b byte) bool {
	if a == b {
		return true
	}
	if 'A' <= a && a <= 'Z' {
		a += 'a' - 'A'
	}
	if 'A' <= b && b <= 'Z' {
		b += 'a' - 'A'
	}
	return a == b
}

// DecompressPalmDoc decompresses a block compressed with PalmDOC (LZ77)
// compression, as used for the text records of a MOBI file. It mirrors the
// reference calibre cPalmdoc.decompress behaviour: malformed tokens (truncated
// literal runs or back-references whose distance exceeds the output produced
// so far) are silently skipped rather than reported as errors.
//
// remain is the remaining budget for decompressed bytes (maxSize - len(out) from
// the caller). If remain >=0, decompression fails with book.ErrLimitExceeded
// when the output would exceed the remaining budget. Pass remain <0 for unlimited.
func DecompressPalmDoc(in []byte, remain int64) ([]byte, error) {
	capHint := len(in) * 2
	if remain >= 0 && int64(capHint) > remain {
		capHint = int(remain)
		capHint = max(capHint, 0)
	}
	out := make([]byte, 0, capHint)
	p := 0
	for p < len(in) {
		c := int(in[p])
		p++
		switch {
		case c >= 1 && c <= 8:
			// Copy the next c bytes literally.
			if p+c > len(in) {
				c = len(in) - p
			}
			if remain >= 0 && int64(len(out)+c) > remain {
				return nil, book.ErrLimitExceeded
			}
			out = append(out, in[p:p+c]...)
			p += c
		case c < 128:
			// Literal byte.
			if remain >= 0 && int64(len(out)+1) > remain {
				return nil, book.ErrLimitExceeded
			}
			out = append(out, byte(c))
		case c >= 192:
			// Literal space plus the byte with bit 0x80 cleared.
			if remain >= 0 && int64(len(out)+2) > remain {
				return nil, book.ErrLimitExceeded
			}
			out = append(out, ' ', byte(c^128))
		default:
			// Two-byte back-reference: distance m, length n. If the second
			// byte is missing or the distance exceeds the output produced so
			// far, the token is ignored.
			if p >= len(in) {
				continue
			}
			c = (c << 8) | int(in[p])
			p++
			m := (c >> 3) & 0x07ff
			n := (c & 7) + 3
			if m > len(out) {
				continue
			}
			if m > n {
				// No overlap between source and destination ranges.
				if remain >= 0 && int64(len(out)+n) > remain {
					return nil, book.ErrLimitExceeded
				}
				out = append(out, out[len(out)-m:len(out)-m+n]...)
				continue
			}
			if m == 0 {
				if remain >= 0 && int64(len(out)+n) > remain {
					return nil, book.ErrLimitExceeded
				}
				out = append(out, make([]byte, n)...)
				continue
			}
			// Overlapping copy: byte at distance m, re-evaluated as the
			// output grows, repeated n times.
			if remain >= 0 && int64(len(out)+n) > remain {
				return nil, book.ErrLimitExceeded
			}
			for range n {
				out = append(out, out[len(out)-m])
			}
		}
	}
	return out, nil
}
