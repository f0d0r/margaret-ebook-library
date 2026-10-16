package chm

import (
	"bytes"
	"io"
	"path"
	"strings"

	"golang.org/x/net/html"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/util"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

// mediaByExt maps content extensions to media types. Unknown extensions
// serve as octet-stream.
func mediaByExt(name string) string {
	switch extOf(name) {
	case ".htm", ".html", ".xhtml", ".hhc", ".hhk":
		return mediatype.HTML
	case ".css":
		return mediatype.CSS
	case ".jpg", ".jpeg":
		return mediatype.JPEG
	case ".png":
		return mediatype.PNG
	case ".gif":
		return mediatype.GIF
	case ".svg":
		return mediatype.SVG
	case ".bmp":
		return mediatype.BMP
	case ".webp":
		return mediatype.WEBP
	}
	return mediatype.OctetStream
}

// isControl reports internal control entries (no book content): #SYSTEM and
// friends, $FIftiMain, ::DataSpace storage.
func isControl(name string) bool {
	trimmed := strings.TrimPrefix(name, "/")
	return strings.HasPrefix(trimmed, "#") ||
		strings.HasPrefix(trimmed, "$") ||
		strings.HasPrefix(trimmed, "::")
}

// buildResources assembles the manifest resources, the TOC-derived reading
// order and the cover. Entry payloads serve raw bytes lazily; unresolvable
// TOC targets are skipped like missing OPF entries.
func (c *container) buildResources() ([]*book.Resource, []book.ReadingOrderItem, *book.Resource) {
	entries, err := c.enumerate()
	if err != nil {
		return nil, nil, nil
	}
	byPath := map[string]*book.Resource{}
	var all []*book.Resource
	for _, ne := range entries {
		if isControl(ne.name) {
			continue
		}
		// PMGL also lists directories; only files become resources.
		if ne.name == "" || strings.HasSuffix(ne.name, "/") {
			continue
		}
		href := util.CleanHref(strings.TrimPrefix(ne.name, "/"))
		if href == "" {
			continue
		}
		ent := ne.ent
		name := ne.name
		res := &book.Resource{
			Id:           href,
			Name:         path.Base(href),
			Href:         href,
			ResolvedHref: href,
			MediaType:    mediaByExt(href),
			Size:         int64(ent.length),
			Open: func() (io.ReadCloser, error) {
				data, err := c.readEntry(name, ent)
				if err != nil {
					return nil, err
				}
				return io.NopCloser(bytes.NewReader(data)), nil
			},
		}
		all = append(all, res)
		if _, ok := byPath[href]; !ok {
			byPath[href] = res
		}
	}

	order := c.readingOrder(entries, byPath)
	return all, order, c.cover(byPath)
}

// readingOrder derives the spine: HHC sitemap first, #TOPICS chain next,
// default topic alone last.
func (c *container) readingOrder(entries []namedEntry, byPath map[string]*book.Resource) []book.ReadingOrderItem {
	defaultTopic := c.defaultTopicPath()
	paths := c.hhcOrder(entries, defaultTopic)
	if len(paths) == 0 {
		paths = c.topicsOrder()
	}
	if len(paths) == 0 && defaultTopic != "" && c.resolveProbe(defaultTopic) {
		paths = []string{defaultTopic}
	}
	var order []book.ReadingOrderItem
	for _, p := range paths {
		href := util.CleanHref(strings.TrimPrefix(p, "/"))
		res, ok := byPath[href]
		if !ok {
			res = lookupFold(byPath, href)
		}
		if res == nil || !isTopic(res) {
			continue
		}
		order = append(order, book.ReadingOrderItem{Resource: res, Linear: true})
	}
	return order
}

// isTopic reports spine-eligible resources: HTML topics only. Sitemaps,
// images and other payloads stay in the manifest but out of the spine,
// mirroring EPUB (manifest-all vs. spine-subset).
func isTopic(res *book.Resource) bool {
	if res.MediaType != mediatype.HTML {
		return false
	}
	switch extOf(res.ResolvedHref) {
	case ".hhc", ".hhk":
		return false
	}
	return true
}

// lookupFold finds a manifest resource case-insensitively.
func lookupFold(byPath map[string]*book.Resource, href string) *book.Resource {
	for h, res := range byPath {
		if foldEqual(h, href) {
			return res
		}
	}
	return nil
}

// cover resolves the conservative cover heuristic: an image with 'cover' in
// its alt text on the default topic. Anything else yields no cover.
func (c *container) cover(byPath map[string]*book.Resource) *book.Resource {
	defaultTopic := c.defaultTopicPath()
	if defaultTopic == "" {
		return nil
	}
	_, doc, err := c.defaultTopicDoc()
	if err != nil {
		return nil
	}
	src := coverSrc(doc)
	if src == "" {
		return nil
	}
	href := util.CleanHref(joinTopicDir(defaultTopic, src))
	if res, ok := byPath[href]; ok && strings.HasPrefix(res.MediaType, "image/") {
		return res
	}
	if res := lookupFold(byPath, href); res != nil && strings.HasPrefix(res.MediaType, "image/") {
		return res
	}
	return nil
}

// coverSrc returns the first image source whose alt text mentions cover.
func coverSrc(head *html.Node) string {
	if head == nil {
		return ""
	}
	var src string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if src != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "img" {
			var alt, s string
			for _, a := range n.Attr {
				switch a.Key {
				case "alt":
					alt = a.Val
				case "src":
					s = a.Val
				}
			}
			if s != "" && strings.Contains(strings.ToLower(alt), "cover") {
				src = s
				return
			}
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(head)
	return src
}

// joinTopicDir resolves a possibly relative image src against the topic path.
func joinTopicDir(topic, src string) string {
	src = strings.TrimSpace(src)
	if i := strings.IndexByte(src, '#'); i >= 0 {
		src = src[:i]
	}
	if strings.HasPrefix(src, "/") {
		return strings.TrimPrefix(src, "/")
	}
	dir := path.Dir(strings.TrimPrefix(topic, "/"))
	if dir == "." {
		return src
	}
	return path.Join(dir, src)
}
