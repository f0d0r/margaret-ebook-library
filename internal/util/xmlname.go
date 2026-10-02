package util

import "strings"

// IsXMLNameChar reports whether c may appear in an XML tag or
// namespace-prefix name after the first character (ASCII subset; the
// name-matching call sites only ever need to recognize ASCII names).
func IsXMLNameChar(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	case c == '_' || c == '-' || c == '.' || c == ':':
		return true
	default:
		return false
	}
}

// ScanOpenTag scans buf for an opening <tag> element. Closing tags,
// processing instructions and markup declarations are skipped, and the
// character following the tag name must be a tag delimiter so that prose
// mentioning the word does not match. When foldCase is set the name
// matches case-insensitively (HTML sniffing); otherwise it is exact (XML,
// matching calibre's lxml parsing). When stripPrefix is set an optional
// namespace prefix is accepted (e.g. <fb:FictionBook>). A probe truncated
// mid-tag still counts as found.
func ScanOpenTag(buf []byte, tag string, foldCase, stripPrefix bool) bool {
	for i := range len(buf) {
		if buf[i] != '<' {
			continue
		}
		j := i + 1
		if j < len(buf) && (buf[j] == '/' || buf[j] == '?' || buf[j] == '!') {
			continue
		}
		k := j
		for k < len(buf) && IsXMLNameChar(buf[k]) {
			k++
		}
		name := string(buf[j:k])
		if stripPrefix {
			if idx := strings.LastIndexByte(name, ':'); idx >= 0 {
				name = name[idx+1:]
			}
		}
		match := name == tag
		if foldCase {
			match = strings.EqualFold(name, tag)
		}
		if !match {
			continue
		}
		if k >= len(buf) {
			return true
		}
		if buf[k] == '>' || buf[k] == '/' || isTagSpace(buf[k]) {
			return true
		}
	}
	return false
}

func isTagSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}
