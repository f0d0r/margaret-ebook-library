package fb2

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"unicode"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/util"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

// binaryFile is one decoded <binary> payload.
type binaryFile struct {
	id        string // FB2 binary id
	href      string // images/<sanitized-id>.<ext>
	mediaType string
	data      []byte
}

// collectBinaries streams the document once, decoding every image <binary>
// payload. Non-image binaries are skipped, corrupt payloads are skipped
// (calibre parity: log-and-ignore, here silent to keep Read total).
func collectBinaries(data []byte) ([]*binaryFile, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = charsetReader
	var out []*binaryFile
	used := make(map[string]int)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != elBinary {
			continue
		}
		bf, err := decodeBinary(dec, start, used)
		if err != nil {
			return nil, err
		}
		if bf != nil {
			out = append(out, bf)
		}
	}
}

// decodeBinary consumes one <binary> element (decoder positioned after its
// start tag). It returns nil, nil for skipped payloads (non-image declared
// type, undecodable data, unidentifiable type).
func decodeBinary(dec *xml.Decoder, start xml.StartElement, used map[string]int) (*binaryFile, error) {
	var id, contentType string
	for _, a := range start.Attr {
		switch a.Name.Local {
		case attrID:
			id = a.Value
		case attrContentType:
			contentType = a.Value
		}
	}
	payload, err := textOf(dec)
	if err != nil {
		return nil, err
	}
	mediaTypePart, _, _ := strings.Cut(contentType, ";")
	declared := strings.ToLower(strings.TrimSpace(mediaTypePart))
	if declared != "" && !strings.HasPrefix(declared, "image/") {
		return nil, nil
	}
	raw, err := decodeBase64(payload)
	if err != nil || len(raw) == 0 {
		return nil, nil
	}
	mediaType := declared
	ext, ok := mediatype.Extension(declared)
	if !ok {
		if media := util.DetectImageMedia(raw[:min(32, len(raw))]); media != nil {
			mediaType, ext = media.Type, media.Extension
		} else {
			return nil, nil
		}
	}
	stem := sanitizeImageID(id)
	if !strings.HasSuffix(strings.ToLower(stem), "."+ext) {
		stem += "." + ext
	}
	href := "images/" + stem
	if n := used[href]; n > 0 {
		href = fmt.Sprintf("images/%s-%d.%s", strings.TrimSuffix(stem, "."+ext), n+1, ext)
	}
	used[href]++
	return &binaryFile{
		id:        id,
		href:      util.CleanHref(href),
		mediaType: mediaType,
		data:      raw,
	}, nil
}

// decodeBase64 decodes a base64 payload tolerantly: surrounding and embedded
// whitespace is ignored (pretty-printed binaries) and missing padding is
// restored, mirroring Python's non-validating b64decode that calibre uses.
func decodeBase64(payload string) ([]byte, error) {
	s := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, payload)
	if m := len(s) % 4; m != 0 {
		s += strings.Repeat("=", 4-m)
	}
	return base64.StdEncoding.DecodeString(s)
}

// sanitizeImageID maps a binary id to a safe file name stem (calibre's
// sanitize_file_name parity): letters and digits are kept, the rest becomes
// '_'.
func sanitizeImageID(id string) string {
	var sb strings.Builder
	for _, r := range id {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '-', r == '_' || r == '.':
			sb.WriteRune(r)
		default:
			sb.WriteByte('_')
		}
	}
	if sb.Len() == 0 {
		return "image"
	}
	return sb.String()
}

// findBodyRanges returns [start, end) byte ranges of every top-level body's
// inner content in document order. Offsets index data, so data must be
// normalized UTF-8 with an honest declaration (see normalizeFB2):
// transcoding shifts offsets, and stale declarations would double-decode.
func findBodyRanges(data []byte) ([][2]int64, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = charsetReader
	var out [][2]int64
	depth := 0
	var start int64 = -1
	for {
		prev := dec.InputOffset()
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if isTruncated(err) && start >= 0 {
			// Truncated inside a body: extend to end of input (tolerant,
			// matching the library's truncated-MOBI behavior).
			out = append(out, [2]int64{start, int64(len(data))})
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth == 2 && t.Name.Local == elBody {
				start = dec.InputOffset()
			}
		case xml.EndElement:
			if depth == 2 && t.Name.Local == elBody && start >= 0 {
				out = append(out, [2]int64{start, prev})
				start = -1
			}
			depth--
		}
	}
	return out, nil
}

// isTruncated reports truncation errors: raw ErrUnexpectedEOF or the
// SyntaxError("unexpected EOF") that encoding/xml returns mid-document.
func isTruncated(err error) bool {
	if err == io.ErrUnexpectedEOF {
		return true
	}
	var syntaxErr *xml.SyntaxError
	return errors.As(err, &syntaxErr) && strings.Contains(syntaxErr.Msg, "unexpected EOF")
}

// buildFb2Resources assembles body-slice content resources, image
// resources, the reading order and the cover from the normalized FB2
// document buffer and parsed metadata. Body slices are zero-copy views into
// data; only decoded image payloads add to the memory footprint.
func buildFb2Resources(data []byte, pm parsedMeta, ranges [][2]int64, maxSize int64) ([]*book.Resource, []book.ReadingOrderItem, *book.Resource, error) {
	bins, err := collectBinaries(data)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read fb2 binaries: %w", err)
	}

	var all []*book.Resource
	readingOrder := make([]book.ReadingOrderItem, 0, len(ranges))
	for i, r := range ranges {
		// The slice shares the document buffer (zero-copy); capturing
		// only the slice keeps nothing else alive.
		name := fmt.Sprintf("body%04d.xml", i+1)
		content := data[r[0]:r[1]]
		linear := i == 0
		res := &book.Resource{
			Id:           strings.TrimSuffix(name, ".xml"),
			Name:         name,
			Href:         name,
			ResolvedHref: name,
			MediaType:    mediatype.FB2Body,
			Size:         int64(len(content)),
			Open: func() (io.ReadCloser, error) {
				if maxSize > 0 && int64(len(content)) > maxSize {
					return nil, book.LimitError(name, int64(len(content)), maxSize)
				}
				return io.NopCloser(bytes.NewReader(content)), nil
			},
		}
		all = append(all, res)
		readingOrder = append(readingOrder, book.ReadingOrderItem{Resource: res, Linear: linear})
	}

	var cover *book.Resource
	for _, res := range imageResources(bins, maxSize) {
		all = append(all, res)
		if cover == nil && res.Id == pm.coverID {
			res.Properties = "cover-image"
			cover = res
		}
	}
	return all, readingOrder, cover, nil
}

// imageResources builds lazily-opened image resources from decoded binaries.
func imageResources(bins []*binaryFile, maxSize int64) []*book.Resource {
	out := make([]*book.Resource, 0, len(bins))
	for _, bf := range bins {
		bf := bf
		out = append(out, &book.Resource{
			Id:           bf.id,
			Name:         path.Base(bf.href),
			Href:         bf.href,
			ResolvedHref: bf.href,
			MediaType:    bf.mediaType,
			Size:         int64(len(bf.data)),
			Open: func() (io.ReadCloser, error) {
				if maxSize > 0 && int64(len(bf.data)) > maxSize {
					return nil, book.LimitError(bf.href, int64(len(bf.data)), maxSize)
				}
				return io.NopCloser(bytes.NewReader(bf.data)), nil
			},
		})
	}
	return out
}
