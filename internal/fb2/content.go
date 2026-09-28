package fb2

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"strings"
)

// Internal link targets are emitted as markers during the single streaming
// transform pass (forward references are not yet known then) and resolved to
// real hrefs once every document — and therefore every anchor id — is known.
const (
	linkMarkerStart = "\x00fb2link:"
	linkMarkerEnd   = "\x00"
)

// htmlDoc is one transformed content document: a top-level section (or a
// body header without sections) rendered as XHTML.
type htmlDoc struct {
	name   string // e.g. section0001.html
	linear bool   // false for notes bodies (cf. EPUB spine linear="no")
	inner  string // body HTML with unresolved link markers
	html   string // final envelope, markers resolved
	ids    map[string]bool
}

// buildContentDocs transforms every body of the document into per-top-level-
// section XHTML documents in document order, numbered continuously
// (section0001.html, ...). The first body is linear; further bodies (notes)
// are not. Body-level headers (title, epigraph, ...) attach to the body's
// first section document.
func buildContentDocs(data []byte, title string, images map[string]string) ([]htmlDoc, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = charsetReader
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, errNoRoot(err)
		}
		if start, ok := tok.(xml.StartElement); ok {
			if start.Name.Local != "FictionBook" {
				return nil, fmt.Errorf("not a FictionBook document: root is %q", start.Name.Local)
			}
			break
		}
	}

	b := &docBuilder{title: title, images: images}
	bodyIdx := -1
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local != "body" {
			if err := skipElement(dec); err != nil {
				return nil, err
			}
			continue
		}
		bodyIdx++
		if err := b.buildBody(dec, bodyIdx == 0); err != nil {
			return nil, err
		}
	}
	b.resolveLinks()
	return b.docs, nil
}

func errNoRoot(err error) error {
	if err == io.EOF {
		return fmt.Errorf("no FictionBook root element")
	}
	return err
}

// docBuilder accumulates transformed documents across bodies so section
// numbering stays continuous.
type docBuilder struct {
	title  string
	images map[string]string
	docs   []htmlDoc
}

func (b *docBuilder) addDoc(inner string, ids []string, linear bool) {
	name := fmt.Sprintf("section%04d.html", len(b.docs)+1)
	idSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}
	b.docs = append(b.docs, htmlDoc{name: name, linear: linear, inner: inner, ids: idSet})
}

// buildBody consumes one body element (decoder positioned after its start
// tag), splitting top-level sections into documents.
func (b *docBuilder) buildBody(dec *xml.Decoder, linear bool) error {
	bodyStart := len(b.docs)
	var header strings.Builder
	var headerIDs []string
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			if tok.Name.Local == "section" {
				secHTML, secIDs, err := b.transformSection(dec, tok, 1)
				if err != nil {
					return err
				}
				b.addDoc(header.String()+secHTML, append(headerIDs, secIDs...), linear)
				header.Reset()
				headerIDs = nil
			} else {
				h, ids, err := b.transformElement(dec, tok, 0)
				if err != nil {
					return err
				}
				header.WriteString(h)
				headerIDs = append(headerIDs, ids...)
			}
		case xml.EndElement:
			if tok.Name.Local == "body" {
				if header.Len() > 0 {
					if len(b.docs) > bodyStart {
						last := &b.docs[len(b.docs)-1]
						last.inner += header.String()
						for _, id := range headerIDs {
							last.ids[id] = true
						}
					} else {
						b.addDoc(header.String(), headerIDs, linear)
					}
				}
				return nil
			}
		}
	}
}

// transformSection consumes one section element (decoder positioned after
// its start tag). depth is the section nesting level (top-level = 1) and
// drives title heading levels.
func (b *docBuilder) transformSection(dec *xml.Decoder, start xml.StartElement, depth int) (string, []string, error) {
	var sb strings.Builder
	var ids []string
	if id := attrValue(start.Attr, "id"); id != "" {
		ids = append(ids, id)
		sb.WriteString(`<section id="`)
		sb.WriteString(html.EscapeString(id))
		sb.WriteString(`">`)
	} else {
		sb.WriteString("<section>")
	}
	inner, innerIDs, err := b.transformChildren(dec, "section", depth)
	if err != nil {
		return "", nil, err
	}
	sb.WriteString(inner)
	sb.WriteString("</section>")
	return sb.String(), append(ids, innerIDs...), nil
}

// transformElement consumes one element (decoder positioned after its start
// tag) and returns its HTML plus any anchor ids it defines. Unknown elements
// are transparent: the tag is dropped but children are kept, so no text is
// ever lost.
func (b *docBuilder) transformElement(dec *xml.Decoder, start xml.StartElement, secDepth int) (string, []string, error) {
	local := start.Name.Local
	id := attrValue(start.Attr, "id")

	switch local {
	case "section":
		return b.transformSection(dec, start, secDepth+1)
	case "title":
		return b.wrapped(dec, "title", openTag("h"+headingLevel(secDepth), id), secDepth)
	case "subtitle":
		return b.wrapped(dec, "subtitle", openTag("h"+headingLevel(secDepth+1), id), secDepth)
	case "p", "v":
		return b.wrapped(dec, local, openTag("p", id), secDepth)
	case "poem":
		return b.wrapped(dec, "poem", openTag("div class=\"poem\"", id), secDepth)
	case "stanza":
		return b.wrapped(dec, "stanza", openTag("div class=\"stanza\"", id), secDepth)
	case "cite":
		return b.wrapped(dec, "cite", openTag("blockquote", id), secDepth)
	case "epigraph":
		return b.wrapped(dec, "epigraph", openTag("div class=\"epigraph\"", id), secDepth)
	case "text-author":
		return b.wrapped(dec, "text-author", openTag("p class=\"text-author\"", id), secDepth)
	case "table":
		return b.wrapped(dec, "table", openTag("table", id), secDepth)
	case "tr":
		return b.wrapped(dec, "tr", openTag("tr", id), secDepth)
	case "th":
		return b.wrapped(dec, "th", openTag("th", id), secDepth)
	case "td":
		return b.wrapped(dec, "td", openTag("td", id), secDepth)
	case "date":
		return b.wrapped(dec, "date", openTag("p class=\"date\"", id), secDepth)
	case "annotation":
		return b.wrapped(dec, "annotation", openTag("div class=\"annotation\"", id), secDepth)
	case "empty-line":
		if err := skipElement(dec); err != nil {
			return "", nil, err
		}
		return `<p class="empty-line"><br/></p>`, nil, nil
	case "strong":
		return b.wrapped(dec, "strong", openTag("strong", id), secDepth)
	case "emphasis":
		return b.wrapped(dec, "emphasis", openTag("em", id), secDepth)
	case "strikethrough":
		return b.wrapped(dec, "strikethrough", openTag("s", id), secDepth)
	case "sub":
		return b.wrapped(dec, "sub", openTag("sub", id), secDepth)
	case "sup":
		return b.wrapped(dec, "sup", openTag("sup", id), secDepth)
	case "code":
		return b.wrapped(dec, "code", openTag("code", id), secDepth)
	case "style":
		return b.wrapped(dec, "style", openTag("span", id), secDepth)
	case "a":
		href := hrefAttr(start.Attr)
		inner, ids, err := b.transformChildren(dec, "a", secDepth)
		if err != nil {
			return "", nil, err
		}
		if href == "" {
			return b.anchored(inner, ids, id)
		}
		if strings.HasPrefix(href, "#") && len(href) > 1 {
			return `<a href="` + linkMarkerStart + href[1:] + linkMarkerEnd + `">` + inner + `</a>`, ids, nil
		}
		return `<a href="` + html.EscapeString(href) + `">` + inner + `</a>`, ids, nil
	case "image":
		if err := skipElement(dec); err != nil {
			return "", nil, err
		}
		return b.transformImage(start), nil, nil
	default:
		inner, ids, err := b.transformChildren(dec, local, secDepth)
		if err != nil {
			return "", nil, err
		}
		return b.anchored(inner, ids, id)
	}
}

// wrapped transforms an element's children and wraps them in the given open
// tag (e.g. `p id="x"` without brackets).
func (b *docBuilder) wrapped(dec *xml.Decoder, local, open string, secDepth int) (string, []string, error) {
	tag, _, _ := strings.Cut(open, " ")
	inner, ids, err := b.transformChildren(dec, local, secDepth)
	if err != nil {
		return "", nil, err
	}
	return "<" + open + ">" + inner + "</" + tag + ">", ids, nil
}

// anchored records an anchor id on otherwise transparent content by emitting
// an empty span when there is no wrapping element to carry it.
func (b *docBuilder) anchored(inner string, ids []string, id string) (string, []string, error) {
	if id == "" {
		return inner, ids, nil
	}
	return `<span id="` + html.EscapeString(id) + `"></span>` + inner, append(ids, id), nil
}

// transformImage renders an image element. Known binary ids resolve to the
// image resource href; unknown refs keep their original value (calibre
// parity) so the loss is visible instead of silent.
func (b *docBuilder) transformImage(start xml.StartElement) string {
	href := hrefAttr(start.Attr)
	src := href
	if strings.HasPrefix(href, "#") {
		if resolved, ok := b.images[href[1:]]; ok {
			src = resolved
		}
	}
	var sb strings.Builder
	sb.WriteString(`<img src="`)
	sb.WriteString(html.EscapeString(src))
	sb.WriteString(`"`)
	if alt := attrValue(start.Attr, "alt"); alt != "" {
		sb.WriteString(` alt="`)
		sb.WriteString(html.EscapeString(alt))
		sb.WriteString(`"`)
	}
	sb.WriteString(`/>`)
	return sb.String()
}

// transformChildren consumes child tokens until the parent's end tag,
// returning the transformed HTML plus anchor ids. Character data passes
// through verbatim (escaped), preserving the stored text exactly.
func (b *docBuilder) transformChildren(dec *xml.Decoder, endLocal string, secDepth int) (string, []string, error) {
	var sb strings.Builder
	var ids []string
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", nil, err
		}
		switch tok := tok.(type) {
		case xml.CharData:
			sb.WriteString(html.EscapeString(string(tok)))
		case xml.StartElement:
			h, childIDs, err := b.transformElement(dec, tok, secDepth)
			if err != nil {
				return "", nil, err
			}
			sb.WriteString(h)
			ids = append(ids, childIDs...)
		case xml.EndElement:
			if tok.Name.Local == endLocal {
				return sb.String(), ids, nil
			}
			return "", nil, fmt.Errorf("mismatched end tag %q, want %q", tok.Name.Local, endLocal)
		}
	}
}

// resolveLinks replaces link markers with real hrefs (same-document targets
// stay fragment-only) and wraps every document in its XHTML envelope.
func (b *docBuilder) resolveLinks() {
	idDoc := make(map[string]int)
	for i := range b.docs {
		for id := range b.docs[i].ids {
			if _, ok := idDoc[id]; !ok {
				idDoc[id] = i
			}
		}
	}
	for i := range b.docs {
		inner := resolveMarkers(b.docs[i].inner, i, idDoc, b.docs)
		var sb strings.Builder
		sb.WriteString(`<html xmlns="http://www.w3.org/1999/xhtml"><head><title>`)
		sb.WriteString(html.EscapeString(b.title))
		sb.WriteString(`</title></head><body>`)
		sb.WriteString(inner)
		sb.WriteString(`</body></html>`)
		b.docs[i].html = sb.String()
	}
}

func resolveMarkers(inner string, docIdx int, idDoc map[string]int, docs []htmlDoc) string {
	var sb strings.Builder
	rest := inner
	for {
		start := strings.Index(rest, linkMarkerStart)
		if start < 0 {
			sb.WriteString(rest)
			return sb.String()
		}
		sb.WriteString(rest[:start])
		rest = rest[start+len(linkMarkerStart):]
		end := strings.Index(rest, linkMarkerEnd)
		if end < 0 {
			sb.WriteString(linkMarkerStart)
			sb.WriteString(rest)
			return sb.String()
		}
		id := rest[:end]
		rest = rest[end+len(linkMarkerEnd):]
		if target, ok := idDoc[id]; ok && target != docIdx {
			sb.WriteString(docs[target].name)
			sb.WriteString("#")
			sb.WriteString(id)
		} else {
			sb.WriteString("#")
			sb.WriteString(id)
		}
	}
}

// headingLevel maps a section nesting depth to a heading level: body titles
// are h1, top-level section titles h2, deeper ones down to h6.
func headingLevel(secDepth int) string {
	level := min(max(secDepth+1, 1), 6)
	return fmt.Sprintf("%d", level)
}

// openTag builds an open tag head without brackets, carrying the id when
// present (e.g. `p id="x"` or `div class="poem"`).
func openTag(tag, id string) string {
	if id == "" {
		return tag
	}
	return tag + ` id="` + html.EscapeString(id) + `"`
}

// attrValue returns a plain (non-namespaced) attribute value.
func attrValue(attrs []xml.Attr, name string) string {
	for _, a := range attrs {
		if a.Name.Space == "" && a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}
