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
// reported as-is (like EPUB/MOBI report their declared versions): Version()
// is descriptive, the parser itself is namespace-agnostic.
const fb2NSPrefix = "http://www.gribuser.ru/xml/fictionbook/"

// fbVersionSuffix only accepts a dotted numeric version ("2.0", "8.9",
// ...), so lookalikes like ".../2beta" or ".../2.0/" fall through to "".
var fbVersionSuffix = regexp.MustCompile(`^\d+(\.\d+)*$`)

// FictionBook element and attribute local names (matched
// namespace-agnostically, so prefixed and 2.0/2.1/namespace-less variants
// all parse identically).
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

// parsedMeta is the result of parsing an FB2 description: normalized
// metadata, the format version from the root namespace, and the cover image
// binary id referenced by coverpage ("" when absent).
type parsedMeta struct {
	metadata book.Metadata
	version  string
	coverID  string
}

// parseMetadata extracts the description metadata (title, authors,
// description, languages) from raw FB2 bytes following calibre's
// metadata/fb2.py fallback rules. The input is normalized first
// (idempotent, so already-normalized buffers pass through untouched). On XML
// syntax errors it retries once with bare ampersands fixed (calibre's
// `raw.replace('& ', '&amp;')` parity).
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

// declPattern finds the XML encoding declaration in the prolog.
var declPattern = regexp.MustCompile(`(?i)encoding\s*=\s*['"][^'"]*['"]`)

// XML encoding labels (lowercase, compared after normalization).
const (
	encUTF8     = "utf-8"
	encUTF8Alt  = "utf8"
	encASCII    = "us-ascii"
	encASCIIAlt = "ascii"
	encUTF16    = "utf-16"
	encUTF16LE  = "utf-16le"
	encUTF16BE  = "utf-16be"
)

// prologScanLimit bounds prolog scanning (encoding declaration): the XML
// spec requires the declaration first, so 2KB is ample headroom while
// guaranteeing body text (which may literally discuss encodings) is never
// rewritten. declLabel and rewriteDecl must share this limit: if the label
// were found where the rewrite cannot reach, transcoded bytes would keep a
// lying declaration and double-decode downstream.
const prologScanLimit = 2048

// normalizeFB2 converts raw FB2 bytes to UTF-8 for encoding/xml, which only
// accepts UTF-8: UTF-16 (BOM-detected) is transcoded, a declared single-byte
// encoding is transcoded via the x/text index, a UTF-8 BOM is stripped, and
// stray NUL bytes are removed (calibre strips them before parsing). After a
// transcode the declaration is rewritten to UTF-8 so downstream decoders
// never double-decode. The result is idempotent: re-normalizing is a no-op.
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

// declLabel returns the encoding label from the XML prolog (""),
// or "" when absent. Only the head is scanned: the declaration, if present,
// must appear there.
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

// rewriteDecl sets the prolog encoding declaration to UTF-8. Only the head
// is touched so identical strings in the document body are left alone.
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

// canonicalizeFB2 reads the (already size-capped) source and normalizes it
// via normalizeFB2.
func canonicalizeFB2(r io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return normalizeFB2(raw)
}

// charsetReader resolves the XML encoding declaration. UTF-8/16 and ASCII
// are passed through (UTF-16 was already transcoded by toUTF8Reader);
// anything else goes through x/text's encoding index, which covers the
// single-byte encodings common in FB2 files (windows-1251, koi8-r,
// iso-8859-*, ...).
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

// infoSection holds the metadata collected from one description subsection
// (title-info, src-title-info; document-info contributes authors only).
type infoSection struct {
	title      string
	authors    []string
	annotation string
	lang       string
	coverID    string
}

// parseDocument streams the XML until the description is consumed (or the
// first body starts), then applies calibre's fallback rules. Element matching
// uses local names only, so prefixed, 2.0/2.1 and namespace-less documents
// all parse identically.
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

// parseDescriptionChildren consumes the description element, filling the
// first occurrence of each subsection kind.
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

// parseInfoSection consumes one title-info/src-title-info/document-info
// element (decoder positioned after its start tag).
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

// parseAuthor consumes one author element: "first [middle] last", falling
// back to nickname (calibre parity) and finally to the element's direct
// text for non-standard files like <author>Lev Tolstoy</author> (tolerance
// beyond calibre, which ignores such content). Empty authors return "".
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

// parseCoverpage consumes one coverpage element and returns the referenced
// binary id (the coverpage/image href without its leading '#', calibre
// parity), or "" when there is none.
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

// hrefAttr returns the xlink href attribute, falling back to a plain href
// attribute (calibre parity).
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

// parseAnnotation consumes one annotation element, joining block-level
// children (paragraphs, poem lines, cites, table cells, ...) with newlines
// while leaving the text inside blocks character-for-character intact.
// Formatting whitespace between blocks is dropped; empty-line elements yield
// blank lines.
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

// textOf consumes the current element and returns its concatenated text,
// trimmed of surrounding whitespace.
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

// skipElement consumes the rest of the current element (decoder positioned
// after its start tag).
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

// assemble applies calibre's fallback rules across the collected sections:
// title-info wins, then src-title-info (document-info contributes authors
// only). Missing values stay empty, matching the EPUB/MOBI readers.
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

// fbVersion maps the root namespace to the declared format version.
// Anything else yields "" (unknown), matching the "missing values stay
// empty" convention.
func fbVersion(space string) string {
	rest, ok := strings.CutPrefix(space, fb2NSPrefix)
	if !ok || !fbVersionSuffix.MatchString(rest) {
		return ""
	}
	return rest
}
