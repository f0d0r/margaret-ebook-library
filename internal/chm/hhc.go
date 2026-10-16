package chm

import (
	"bytes"
	"path"
	"sort"
	"strings"

	"golang.org/x/net/html"
)

// hhcOrder returns the .hhc sitemap reading order candidates in document
// order. With several .hhc files the one beside the default topic wins;
// without any, it returns nil.
func (c *container) hhcOrder(entries []namedEntry, defaultTopic string) []string {
	hhc := pickHHC(entries, defaultTopic)
	if hhc == "" {
		return nil
	}
	raw, err := c.getFile(hhc)
	if err != nil {
		return nil
	}
	return c.orderCandidates(hhcLocals(raw, c.charset()))
}

// pickHHC selects the table-of-contents file: same directory as the default
// topic wins, then same stem, then shallowest, then lexical first.
func pickHHC(entries []namedEntry, defaultTopic string) string {
	var hhcs []string
	for _, e := range entries {
		if strings.EqualFold(extOf(e.name), ".hhc") {
			hhcs = append(hhcs, e.name)
		}
	}
	if len(hhcs) == 0 {
		return ""
	}
	sort.Strings(hhcs)
	if defaultTopic == "" {
		return hhcs[0]
	}
	defDir, defStem := splitPath(defaultTopic)
	best, bestScore := hhcs[0], -1
	for _, h := range hhcs {
		dir, stem := splitPath(h)
		score := 0
		if dir == defDir {
			score += 2
		}
		if stem == defStem {
			score += 1
		}
		score -= strings.Count(h, "/")
		if score > bestScore {
			best, bestScore = h, score
		}
	}
	return best
}

// splitPath splits an internal path into directory and stem (no extension).
func splitPath(p string) (dir, stem string) {
	p = strings.TrimPrefix(p, "/")
	dir = path.Dir(p)
	base := path.Base(p)
	stem = strings.TrimSuffix(base, path.Ext(base))
	return dir, stem
}

// hhcLocals extracts sitemap Local targets in document order:
// <OBJECT type="text/sitemap"> with <param name="Local" value="...">.
func hhcLocals(raw []byte, charset string) []string {
	doc, err := html.Parse(bytes.NewReader([]byte(decodeText(raw, charset))))
	if err != nil {
		return nil
	}
	var out []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "object" && objectType(n, "text/sitemap") {
			if local := objectParam(n, "local"); local != "" {
				if p := cleanTopicPath(local); p != "" {
					out = append(out, p)
				}
			}
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(doc)
	return out
}

// objectType reports whether an object tag carries the given type.
func objectType(n *html.Node, want string) bool {
	for _, a := range n.Attr {
		if a.Key == "type" && strings.EqualFold(a.Val, want) {
			return true
		}
	}
	return false
}

// objectParam returns a direct param child's value for name.
func objectParam(n *html.Node, name string) string {
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type != html.ElementNode || ch.Data != "param" {
			continue
		}
		var pname, pvalue string
		for _, a := range ch.Attr {
			switch a.Key {
			case "name":
				pname = a.Val
			case "value":
				pvalue = a.Val
			}
		}
		if strings.EqualFold(pname, name) {
			return pvalue
		}
	}
	return ""
}

// extOf returns the lowercase extension of an internal path.
func extOf(p string) string {
	return strings.ToLower(path.Ext(p))
}
