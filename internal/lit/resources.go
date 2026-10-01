package lit

import (
	"bytes"
	"fmt"
	"io"
	"regexp"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/opf"
	"github.com/f0d0r/margaret-ebook-library/internal/util"
)

// contentHTMLDecl prefixes decoded spine documents, mirroring the reference
// explode path.
const contentHTMLDecl = `<?xml version="1.0" encoding="UTF-8" ?>
<!DOCTYPE html PUBLIC
 "+//ISBN 0-9673008-1-9//DTD OEB 1.0.1 Document//EN"
 "http://openebook.org/dtds/oeb-1.0.1/oebdoc101.dtd">
`

var (
	smartTagRe = regexp.MustCompile(`(?i)</{0,1}st1:(?:personname|place|city|country-region)>`)
	formTagRe  = regexp.MustCompile(`<(/{0,1})form>`)
)

// cleanContentHTML strips MS smart tags and demotes form elements, as the
// reference reader does after decoding.
func cleanContentHTML(content []byte) []byte {
	content = smartTagRe.ReplaceAll(content, nil)
	return formTagRe.ReplaceAll(content, []byte("<$1div>"))
}

// buildResources assembles the OPF-driven resource set. Spine content
// documents decode through UnBinary HTML; every other entry serves raw
// bytes. All reads stay lazy through the returned Open functions.
func (c *container) buildResources(p opf.Package, maxSize int64) ([]*book.Resource, []book.ReadingOrderItem, *book.Resource) {
	return opf.BuildResources(p, func(item opf.Item) (opf.EntrySource, bool) {
		return c.lookup(item, p, maxSize)
	})
}

// lookup resolves one manifest item to its stored bytes. Missing directory
// entries are skipped; present-but-unreadable entries (e.g. sealed
// sections) stay listed with an Open that reports the failure, mirroring
// the MOBI failing-resource pattern.
func (c *container) lookup(item opf.Item, p opf.Package, maxSize int64) (opf.EntrySource, bool) {
	internal, ok := c.opfIds[util.CleanHref(p.ResolvePath(item.Href))]
	if !ok {
		return opf.EntrySource{}, false
	}
	entry, state := c.contentEntry(internal)
	de, ok := c.entries[entry]
	if !ok {
		return opf.EntrySource{}, false
	}
	size := de.size
	if state == contentTransformed {
		return opf.EntrySource{
			Size: size,
			Open: func() (io.ReadCloser, error) {
				return c.openContent(internal, item.Href, maxSize)
			},
		}, true
	}
	return opf.EntrySource{
		Size: size,
		Open: func() (io.ReadCloser, error) {
			if size > maxSize {
				return nil, book.LimitError(item.Href, size, maxSize)
			}
			data, err := c.getFile(entry)
			if err != nil {
				return nil, err
			}
			return io.NopCloser(bytes.NewReader(data)), nil
		},
	}, true
}

// contentKind distinguishes decoded spine documents from raw entries.
type contentKind int

const (
	contentRaw contentKind = iota
	contentTransformed
)

// contentEntry maps an internal id to its directory entry, choosing the
// transformed content variant for spine-state items. Only css/images serve
// raw bytes; every other state decodes through the content entry, matching
// reference substring semantics ('spine' in state).
func (c *container) contentEntry(internal string) (string, contentKind) {
	if item, ok := c.manifest[internal]; ok {
		if item.state == "css" || item.state == "images" {
			return "/data/" + internal, contentRaw
		}
		return "/data/" + internal + "/content", contentTransformed
	}
	return "/data/" + internal, contentRaw
}

// openContent decodes one spine document through UnBinary HTML on Open.
// doc is the document's OPF href; output hrefs relativize against its
// directory.
func (c *container) openContent(internal, doc string, maxSize int64) (io.ReadCloser, error) {
	entry, _ := c.contentEntry(internal)
	data, err := c.getFile(entry)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxSize {
		return nil, book.LimitError(internal, int64(len(data)), maxSize)
	}
	decoded, err := decodeUnbinary(data, &htmlTables, c.opfPaths, c.getAtoms(internal), doc)
	if err != nil {
		return nil, fmt.Errorf("lit: failed to decode content: %w", err)
	}
	out := append([]byte(contentHTMLDecl), cleanContentHTML(decoded)...)
	if int64(len(out)) > maxSize {
		return nil, book.LimitError(internal, int64(len(out)), maxSize)
	}
	return io.NopCloser(bytes.NewReader(out)), nil
}
