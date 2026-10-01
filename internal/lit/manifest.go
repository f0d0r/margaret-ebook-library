package lit

import (
	"encoding/binary"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/util"
)

// manifestItem maps one internal LIT id to its output path.
type manifestItem struct {
	original string
	internal string
	mime     string
	offset   int64
	root     string
	state    string
	path     string
}

// readManifest parses the /manifest entry into internal-id items.
func readManifest(raw []byte) (map[string]manifestItem, error) {
	manifest := map[string]manifestItem{}
	for len(raw) > 0 {
		slen := int(raw[0])
		raw = raw[1:]
		if slen == 0 {
			break
		}
		if len(raw) < slen {
			return nil, fmt.Errorf("lit: truncated manifest root: %w", book.ErrCorrupt)
		}
		root := string(raw[:slen])
		if !utf8.ValidString(root) {
			return nil, fmt.Errorf("lit: invalid manifest root: %w", book.ErrCorrupt)
		}
		raw = raw[slen:]
		if len(raw) == 0 {
			return nil, fmt.Errorf("lit: truncated manifest: %w", book.ErrCorrupt)
		}
		for _, state := range []string{"spine", "not spine", "css", "images"} {
			if len(raw) < 4 {
				return nil, fmt.Errorf("lit: truncated manifest count: %w", book.ErrCorrupt)
			}
			numFiles := int(int32LE(raw))
			raw = raw[4:]
			for i := 0; i < numFiles; i++ {
				if len(raw) < 5 {
					return nil, fmt.Errorf("lit: truncated manifest entry: %w", book.ErrCorrupt)
				}
				offset := int64(binary.LittleEndian.Uint32(raw))
				raw = raw[4:]
				var internal, original, mime string
				var err error
				if internal, raw, err = sizedString(raw, false); err != nil {
					return nil, err
				}
				if original, raw, err = sizedString(raw, false); err != nil {
					return nil, err
				}
				original = unquote(original)
				if mime, raw, err = sizedString(raw, true); err != nil {
					return nil, err
				}
				manifest[internal] = newManifestItem(original, internal, strings.ToLower(mime), offset, root, state)
			}
		}
	}
	stripSharedPrefix(manifest)
	return manifest, nil
}

// newManifestItem normalizes raw manifest fields into an item.
func newManifestItem(original, internal, mime string, offset int64, root, state string) manifestItem {
	p := strings.ReplaceAll(original, "\\", "/")
	if len(p) > 2 && p[1] == ':' && p[2] == '/' {
		p = p[2:]
	}
	p = path.Clean(p)
	for strings.HasPrefix(p, "../") {
		p = p[3:]
	}
	return manifestItem{original: original, internal: internal, mime: mime, offset: offset, root: root, state: state, path: p}
}

// stripSharedPrefix removes path elements common to all items and isolates
// absolute leftovers to basenames. Keys iterate sorted so output never
// depends on map order.
func stripSharedPrefix(manifest map[string]manifestItem) {
	if len(manifest) < 2 {
		return
	}
	ids := make([]string, 0, len(manifest))
	for id := range manifest {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	shared := manifest[ids[0]].path
	for _, id := range ids[1:] {
		for shared != "" && !strings.HasPrefix(manifest[id].path, shared) {
			idx := strings.LastIndex(shared[:len(shared)-1], "/")
			if idx < 0 {
				shared = ""
				break
			}
			shared = shared[:idx+1]
		}
		if shared == "" {
			break
		}
	}
	if shared != "" {
		for id, item := range manifest {
			item.path = strings.TrimPrefix(item.path, shared)
			manifest[id] = item
		}
	}
	for id, item := range manifest {
		if strings.HasPrefix(item.path, "/") {
			item.path = path.Base(item.path)
			manifest[id] = item
		}
	}
}

// indexManifest builds both lookup directions once per container, over
// sorted ids so path collisions resolve deterministically (first wins).
func (c *container) indexManifest() {
	ids := make([]string, 0, len(c.manifest))
	for id := range c.manifest {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	c.opfPaths = make(map[string]string, len(c.manifest))
	c.opfIds = make(map[string]string, len(c.manifest))
	for _, id := range ids {
		item := c.manifest[id]
		c.opfPaths[id] = item.path
		key := util.CleanHref(item.path)
		if _, ok := c.opfIds[key]; !ok {
			c.opfIds[key] = id
		}
	}
}

// sizedString reads a length-prefixed UTF-8 string: the length is one
// character giving the following character count. zpad consumes one trailing
// NUL after the string, which some fields carry.
func sizedString(raw []byte, zpad bool) (string, []byte, error) {
	// decodeRune reports the consumed width, which doubles as the offset of
	// the remainder when called from position 0.
	n, size, err := decodeRune(raw, 0)
	if err != nil {
		return "", nil, err
	}
	raw = raw[size:]
	var sb strings.Builder
	for i := rune(0); i < n; i++ {
		var c rune
		if c, size, err = decodeRune(raw, 0); err != nil {
			return "", nil, err
		}
		sb.WriteRune(c)
		raw = raw[size:]
	}
	if zpad && len(raw) > 0 && raw[0] == 0 {
		raw = raw[1:]
	}
	return sb.String(), raw, nil
}

// unquote percent-decodes manifest paths, keeping raw text on failure.
func unquote(s string) string {
	if decoded, err := url.PathUnescape(s); err == nil {
		return decoded
	}
	return s
}
