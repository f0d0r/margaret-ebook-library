package fb2

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/f0d0r/margaret-ebook-library/book"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/language"
	"golang.org/x/text/transform"
)

// FictionBook namespace family prefix. The declared version suffix is
// reported as-is; the parser itself is namespace-agnostic.
const fb2NSPrefix = "http://www.gribuser.ru/xml/fictionbook/"

// fbVersionSuffix accepts dotted numeric versions only; lookalikes fall through to "".
var fbVersionSuffix = regexp.MustCompile(`^\d+(\.\d+)*$`)

// FictionBook element and attribute local names (matched namespace-agnostically).
const (
	elFictionBook  = "FictionBook"
	elDescription  = "description"
	elTitleInfo    = "title-info"
	elSrcTitleInfo = "src-title-info"
	elDocumentInfo = "document-info"
	elBookTitle    = "book-title"
	elLang         = "lang"
	elAuthor       = "author"
	elAnnotation   = "annotation"
	elCoverpage    = "coverpage"
	elFirstName    = "first-name"
	elMiddleName   = "middle-name"
	elLastName     = "last-name"
	elNickname     = "nickname"
	elImage        = "image"
	elBody         = "body"
	elBinary       = "binary"
	elP            = "p"
	elV            = "v"
	elSubtitle     = "subtitle"
	elTextAuthor   = "text-author"
	elTh           = "th"
	elTd           = "td"
	elEmptyLine    = "empty-line"

	attrID          = "id"
	attrContentType = "content-type"
	attrHref        = "href"
	xlinkNS         = "http://www.w3.org/1999/xlink"
)

// parsedMeta is the parsed description: metadata, version and cover binary id.
type parsedMeta struct {
	metadata book.Metadata
	version  string
	coverID  string
}

// parseMetadata extracts metadata from raw FB2 bytes, normalizing first,
// retrying once with bare ampersands fixed.
func parseMetadata(data []byte) (parsedMeta, error) {
	norm, err := normalizeFB2(data)
	if err != nil {
		return parsedMeta{}, err
	}
	pm, err := parseDoc(norm)
	if err == nil {
		return pm, nil
	}
	var syntaxErr *xml.SyntaxError
	if !errors.As(err, &syntaxErr) {
		return parsedMeta{}, err
	}
	fixed := bytes.ReplaceAll(norm, []byte("& "), []byte("&amp; "))
	return parseDoc(fixed)
}

func parseDoc(data []byte) (parsedMeta, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = charsetReader
	return parseDocument(dec)
}

var declPattern = regexp.MustCompile(`(?i)encoding\s*=\s*['"][^'"]*['"]`)

const (
	encUTF8     = "utf-8"
	encUTF8Alt  = "utf8"
	encASCII    = "us-ascii"
	encASCIIAlt = "ascii"
	encUTF16    = "utf-16"
	encUTF16LE  = "utf-16le"
	encUTF16BE  = "utf-16be"
)

// prologScanLimit bounds prolog scanning. declLabel and rewriteDecl must
// share this limit, or transcoded bytes would keep a lying declaration.
const prologScanLimit = 2048

// normalizeFB2 converts raw FB2 bytes to UTF-8 (transcoding UTF-16 and
// declared single-byte encodings, stripping BOM and NUL bytes). Idempotent.
func normalizeFB2(raw []byte) ([]byte, error) {
	if len(raw) >= 2 && raw[0] == 0xFF && raw[1] == 0xFE {
		if len(raw) >= 4 && raw[2] == 0x00 && raw[3] == 0x00 {
			return nil, fmt.Errorf("unsupported xml encoding: utf-32")
		}
		out, _, err := transform.Bytes(unicode.UTF16(unicode.LittleEndian, unicode.ExpectBOM).NewDecoder(), raw)
		if err != nil {
			return nil, err
		}
		return rewriteDecl(out), nil
	}
	if len(raw) >= 2 && raw[0] == 0xFE && raw[1] == 0xFF {
		out, _, err := transform.Bytes(unicode.UTF16(unicode.BigEndian, unicode.ExpectBOM).NewDecoder(), raw)
		if err != nil {
			return nil, err
		}
		return rewriteDecl(out), nil
	}
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	if bytes.IndexByte(raw, 0x00) >= 0 {
		stripped := make([]byte, 0, len(raw))
		for _, c := range raw {
			if c != 0x00 {
				stripped = append(stripped, c)
			}
		}
		raw = stripped
	}
	label := declLabel(raw)
	switch strings.ToLower(label) {
	case "", encUTF8, encUTF8Alt, encASCII, encASCIIAlt:
		return raw, nil
	default:
		enc, err := htmlindex.Get(label)
		if err != nil {
			return nil, fmt.Errorf("unsupported xml encoding %q", label)
		}
		out, _, err := transform.Bytes(enc.NewDecoder(), raw)
		if err != nil {
			return nil, err
		}
		return rewriteDecl(out), nil
	}
}

// declLabel returns the encoding label from the XML prolog, or "".
func declLabel(data []byte) string {
	head := data[:min(prologScanLimit, len(data))]
	m := declPattern.Find(head)
	if m == nil {
		return ""
	}
	s := string(m)
	if i := strings.IndexAny(s, `"'`); i >= 0 {
		s = s[i+1:]
	}
	return strings.TrimRight(s, `"'`)
}

// rewriteDecl sets the prolog encoding declaration to UTF-8.
func rewriteDecl(data []byte) []byte {
	headLen := min(prologScanLimit, len(data))
	loc := declPattern.FindIndex(data[:headLen])
	if loc == nil {
		return data
	}
	out := make([]byte, 0, len(data))
	out = append(out, data[:loc[0]]...)
	out = append(out, `encoding="UTF-8"`...)
	return append(out, data[loc[1]:]...)
}

func canonicalizeFB2(r io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return normalizeFB2(raw)
}

// charsetReader resolves the XML encoding declaration via x/text,
// passing UTF-8/16 and ASCII through.
func charsetReader(label string, input io.Reader) (io.Reader, error) {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "", encUTF8, encUTF8Alt, encASCII, encASCIIAlt:
		return input, nil
	case encUTF16, encUTF16LE, encUTF16BE:
		return input, nil
	default:
		enc, err := htmlindex.Get(label)
		if err != nil {
			return nil, fmt.Errorf("unsupported xml encoding %q", label)
		}
		return transform.NewReader(input, enc.NewDecoder()), nil
	}
}

// infoSection holds metadata collected from one description subsection.
type infoSection struct {
	title      string
	authors    []string
	annotation string
	lang       string
	coverID    string
}

// parseDocument streams the XML until the description is consumed,
// then applies the section fallback rules.
func parseDocument(dec *xml.Decoder) (parsedMeta, error) {
	var titleInfo, srcTitleInfo, docInfo infoSection
	var version string
	seenRoot := false

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return parsedMeta{}, err
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if !seenRoot {
			seenRoot = true
			if start.Name.Local != elFictionBook {
				return parsedMeta{}, fmt.Errorf("not a FictionBook document: root is %q", start.Name.Local)
			}
			version = fbVersion(start.Name.Space)
			continue
		}
		switch start.Name.Local {
		case elDescription:
			if err := parseDescriptionChildren(dec, &titleInfo, &srcTitleInfo, &docInfo); err != nil {
				return parsedMeta{}, err
			}
			return assemble(titleInfo, srcTitleInfo, docInfo, version), nil
		case elBody, elBinary:
			// No description before content: nothing to collect.
			return assemble(titleInfo, srcTitleInfo, docInfo, version), nil
		default:
			if err := skipElement(dec); err != nil {
				return parsedMeta{}, err
			}
		}
	}
	if !seenRoot {
		return parsedMeta{}, fmt.Errorf("no FictionBook root element")
	}
	return assemble(titleInfo, srcTitleInfo, docInfo, version), nil
}

// parseDescriptionChildren fills the first occurrence of each subsection.
func parseDescriptionChildren(dec *xml.Decoder, titleInfo, srcTitleInfo, docInfo *infoSection) error {
	seen := map[string]bool{}
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			if seen[tok.Name.Local] {
				if err := skipElement(dec); err != nil {
					return err
				}
				continue
			}
			seen[tok.Name.Local] = true
			var dst *infoSection
			switch tok.Name.Local {
			case elTitleInfo:
				dst = titleInfo
			case elSrcTitleInfo:
				dst = srcTitleInfo
			case elDocumentInfo:
				dst = docInfo
			default:
				if err := skipElement(dec); err != nil {
					return err
				}
				continue
			}
			if err := parseInfoSection(dec, tok.Name.Local, dst); err != nil {
				return err
			}
		case xml.EndElement:
			if tok.Name.Local == elDescription {
				return nil
			}
		}
	}
}

// parseInfoSection consumes one info subsection.
func parseInfoSection(dec *xml.Decoder, name string, dst *infoSection) error {
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			switch tok.Name.Local {
			case elBookTitle:
				text, err := textOf(dec)
				if err != nil {
					return err
				}
				if dst.title == "" {
					dst.title = text
				}
			case elLang:
				text, err := textOf(dec)
				if err != nil {
					return err
				}
				if dst.lang == "" {
					dst.lang = text
				}
			case elAuthor:
				author, err := parseAuthor(dec)
				if err != nil {
					return err
				}
				if author != "" {
					dst.authors = append(dst.authors, author)
				}
			case elAnnotation:
				annotation, err := parseAnnotation(dec)
				if err != nil {
					return err
				}
				if dst.annotation == "" {
					dst.annotation = annotation
				}
			case elCoverpage:
				coverID, err := parseCoverpage(dec)
				if err != nil {
					return err
				}
				if dst.coverID == "" {
					dst.coverID = coverID
				}
			default:
				if err := skipElement(dec); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if tok.Name.Local == name {
				return nil
			}
		}
	}
}

// parseAuthor formats one author as "first [middle] last", falling back
// to nickname, then to direct text. Empty authors return "".
func parseAuthor(dec *xml.Decoder) (string, error) {
	var first, middle, last, nickname string
	var direct strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", err
		}
		switch tok := tok.(type) {
		case xml.CharData:
			direct.WriteString(string(tok))
		case xml.StartElement:
			text, err := textOf(dec)
			if err != nil {
				return "", err
			}
			switch tok.Name.Local {
			case elFirstName:
				if first == "" {
					first = text
				}
			case elMiddleName:
				if middle == "" {
					middle = text
				}
			case elLastName:
				if last == "" {
					last = text
				}
			case elNickname:
				if nickname == "" {
					nickname = text
				}
			}
		case xml.EndElement:
			if tok.Name.Local == elAuthor {
				parts := make([]string, 0, 3)
				for _, p := range []string{first, middle, last} {
					if p != "" {
						parts = append(parts, p)
					}
				}
				if len(parts) > 0 {
					return strings.Join(parts, " "), nil
				}
				if nickname != "" {
					return nickname, nil
				}
				return strings.TrimSpace(direct.String()), nil
			}
		}
	}
}

// parseCoverpage returns the referenced binary id (href without '#'), or "".
func parseCoverpage(dec *xml.Decoder) (string, error) {
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", err
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			if tok.Name.Local == elImage {
				href := hrefAttr(tok.Attr)
				if err := skipElement(dec); err != nil {
					return "", err
				}
				return strings.TrimPrefix(href, "#"), nil
			}
			if err := skipElement(dec); err != nil {
				return "", err
			}
		case xml.EndElement:
			if tok.Name.Local == elCoverpage {
				return "", nil
			}
		}
	}
}

// hrefAttr returns the href attribute, preferring xlink.
func hrefAttr(attrs []xml.Attr) string {
	for _, a := range attrs {
		if a.Name.Space == xlinkNS && a.Name.Local == attrHref {
			return a.Value
		}
	}
	for _, a := range attrs {
		if a.Name.Local == attrHref {
			return a.Value
		}
	}
	return ""
}

// blockEndFlush lists elements whose end starts a new description line.
func blockEndFlush(local string) bool {
	switch local {
	case elP, elV, elSubtitle, elTextAuthor, elTh, elTd:
		return true
	default:
		return false
	}
}

// parseAnnotation joins annotation blocks with newlines; empty-line
// elements yield blank lines.
func parseAnnotation(dec *xml.Decoder) (string, error) {
	var lines []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			lines = append(lines, cur.String())
			cur.Reset()
		}
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			flush()
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return strings.Join(lines, "\n"), err
		}
		switch tok := tok.(type) {
		case xml.CharData:
			s := string(tok)
			if cur.Len() == 0 && strings.TrimSpace(s) == "" {
				continue
			}
			cur.WriteString(s)
		case xml.StartElement:
			if tok.Name.Local == elEmptyLine {
				flush()
				lines = append(lines, "")
			}
		case xml.EndElement:
			switch {
			case tok.Name.Local == elAnnotation:
				flush()
				return strings.Join(lines, "\n"), nil
			case blockEndFlush(tok.Name.Local):
				flush()
			}
		}
	}
}

// textOf returns an element's concatenated text, trimmed.
func textOf(dec *xml.Decoder) (string, error) {
	var sb strings.Builder
	depth := 1
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return strings.TrimSpace(sb.String()), err
		}
		switch tok := tok.(type) {
		case xml.CharData:
			sb.WriteString(string(tok))
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}
	return strings.TrimSpace(sb.String()), nil
}

// skipElement consumes the rest of the current element.
func skipElement(dec *xml.Decoder) error {
	depth := 1
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}
	return nil
}

// assemble merges sections: title-info wins, then src-title-info;
// authors may come from document-info. Missing values stay empty.
func assemble(titleInfo, srcTitleInfo, docInfo infoSection, version string) parsedMeta {
	pm := parsedMeta{version: version}
	for _, s := range []infoSection{titleInfo, srcTitleInfo} {
		if pm.metadata.Title == "" {
			pm.metadata.Title = s.title
		}
		if pm.metadata.Description == "" {
			pm.metadata.Description = s.annotation
		}
		if pm.coverID == "" {
			pm.coverID = s.coverID
		}
	}
	for _, s := range []infoSection{titleInfo, srcTitleInfo, docInfo} {
		if len(s.authors) > 0 {
			pm.metadata.Authors = s.authors
			break
		}
	}
	if lang := strings.TrimSpace(titleInfo.lang); lang != "" {
		if tag, err := language.Parse(lang); err == nil && tag.String() != "und" {
			if _, confidence := tag.Base(); confidence != language.No {
				pm.metadata.Languages = []string{lang}
			}
		}
	}
	return pm
}

// fbVersion maps the root namespace to the declared version, else "".
func fbVersion(space string) string {
	rest, ok := strings.CutPrefix(space, fb2NSPrefix)
	if !ok || !fbVersionSuffix.MatchString(rest) {
		return ""
	}
	return rest
}
