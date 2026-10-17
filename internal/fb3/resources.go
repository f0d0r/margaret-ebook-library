package fb3

import (
	"archive/zip"
	"io"
	"path"
	"strings"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/charset"
	compressutil "github.com/f0d0r/margaret-ebook-library/internal/compress"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
	"github.com/f0d0r/margaret-ebook-library/internal/util"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

// charsetSniffSize bounds the body-head read used to record the storage
// encoding. The head is decompressed but never served: Open still streams
// the raw entry bytes.
const charsetSniffSize = 2048

// buildFb3Resources assembles body, images, reading order and cover
// without decompressing anything beyond the body's head (charset sniff).
func buildFb3Resources(zr *zip.Reader, body *zip.File, images []imageRef, media *mediaMap, coverPath string, maxSize int64) ([]*book.Resource, []book.ReadingOrderItem, *book.Resource) {
	if maxSize <= 0 {
		maxSize = config.DefaultConfig().MaxResourceSize
	}

	bodyRes := lazyZipResource("body", body, mediatype.FB3Body, bodyCharset(body), maxSize)
	all := []*book.Resource{bodyRes}
	readingOrder := []book.ReadingOrderItem{{Resource: bodyRes, Linear: true}}

	var cover *book.Resource
	for _, img := range images {
		f := resolvePart(zr, path.Dir(body.Name), img.target)
		if f == nil {
			continue
		}
		res := lazyZipResource(img.id, f, media.mediaFor(f.Name), "", maxSize)
		all = append(all, res)
		if cover == nil && coverPath != "" && strings.EqualFold(f.Name, coverPath) {
			res.Properties = "cover-image"
			cover = res
		}
	}
	if cover == nil && coverPath != "" {
		// The thumbnail is not a body image (e.g. a root cover.jpg):
		// list it as the cover anyway.
		if f := resolvePart(zr, "", coverPath); f != nil {
			cover = lazyZipResource(path.Base(coverPath), f, media.mediaFor(f.Name), "", maxSize)
			cover.Properties = "cover-image"
			all = append(all, cover)
		}
	}
	return all, readingOrder, cover
}

// bodyCharset reports the storage encoding of the FB3 body by sniffing its
// head for an XML declaration, defaulting to UTF-8 (also the XML default).
func bodyCharset(body *zip.File) string {
	if head, err := compressutil.ReadHead(body, charsetSniffSize); err == nil {
		if cs, ok := charset.DetectDeclared(head); ok {
			return cs
		}
	}
	return book.CharsetUTF8
}

// lazyZipResource wraps a ZIP entry as a lazily-opened resource. charset is
// the storage encoding ("", when unknown, resolves through the media type).
func lazyZipResource(id string, f *zip.File, mediaType, charset string, maxSize int64) *book.Resource {
	href := f.Name
	return &book.Resource{
		Id:           id,
		Name:         path.Base(href),
		MediaType:    mediaType,
		Charset:      charset,
		Href:         href,
		ResolvedHref: util.CleanHref(href),
		Size:         int64(f.UncompressedSize64),
		Open: func() (io.ReadCloser, error) {
			return compressutil.OpenLimited(f, maxSize)
		},
	}
}
