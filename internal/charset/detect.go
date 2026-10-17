package charset

import (
	"regexp"

	"golang.org/x/text/encoding/htmlindex"
)

var (
	xmlEncodingRe = regexp.MustCompile(`(?i)<\?xml\s+[^>]*?\bencoding\s*=\s*['"]([^'"]+)['"]`)
	metaCharsetRe = regexp.MustCompile(`(?i)<meta[^>]*?charset\s*=\s*['"]?([^'"\s/>;]+)`)
)

// xmlPrologScanLimit bounds the XML declaration search (mirrors the FB2
// prolog limit); htmlHeadScanLimit bounds the <meta charset> search.
const (
	xmlPrologScanLimit = 2048
	htmlHeadScanLimit  = 8192
)

// DetectDeclared sniffs an explicit encoding declaration from head:
// first UTF-16 (BOM or the UTF-16 byte pattern of an ASCII prolog), then the
// XML encoding declaration, then an HTML <meta charset>.
// It returns the normalized label and true only when a supported,
// non-UTF-8 encoding is detected. UTF-8 (or no declaration) reports
// (UTF8, false) so callers pass the bytes through untouched.
func DetectDeclared(head []byte) (string, bool) {
	if cs, ok := utf16Label(head); ok {
		return cs, true
	}
	if len(head) > htmlHeadScanLimit {
		head = head[:htmlHeadScanLimit]
	}
	prolog := head
	if len(prolog) > xmlPrologScanLimit {
		prolog = prolog[:xmlPrologScanLimit]
	}
	if m := xmlEncodingRe.FindSubmatch(prolog); m != nil {
		if cs, ok := validatedCharset(string(m[1])); ok {
			return cs, true
		}
	}
	if m := metaCharsetRe.FindSubmatch(head); m != nil {
		if cs, ok := validatedCharset(string(m[1])); ok {
			return cs, true
		}
	}
	return UTF8, false
}

// utf16Label recognizes UTF-16 input by its BOM or by the UTF-16 encoding of
// an ASCII prolog start ('<' followed or preceded by a NUL byte); the latter
// also covers BOM-less files written by Windows tools.
func utf16Label(head []byte) (string, bool) {
	if len(head) >= 2 {
		switch {
		case head[0] == 0xFE && head[1] == 0xFF:
			return "utf-16be", true
		case head[0] == 0xFF && head[1] == 0xFE:
			return "utf-16le", true
		case head[0] == '<' && head[1] == 0x00:
			return "utf-16le", true
		case head[0] == 0x00 && head[1] == '<':
			return "utf-16be", true
		}
	}
	return "", false
}

// validatedCharset normalizes label and reports whether it names a
// supported non-UTF-8 encoding. UTF-8 family labels report false (nothing
// to transcode); unknown labels report false (caller keeps raw bytes
// rather than guessing).
func validatedCharset(label string) (string, bool) {
	cs := Normalize(label)
	switch cs {
	case "", UTF8:
		return UTF8, false
	case "utf-16", "utf-16le", "utf-16be":
		return cs, true
	}
	if _, err := htmlindex.Get(cs); err != nil {
		return UTF8, false
	}
	return cs, true
}
