package charset

import (
	"bytes"
	"io"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// decoderFor returns a decoder transforming charset bytes to UTF-8, or nil
// when the input is already UTF-8. The UTF-16 family uses BOM-aware x/text
// decoders (a BOM is consumed and picks the endianness); Binary and
// unsupported labels error: decoding those would corrupt data, so callers
// must not get here with them (DecodeForTransform guards first).
func decoderFor(charset string) (transform.Transformer, error) {
	cs := Normalize(charset)
	switch cs {
	case "", UTF8, Binary:
		return nil, nil
	case "utf-16":
		return unicode.UTF16(unicode.BigEndian, unicode.UseBOM).NewDecoder(), nil
	case "utf-16le":
		return unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder(), nil
	case "utf-16be":
		return unicode.UTF16(unicode.BigEndian, unicode.UseBOM).NewDecoder(), nil
	}
	enc, err := htmlindex.Get(cs)
	if err != nil {
		return nil, err
	}
	return enc.NewDecoder(), nil
}

// sniffLen bounds the peek window for declaration sniffing during transform.
const sniffLen = 8192

// DecodeForTransform wraps rc so downstream transformers always see UTF-8.
// hint is the resource's declared charset (may be "" for hand-built
// resources): an explicit non-UTF-8 hint wins; otherwise the head is
// sniffed for XML/meta declarations; UTF-8 (or binary media) passes
// through byte-identically.
func DecodeForTransform(rc io.ReadCloser, hint, mediaType string) io.ReadCloser {
	if rc == nil {
		return nil
	}
	if !IsTextMedia(mediaType) {
		return rc
	}
	cs := Normalize(hint)
	if cs == "" {
		cs = UTF8
	}
	if cs == Binary {
		return rc
	}
	if cs != UTF8 {
		dec, err := decoderFor(cs)
		if err != nil || dec == nil {
			return rc
		}
		return &closeProxy{Reader: transform.NewReader(rc, dec), Closer: rc}
	}
	head := make([]byte, sniffLen)
	n, _ := io.ReadFull(rc, head)
	head = head[:n]
	rest := io.MultiReader(bytes.NewReader(head), rc)
	if len(head) == 0 {
		return &closeProxy{Reader: rest, Closer: rc}
	}
	if sniffed, ok := DetectDeclared(head); ok {
		dec, err := decoderFor(sniffed)
		if err != nil || dec == nil {
			return &closeProxy{Reader: rest, Closer: rc}
		}
		return &closeProxy{Reader: transform.NewReader(rest, dec), Closer: rc}
	}
	return &closeProxy{Reader: rest, Closer: rc}
}

// Encode re-encodes a UTF-8 text stream to outCharset.
// ""/UTF8/Binary pass through; the UTF-16 family uses BOM-writing
// x/text encoders; anything else goes through the x/text encoder, erroring
// (and closing rc) on unknown labels.
func Encode(rc io.ReadCloser, outCharset string) (io.ReadCloser, error) {
	cs := Normalize(outCharset)
	var enc encoding.Encoding
	switch cs {
	case "", UTF8, Binary:
		return rc, nil
	case "utf-16":
		enc = unicode.UTF16(unicode.BigEndian, unicode.UseBOM)
	case "utf-16le":
		enc = unicode.UTF16(unicode.LittleEndian, unicode.UseBOM)
	case "utf-16be":
		enc = unicode.UTF16(unicode.BigEndian, unicode.UseBOM)
	default:
		var err error
		enc, err = htmlindex.Get(cs)
		if err != nil {
			_ = rc.Close()
			return nil, err
		}
	}
	return &closeProxy{Reader: transform.NewReader(rc, enc.NewEncoder()), Closer: rc}, nil
}

// closeProxy pairs a transforming Reader with the underlying Closer.
type closeProxy struct {
	io.Reader
	Closer io.Closer
}

func (c *closeProxy) Close() error {
	if c == nil || c.Closer == nil {
		return nil
	}
	return c.Closer.Close()
}
