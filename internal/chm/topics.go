package chm

import (
	"encoding/binary"
	"strings"
)

// topicsPaths resolves every #TOPICS entry to its topic path in file order.
// Each entry is 16 bytes: the #URLTBL offset at +8 points at a 12-byte slot
// whose #URLSTR offset at +8 locates the URL string. See chmspec 5.3.8-5.3.10.
// Unresolvable entries are skipped.
func (c *container) topicsPaths() []string {
	topics, err := c.getFile("/#TOPICS")
	if err != nil {
		return nil
	}
	urltbl, err := c.getFile("/#URLTBL")
	if err != nil {
		return nil
	}
	urlstr, err := c.getFile("/#URLSTR")
	if err != nil {
		return nil
	}
	var out []string
	for pos := 0; pos+16 <= len(topics); pos += 16 {
		off := binary.LittleEndian.Uint32(topics[pos+8:])
		if uint64(off)+12 > uint64(len(urltbl)) {
			continue
		}
		strOff := binary.LittleEndian.Uint32(urltbl[off+8:])
		url := urlstrAt(urlstr, strOff)
		if url == "" {
			continue
		}
		if p := cleanTopicPath(url); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// urlstrAt reads one #URLSTR entry: two DWORD offsets (URL, frame name)
// followed by the Local string. A zero URL offset means the URL is the
// Local itself, the common case. See chmspec 5.3.9.
func urlstrAt(raw []byte, off uint32) string {
	if uint64(off)+8 > uint64(len(raw)) {
		return ""
	}
	local := string(nulCut(raw[off+8:]))
	urlOff := binary.LittleEndian.Uint32(raw[off:])
	if urlOff == 0 {
		return local
	}
	if uint64(urlOff) >= uint64(len(raw)) {
		return ""
	}
	return string(nulCut(raw[urlOff:]))
}

// cleanTopicPath normalizes a TOC URL to an internal absolute path: strips
// ';junk' suffixes and '#fragment's, rejects off-site targets.
func cleanTopicPath(url string) string {
	url = strings.TrimSpace(url)
	if i := strings.IndexByte(url, ';'); i >= 0 {
		url = url[:i]
	}
	if i := strings.IndexByte(url, '#'); i >= 0 {
		url = url[:i]
	}
	url = strings.TrimSpace(url)
	if url == "" || strings.Contains(url, "://") || strings.HasPrefix(url, "javascript:") {
		return ""
	}
	if !strings.HasPrefix(url, "/") {
		url = "/" + url
	}
	return url
}

// resolveProbe reports whether an internal path resolves.
func (c *container) resolveProbe(path string) bool {
	_, err := c.resolve(path)
	return err == nil
}

// orderCandidates filters candidate paths to resolvable, deduplicated ones.
func (c *container) orderCandidates(paths []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, p := range paths {
		if seen[p] || !c.resolveProbe(p) {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// topicsOrder returns the #TOPICS-derived reading order candidates.
func (c *container) topicsOrder() []string {
	return c.orderCandidates(c.topicsPaths())
}

// defaultTopicPath returns the #SYSTEM default topic as an internal path.
func (c *container) defaultTopicPath() string {
	sys, err := c.cachedSystem()
	if err != nil {
		return ""
	}
	topic := squash(decodeText(nulCut(sys.first(sysDefaultTopic)), c.charsetFromSys(sys)))
	if topic == "" {
		return ""
	}
	return cleanTopicPath(topic)
}

// charsetFromSys derives the ANSI charset from the #SYSTEM LCID entry.
func (c *container) charsetFromSys(sys *system) string {
	if info := sys.first(sysInfo); len(info) >= 4 {
		return lcidCharset(binary.LittleEndian.Uint32(info[:4]))
	}
	return "windows-1252"
}

// charset returns the container charset, best-effort.
func (c *container) charset() string {
	sys, err := c.cachedSystem()
	if err != nil {
		return "windows-1252"
	}
	return c.charsetFromSys(sys)
}
