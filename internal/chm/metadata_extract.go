package chm

import (
	"strings"

	"golang.org/x/net/html"

	"github.com/f0d0r/margaret-ebook-library/tools"
)

// Meta keys checked for declared publisher, date and ISBN values, in
// priority order. Both ':' and '.' forms are accepted because help
// generators emit Dublin Core metadata either way.
var (
	publisherMetaKeys = []string{
		"publisher", "dc.publisher", "dc:publisher",
		"dcterms.publisher", "dcterms:publisher", "imprint",
	}
	dateMetaKeys = []string{
		"date", "dc.date", "dc:date", "dcterms.date", "dcterms:date",
		"dcterms.issued", "dcterms:issued",
	}
	isbnMetaKeys = []string{
		"isbn", "dc.identifier", "dc:identifier",
		"dcterms.identifier", "dcterms:identifier", "identifier",
	}
)

// publisherFromDoc returns a declared publisher when the default topic
// declares one via meta tags; otherwise it falls back to the same span and
// table heuristics calibre uses.
func publisherFromDoc(doc *html.Node, tags metaTags) string {
	if value := tags.first(publisherMetaKeys...); value != "" {
		return cleanPublisher(value)
	}
	if value := classText(doc, "imprint"); value != "" {
		return cleanPublisher(value)
	}
	return cleanPublisher(tableValue(doc, "publisher"))
}

// declaredDate returns an explicitly declared publication date. Heuristic
// "pub date" text is deliberately not treated as metadata.
func declaredDate(tags metaTags) string {
	return tags.first(dateMetaKeys...)
}

// isbnFromDoc validates declared and heuristically extracted ISBN candidates
// and returns the first checksum-valid value with any scheme label removed.
// Invalid candidates are skipped so a junk span or table cell cannot shadow a
// valid value elsewhere.
func isbnFromDoc(doc *html.Node, tags metaTags) string {
	for _, candidate := range tags.all(isbnMetaKeys...) {
		if value, ok := cleanISBN(candidate); ok {
			return value
		}
	}
	for _, candidate := range []string{classText(doc, "isbn"), tableValue(doc, "isbn")} {
		if value, ok := cleanISBN(candidate); ok {
			return value
		}
	}
	return ""
}

// metaTags maps a lowercased meta name/property to its contents in document
// order.
type metaTags map[string][]string

func collectMetaTags(doc *html.Node) metaTags {
	tags := metaTags{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "meta" {
			var key, content string
			for _, a := range n.Attr {
				switch strings.ToLower(a.Key) {
				case "name", "property":
					key = strings.ToLower(strings.TrimSpace(a.Val))
				case "content":
					content = a.Val
				}
			}
			if key != "" {
				tags[key] = append(tags[key], content)
			}
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	if doc != nil {
		walk(doc)
	}
	return tags
}

// first returns the first non-empty value for keys in the given priority
// order; within one key, document order is preserved.
func (m metaTags) first(keys ...string) string {
	for _, key := range keys {
		for _, value := range m[strings.ToLower(key)] {
			if value = strings.TrimSpace(value); value != "" {
				return value
			}
		}
	}
	return ""
}

// all returns every non-empty value for keys in priority order, preserving
// document order within each key.
func (m metaTags) all(keys ...string) []string {
	var out []string
	for _, key := range keys {
		for _, value := range m[strings.ToLower(key)] {
			if value = strings.TrimSpace(value); value != "" {
				out = append(out, value)
			}
		}
	}
	return out
}

// classText returns the text of the first span whose class attribute contains
// class, case-insensitively. Like calibre, the match is a substring so e.g.
// "isbn13" also matches "isbn".
func classText(doc *html.Node, class string) string {
	needle := strings.ToLower(class)
	var found string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "span" {
			for _, a := range n.Attr {
				if strings.ToLower(a.Key) == "class" && strings.Contains(strings.ToLower(a.Val), needle) {
					found = strings.TrimSpace(textOf(n))
					return
				}
			}
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	if doc != nil {
		walk(doc)
	}
	return found
}

// tableValue returns the value cell belonging to the first table cell whose
// own text contains label. A value in the next cell wins; otherwise the label
// is stripped from the row text (the "Publisher: value" form).
func tableValue(doc *html.Node, label string) string {
	needle := strings.ToLower(label)
	var found string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "td" &&
			strings.Contains(strings.ToLower(cellText(n)), needle) {
			if value := rowValue(n, needle); value != "" {
				found = value
				return
			}
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	if doc != nil {
		walk(doc)
	}
	return found
}

// cellText returns the cell's own text, matching BeautifulSoup's .string
// semantics: a cell with element children has no single string.
func cellText(n *html.Node) string {
	var sb strings.Builder
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		switch ch.Type {
		case html.ElementNode:
			return ""
		case html.TextNode:
			sb.WriteString(ch.Data)
		}
	}
	return strings.TrimSpace(sb.String())
}

// rowValue returns the next cell's text, or the row text with the leading
// label (and optional colon) removed.
func rowValue(cell *html.Node, label string) string {
	for sib := cell.NextSibling; sib != nil; sib = sib.NextSibling {
		if sib.Type != html.ElementNode {
			continue
		}
		if sib.Data == "td" {
			return cleanCellValue(textOf(sib))
		}
		break
	}
	row := cell.Parent
	if row == nil {
		return ""
	}
	return cleanCellValue(stripLabel(textOf(row), label))
}

func cleanCellValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, ":")
	return strings.TrimSpace(value)
}

// stripLabel removes a leading label and optional colon from row text.
func stripLabel(text, label string) string {
	text = strings.TrimSpace(text)
	lower := strings.ToLower(text)
	if !strings.HasPrefix(lower, label) {
		return text
	}
	return strings.TrimSpace(text[len(label):])
}

// cleanISBN validates raw with tools.ParseISBN and removes a leading
// "urn:isbn:", "ISBN", "ISBN-10" or "ISBN-13" label, preserving the declared
// formatting (hyphens) of the remaining value.
func cleanISBN(raw string) (string, bool) {
	if _, ok := tools.ParseISBN(raw); !ok {
		return "", false
	}
	value := strings.TrimSpace(raw)
	lower := strings.ToLower(value)
	for _, prefix := range []string{
		"urn:isbn:", "isbn-13:", "isbn-10:", "isbn13:", "isbn10:",
		"isbn-13", "isbn-10", "isbn13", "isbn10", "isbn:", "isbn",
	} {
		if strings.HasPrefix(lower, prefix) {
			value = strings.TrimSpace(value[len(prefix):])
			value = strings.TrimPrefix(value, ":")
			value = strings.TrimSpace(value)
			break
		}
	}
	return value, true
}

// cleanPublisher trims a declared value and removes a leading
// "Publisher:"/"Imprint:" label.
func cleanPublisher(value string) string {
	value = strings.TrimSpace(value)
	if i := strings.IndexByte(value, ':'); i > 0 {
		switch strings.ToLower(strings.TrimSpace(value[:i])) {
		case "publisher", "imprint", "kiadó", "kiado":
			return strings.TrimSpace(value[i+1:])
		}
	}
	return value
}
