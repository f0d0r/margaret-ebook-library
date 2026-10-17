// Package charset centralizes the charset-label handling shared by the
// format readers and the book layer: label normalization, media-type text
// classification, declared-charset sniffing for XML/HTML prologs, and the
// UTF-8 decode/re-encode stream wrappers used by Resource.OpenAs.
package charset

import (
	"strings"

	"github.com/f0d0r/margaret-ebook-library/mediatype"
	"golang.org/x/text/encoding/htmlindex"
)

// Charset labels used in Resource.Charset.
const (
	// UTF8 marks textual content stored as UTF-8 (explicit default when
	// no other encoding is declared).
	UTF8 = "utf-8"
	// Binary marks non-textual content (images, octet-stream, ...) for
	// which a character set is meaningless. It is a sentinel, not a
	// real encoding: never passed to a decoder.
	Binary = "binary"
)

// textApplicationTypes are application/* MIME types with textual payloads.
var textApplicationTypes = map[string]bool{
	mediatype.XHTML:        true,
	mediatype.HTML:         true, // text/html also matches the text/ prefix; listed for clarity
	mediatype.MobiHTML:     true,
	mediatype.FB2Body:      true,
	mediatype.FB3Body:      true,
	mediatype.EreaderPML:   true,
	mediatype.OEB1Document: true,
	mediatype.NCX:          true,
	mediatype.OPF:          true,
	mediatype.CSS:          true, // text/css also matches the text/ prefix; listed for clarity
	mediatype.SVG:          true, // image/svg+xml is XML text; its bytes carry an encoding declaration
}

// IsTextMedia reports whether mime names a textual payload whose bytes are
// governed by a resource charset. Everything else (image/*,
// application/octet-stream, ...) is binary.
func IsTextMedia(mime string) bool {
	m := mediatype.Normalize(mime)
	if m == "" {
		return false
	}
	if strings.HasPrefix(m, "text/") {
		return true
	}
	if strings.HasSuffix(m, "+xml") || strings.HasSuffix(m, "+json") {
		return true
	}
	if m == "application/xml" || m == "application/json" {
		return true
	}
	return textApplicationTypes[m]
}

// Default returns the charset value for a media type when the format
// reader has no better information: UTF8 for text, Binary otherwise.
// It never returns "".
func Default(mediaType string) string {
	if IsTextMedia(mediaType) {
		return UTF8
	}
	return Binary
}

// Normalize canonicalizes a charset label: trims whitespace, lowercases,
// and folds common aliases ("utf8" -> "utf-8", "us-ascii"/"ascii" ->
// "utf-8", "cp1252" -> "windows-1252"). A cpNNN label is only rewritten
// when the resulting windows-NNN label is actually known (cp866 stays
// cp866, which decoders understand). Binary is preserved as-is. "" stays
// "" (callers map it via Default); it must never escape the library on a
// Resource.
func Normalize(label string) string {
	s := strings.ToLower(strings.TrimSpace(label))
	switch s {
	case "", Binary:
		return s
	case "utf8":
		return UTF8
	case "ascii", "us-ascii":
		return UTF8
	case "utf-16", "utf-16le", "utf-16be":
		return s
	}
	if strings.HasPrefix(s, "cp") {
		if n := strings.TrimPrefix(s, "cp"); n != "" && isDigits(n) {
			if w := "windows-" + n; isKnownCharsetLabel(w) {
				return w
			}
		}
		return s
	}
	return s
}

// isKnownCharsetLabel reports whether htmlindex can resolve label.
func isKnownCharsetLabel(label string) bool {
	_, err := htmlindex.Get(label)
	return err == nil
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
