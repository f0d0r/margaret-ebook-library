package chm

import (
	"bytes"
	"encoding/binary"
	"strings"

	"github.com/f0d0r/margaret-ebook-library/book"
	"golang.org/x/net/html"
)

// readMetadata assembles book metadata. Title falls back from the #SYSTEM
// title through the default-topic HTML title to the first #TOPICS title.
// Authors and description come only from default-topic meta tags; languages
// from the #SYSTEM LCID. Missing sources yield empty fields, never errors.
func (c *container) readMetadata() book.Metadata {
	sys, err := c.cachedSystem()
	if err != nil {
		return book.Metadata{}
	}
	charset := c.charsetFromSys(sys)
	var lcid uint32
	if info := sys.first(sysInfo); len(info) >= 4 {
		lcid = binary.LittleEndian.Uint32(info[:4])
	}

	meta := book.Metadata{}
	if lang := lcidLanguage(lcid); lang != "" {
		meta.Languages = []string{lang}
	}

	_, doc, _ := c.defaultTopicDoc()
	head := headFromDoc(doc)

	meta.Title = squash(decodeText(nulCut(sys.first(sysTitle)), charset))
	if meta.Title == "" {
		meta.Title = headTitle(head)
	}
	if meta.Title == "" {
		meta.Title = c.topicsTitle(charset)
	}

	if head != nil {
		if author := headMeta(head, "author"); author != "" {
			for _, a := range strings.Split(author, ";") {
				if a = strings.TrimSpace(a); a != "" {
					meta.Authors = append(meta.Authors, a)
				}
			}
		}
		meta.Description = strings.TrimSpace(headMeta(head, "description"))
	}
	return meta
}

// topicsTitle returns the first titled #TOPICS entry via #STRINGS.
func (c *container) topicsTitle(charset string) string {
	topics, err := c.getFile("/#TOPICS")
	if err != nil {
		return ""
	}
	stringsRaw, err := c.getFile("/#STRINGS")
	if err != nil {
		return ""
	}
	for pos := 0; pos+16 <= len(topics); pos += 16 {
		off := binary.LittleEndian.Uint32(topics[pos+4:])
		if off == 0xFFFFFFFF {
			continue
		}
		if title := squash(stringsAt(stringsRaw, off, charset)); title != "" {
			return title
		}
	}
	return ""
}

// defaultTopicDoc returns the default topic bytes and parsed document,
// memoized so metadata and cover share one decompression and parse.
func (c *container) defaultTopicDoc() ([]byte, *html.Node, error) {
	c.mu.Lock()
	if c.topicDone {
		raw, doc, err := c.topicRaw, c.topicDoc, c.topicErr
		c.mu.Unlock()
		return raw, doc, err
	}
	c.mu.Unlock()

	raw, doc, err := c.loadTopicDoc()

	c.mu.Lock()
	if !c.topicDone {
		c.topicRaw, c.topicDoc, c.topicErr, c.topicDone = raw, doc, err, true
	} else {
		raw, doc, err = c.topicRaw, c.topicDoc, c.topicErr
	}
	c.mu.Unlock()
	return raw, doc, err
}

// loadTopicDoc reads and parses the default topic without memoization.
func (c *container) loadTopicDoc() ([]byte, *html.Node, error) {
	sys, err := c.cachedSystem()
	if err != nil {
		return nil, nil, err
	}
	charset := c.charsetFromSys(sys)
	topicPath := squash(decodeText(nulCut(sys.first(sysDefaultTopic)), charset))
	if topicPath == "" {
		return nil, nil, nil
	}
	raw, err := c.getFile("/" + strings.TrimPrefix(topicPath, "/"))
	if err != nil {
		return nil, nil, err
	}
	return raw, parseDoc(raw, charset), nil
}

// cachedSystem returns the parsed #SYSTEM, memoized under mu. The lock is
// never held across the read itself: getFile may recurse into ensureComp,
// which takes the same mutex.
func (c *container) cachedSystem() (*system, error) {
	c.mu.Lock()
	if c.sysDone {
		sys, err := c.sysCache, c.sysErr
		c.mu.Unlock()
		return sys, err
	}
	c.mu.Unlock()

	sys, err := c.readSystem()

	c.mu.Lock()
	if !c.sysDone {
		c.sysCache, c.sysErr, c.sysDone = sys, err, true
	} else {
		sys, err = c.sysCache, c.sysErr
	}
	c.mu.Unlock()
	return sys, err
}

// parseDoc parses HTML bytes into a document tree.
func parseDoc(raw []byte, charset string) *html.Node {
	text := decodeText(raw, charset)
	doc, err := html.Parse(bytes.NewReader([]byte(text)))
	if err != nil {
		return nil
	}
	return doc
}

// headFromDoc returns the head section of a parsed document.
func headFromDoc(doc *html.Node) *html.Node {
	if doc == nil {
		return nil
	}
	var head *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if head != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "head" {
			head = n
			return
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(doc)
	if head == nil {
		return doc
	}
	return head
}

// headTitle returns the head title text.
func headTitle(head *html.Node) string {
	if head == nil {
		return ""
	}
	var title string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if title != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "title" {
			title = squash(textOf(n))
			return
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(head)
	return title
}

// headMeta returns the first meta content for name (case-insensitive; x/net
// lowercases attribute names already).
func headMeta(head *html.Node, name string) string {
	if head == nil {
		return ""
	}
	var content string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if content != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "meta" {
			var mname, mcontent string
			for _, a := range n.Attr {
				switch a.Key {
				case "name", "property":
					mname = strings.ToLower(a.Val)
				case "content":
					mcontent = a.Val
				}
			}
			if mname == name {
				content = mcontent
				return
			}
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(head)
	return content
}

// textOf concatenates text descendants.
func textOf(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(n)
	return sb.String()
}
