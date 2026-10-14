package fb3

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/f0d0r/margaret-ebook-library/book"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/language"
	"golang.org/x/text/transform"
)

// Element and attribute local names (matched namespace-agnostically).
const (
	elFb3Description = "fb3-description"
	elFb3Body        = "fb3-body"
	elTitle          = "title"
	elMain           = "main"
	elRelations      = "fb3-relations"
	elSubject        = "subject"
	elFirstName      = "first-name"
	elMiddleName     = "middle-name"
	elLastName       = "last-name"
	elLang           = "lang"
	elAnnotation     = "annotation"
	elP              = "p"
	elBr             = "br"

	attrLink = "link"

	// linkAuthor is the only subject role mapped to authors.
	linkAuthor = "author"
)

// parsedMeta is the parsed description metadata.
type parsedMeta struct {
	metadata book.Metadata
}

// parseDescription extracts metadata from description-part bytes,
// retrying once with bare ampersands fixed.
func parseDescription(data []byte) (parsedMeta, error) {
	pm, err := decodeDescription(data)
	if err == nil {
		return pm, nil
	}
	var syntaxErr *xml.SyntaxError
	if !errors.As(err, &syntaxErr) {
		return parsedMeta{}, err
	}
	return decodeDescription(bytes.ReplaceAll(data, []byte("& "), []byte("&amp; ")))
}

func decodeDescription(data []byte) (parsedMeta, error) {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = charsetReader
	return parseDescriptionDoc(dec)
}

// charsetReader resolves the XML encoding declaration.
func charsetReader(label string, input io.Reader) (io.Reader, error) {
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

// parseDescriptionDoc consumes the fb3-description document.
func parseDescriptionDoc(dec *xml.Decoder) (parsedMeta, error) {
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				return parsedMeta{}, fmt.Errorf("no fb3-description root element")
			}
			return parsedMeta{}, err
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local != elFb3Description {
			return parsedMeta{}, fmt.Errorf("not an fb3-description document: root is %q", start.Name.Local)
		}
		var pm parsedMeta
		if err := parseDescriptionChildren(dec, &pm); err != nil {
			return parsedMeta{}, err
		}
		validateLang(&pm)
		return pm, nil
	}
}

// parseDescriptionChildren collects title, authors, lang and annotation.
func parseDescriptionChildren(dec *xml.Decoder, pm *parsedMeta) error {
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			switch tok.Name.Local {
			case elTitle:
				title, err := parseTitle(dec)
				if err != nil {
					return err
				}
				if pm.metadata.Title == "" {
					pm.metadata.Title = title
				}
			case elRelations:
				authors, err := parseRelations(dec)
				if err != nil {
					return err
				}
				pm.metadata.Authors = append(pm.metadata.Authors, authors...)
			case elLang:
				text, err := textOf(dec)
				if err != nil {
					return err
				}
				if len(pm.metadata.Languages) == 0 && text != "" {
					pm.metadata.Languages = []string{text}
				}
			case elAnnotation:
				text, err := parseAnnotation(dec)
				if err != nil {
					return err
				}
				if pm.metadata.Description == "" {
					pm.metadata.Description = text
				}
			default:
				if err := skipElement(dec); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if tok.Name.Local == elFb3Description {
				return nil
			}
		}
	}
}

// parseTitle returns the main text of a title element.
func parseTitle(dec *xml.Decoder) (string, error) {
	var main string
	for {
		tok, err := dec.Token()
		if err != nil {
			return main, err
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			if tok.Name.Local == elMain && main == "" {
				text, err := textOf(dec)
				if err != nil {
					return main, err
				}
				main = text
			} else if err := skipElement(dec); err != nil {
				return main, err
			}
		case xml.EndElement:
			if tok.Name.Local == elTitle {
				return main, nil
			}
		}
	}
}

// parseRelations returns author subjects in document order.
func parseRelations(dec *xml.Decoder) ([]string, error) {
	var authors []string
	for {
		tok, err := dec.Token()
		if err != nil {
			return authors, err
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			if tok.Name.Local != elSubject {
				if err := skipElement(dec); err != nil {
					return authors, err
				}
				continue
			}
			link := subjectLink(tok.Attr)
			author, err := parseSubject(dec)
			if err != nil {
				return authors, err
			}
			if link == linkAuthor && author != "" {
				authors = append(authors, author)
			}
		case xml.EndElement:
			if tok.Name.Local == elRelations {
				return authors, nil
			}
		}
	}
}

func subjectLink(attrs []xml.Attr) string {
	for _, a := range attrs {
		if a.Name.Local == attrLink {
			return strings.TrimSpace(a.Value)
		}
	}
	return ""
}

// parseSubject formats one subject as "first [middle] last",
// falling back to its title.
func parseSubject(dec *xml.Decoder) (string, error) {
	var first, middle, last, title string
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", err
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			switch tok.Name.Local {
			case elFirstName, elMiddleName, elLastName:
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
				}
			case elTitle:
				text, err := parseTitle(dec)
				if err != nil {
					return "", err
				}
				if title == "" {
					title = text
				}
			default:
				if err := skipElement(dec); err != nil {
					return "", err
				}
			}
		case xml.EndElement:
			if tok.Name.Local == elSubject {
				parts := make([]string, 0, 3)
				for _, p := range []string{first, middle, last} {
					if p != "" {
						parts = append(parts, p)
					}
				}
				if len(parts) > 0 {
					return strings.Join(parts, " "), nil
				}
				return strings.TrimSpace(title), nil
			}
		}
	}
}

// parseAnnotation joins annotation paragraphs with newlines;
// br yields blank lines.
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
			if tok.Name.Local == elBr {
				flush()
				lines = append(lines, "")
			}
		case xml.EndElement:
			switch tok.Name.Local {
			case elAnnotation:
				flush()
				return strings.Join(lines, "\n"), nil
			case elP:
				flush()
			}
		}
	}
}

// validateLang keeps the language only if it parses.
func validateLang(pm *parsedMeta) {
	if len(pm.metadata.Languages) == 0 {
		return
	}
	lang := strings.TrimSpace(pm.metadata.Languages[0])
	pm.metadata.Languages = nil
	if lang == "" {
		return
	}
	tag, err := language.Parse(lang)
	if err != nil || tag.String() == "und" {
		return
	}
	if _, confidence := tag.Base(); confidence == language.No {
		return
	}
	pm.metadata.Languages = []string{lang}
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
