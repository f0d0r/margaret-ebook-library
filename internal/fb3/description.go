package fb3

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/util"
	xmlx "github.com/f0d0r/margaret-ebook-library/internal/xml"
	"golang.org/x/text/language"
)

// Element and attribute local names (matched namespace-agnostically).
const (
	elFb3Description   = "fb3-description"
	elFb3Body          = "fb3-body"
	elTitle            = "title"
	elMain             = "main"
	elRelations        = "fb3-relations"
	elSubject          = "subject"
	elClassification   = "fb3-classification"
	elFirstName        = "first-name"
	elMiddleName       = "middle-name"
	elLastName         = "last-name"
	elLang             = "lang"
	elAnnotation       = "annotation"
	elP                = "p"
	elBr               = "br"
	elDocumentInfo     = "document-info"
	elPaperPublishInfo = "paper-publish-info"
	elPeriodical       = "periodical"
	elISSN             = "issn"
	elISBN             = "isbn"

	attrLink      = "link"
	attrID        = "id"
	attrDOI       = "doi"
	attrSrcURL    = "src-url"
	attrPublisher = "publisher"
	attrYear      = "year"

	// linkAuthor is the only subject role mapped to authors.
	linkAuthor = "author"
	// linkCoAuthor marks co-authors, likewise mapped to authors.
	linkCoAuthor = "co_author"
)

// parsedMeta is the parsed description metadata and identifier fields.
type parsedMeta struct {
	metadata        book.Metadata
	rootID          string
	documentISBNs   []string
	doi             string
	srcURLs         []string
	paperISBNs      []string
	periodicalISSNs []string
	publisher       string
	publicationDate string
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
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM; UTF-16 handled by the decoder
	return parseDescriptionDoc(xmlx.NewDecoder(bytes.NewReader(data)))
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
		for _, a := range start.Attr {
			if a.Name.Local == attrID {
				pm.rootID = strings.TrimSpace(a.Value)
			}
		}
		if err := parseDescriptionChildren(dec, &pm); err != nil {
			return parsedMeta{}, err
		}
		validateLang(&pm)
		pm.metadata.Subjects = util.DedupeSubjects(pm.metadata.Subjects)
		pm.metadata.Identifiers = fb3Identifiers(pm)
		pm.metadata.Publisher = strings.TrimSpace(pm.publisher)
		pm.metadata.PublicationDate = strings.TrimSpace(pm.publicationDate)
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
			case elClassification:
				subjects, err := parseClassification(dec)
				if err != nil {
					return err
				}
				pm.metadata.Subjects = append(pm.metadata.Subjects, subjects...)
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
			case elDocumentInfo:
				if err := parseDocumentInfo(dec, tok.Attr, pm); err != nil {
					return err
				}
			case elPaperPublishInfo:
				if err := parsePaperPublishInfo(dec, tok.Attr, pm); err != nil {
					return err
				}
			case elPeriodical:
				if err := parsePeriodical(dec, pm); err != nil {
					return err
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

// parseDocumentInfo collects the document ISBNs, DOI and source URLs.
func parseDocumentInfo(dec *xml.Decoder, attrs []xml.Attr, pm *parsedMeta) error {
	for _, a := range attrs {
		switch a.Name.Local {
		case attrDOI:
			if pm.doi == "" {
				pm.doi = strings.TrimSpace(a.Value)
			}
		case attrSrcURL:
			if value := strings.TrimSpace(a.Value); value != "" {
				pm.srcURLs = append(pm.srcURLs, value)
			}
		}
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			if tok.Name.Local != elISBN {
				if err := skipElement(dec); err != nil {
					return err
				}
				continue
			}
			text, err := textOf(dec)
			if err != nil {
				return err
			}
			if text != "" {
				pm.documentISBNs = append(pm.documentISBNs, text)
			}
		case xml.EndElement:
			if tok.Name.Local == elDocumentInfo {
				return nil
			}
		}
	}
}

// parsePaperPublishInfo collects the publisher and year from paper-publish-info attributes.
func parsePaperPublishInfo(dec *xml.Decoder, attrs []xml.Attr, pm *parsedMeta) error {
	for _, a := range attrs {
		switch a.Name.Local {
		case attrPublisher:
			if pm.publisher == "" {
				pm.publisher = strings.TrimSpace(a.Value)
			}
		case attrYear:
			if pm.publicationDate == "" {
				pm.publicationDate = strings.TrimSpace(a.Value)
			}
		}
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			if tok.Name.Local == elISBN {
				text, err := textOf(dec)
				if err != nil {
					return err
				}
				if text != "" {
					pm.paperISBNs = append(pm.paperISBNs, text)
				}
			} else if err := skipElement(dec); err != nil {
				return err
			}
		case xml.EndElement:
			if tok.Name.Local == elPaperPublishInfo {
				return nil
			}
		}
	}
}

// parsePeriodical collects the ISSN of the source periodical.
func parsePeriodical(dec *xml.Decoder, pm *parsedMeta) error {
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			if tok.Name.Local == elISSN {
				text, err := textOf(dec)
				if err != nil {
					return err
				}
				if text != "" {
					pm.periodicalISSNs = append(pm.periodicalISSNs, text)
				}
			} else if err := skipElement(dec); err != nil {
				return err
			}
		case xml.EndElement:
			if tok.Name.Local == elPeriodical {
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
			if isAuthorRole(link) && author != "" {
				authors = append(authors, author)
			}
		case xml.EndElement:
			if tok.Name.Local == elRelations {
				return authors, nil
			}
		}
	}
}

// parseClassification returns the genre subjects of an fb3-classification
// element in document order. Subject whitespace is collapsed per xs:token.
func parseClassification(dec *xml.Decoder) ([]string, error) {
	var subjects []string
	for {
		tok, err := dec.Token()
		if err != nil {
			return subjects, err
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			if tok.Name.Local != elSubject {
				if err := skipElement(dec); err != nil {
					return subjects, err
				}
				continue
			}
			text, err := textOf(dec)
			if err != nil {
				return subjects, err
			}
			if text = strings.Join(strings.Fields(text), " "); text != "" {
				subjects = append(subjects, text)
			}
		case xml.EndElement:
			if tok.Name.Local == elClassification {
				return subjects, nil
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

// isAuthorRole reports author-like subject roles (authors and co-authors).
func isAuthorRole(link string) bool {
	return link == linkAuthor || link == linkCoAuthor
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

// fb3Identifiers returns the book's own identifiers first, followed by the
// source publication and provenance identifiers. Declarations are not deduped.
func fb3Identifiers(pm parsedMeta) []book.Identifier {
	var out []book.Identifier
	appendID := func(value, explicit string) {
		scheme, id := util.SplitIdentifier(value, explicit)
		if id == "" {
			return
		}
		out = append(out, book.Identifier{Scheme: scheme, Value: id})
	}
	appendID(pm.rootID, "UUID")
	for _, isbn := range pm.documentISBNs {
		appendID(isbn, "ISBN")
	}
	appendID(pm.doi, "DOI")
	for _, isbn := range pm.paperISBNs {
		appendID(isbn, "ISBN")
	}
	for _, issn := range pm.periodicalISSNs {
		appendID(issn, "ISSN")
	}
	for _, srcURL := range pm.srcURLs {
		appendID(srcURL, "URL")
	}
	return out
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
