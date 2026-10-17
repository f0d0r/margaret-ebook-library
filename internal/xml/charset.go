// Package xml provides XML helpers shared across readers: an encoding/xml
// decoder that accepts UTF-16 input and resolves declared charsets, and the
// lightweight name/tag scanners used for format sniffing.
package xml

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// CharsetReader resolves an XML encoding declaration for
// xml.Decoder.CharsetReader. UTF-8/ASCII labels and the UTF-16 family pass
// the input through unchanged (UTF-16 input is transcoded before XML
// scanning, see NewDecoder; the FB2 reader handles UTF-16 at the byte
// level); every other label is decoded via x/text. Unknown labels error so
// callers can reject the document instead of mis-decoding it.
func CharsetReader(label string, input io.Reader) (io.Reader, error) {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "", "utf-8", "utf8", "us-ascii", "ascii", "utf-16", "utf-16le", "utf-16be":
		return input, nil
	default:
		enc, err := htmlindex.Get(label)
		if err != nil {
			return nil, fmt.Errorf("unsupported xml encoding %q", label)
		}
		return transform.NewReader(input, enc.NewDecoder()), nil
	}
}

// utf16SniffLen bounds the lookahead used to recognize UTF-16 input.
const utf16SniffLen = 32

// NewDecoder returns an xml.Decoder reading r, with CharsetReader installed
// as CharsetReader. Unlike a bare xml.NewDecoder it also accepts UTF-16
// input: encoding/xml's scanner needs ASCII-compatible bytes to reach the
// encoding declaration, so a UTF-16 BOM (or the UTF-16 byte pattern of an
// ASCII XML prolog) is recognized up front and the stream is transcoded to
// UTF-8. The declaration's own label is then a harmless passthrough in
// CharsetReader.
func NewDecoder(r io.Reader) *xml.Decoder {
	dec := xml.NewDecoder(decodeUTF16(r))
	dec.CharsetReader = CharsetReader
	return dec
}

// decodeUTF16 wraps r in a UTF-16 decoder when a BOM or a BOM-less UTF-16
// encoding of the XML prolog start is detected; otherwise r passes through
// untouched.
func decodeUTF16(r io.Reader) io.Reader {
	br := bufio.NewReaderSize(r, utf16SniffLen)
	head, _ := br.Peek(utf16SniffLen)
	var order unicode.Endianness
	switch {
	case len(head) >= 2 && head[0] == 0xFE && head[1] == 0xFF:
		order = unicode.BigEndian
	case len(head) >= 2 && head[0] == 0xFF && head[1] == 0xFE:
		order = unicode.LittleEndian
	case len(head) >= 2 && head[0] == '<' && head[1] == 0x00:
		order = unicode.LittleEndian
	case len(head) >= 2 && head[0] == 0x00 && head[1] == '<':
		order = unicode.BigEndian
	default:
		return br
	}
	return transform.NewReader(br, unicode.UTF16(order, unicode.UseBOM).NewDecoder())
}
