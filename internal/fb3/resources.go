package fb3

import (
	"archive/zip"
	"io"
	"path"
	"strings"

	"github.com/f0d0r/margaret-ebook-library/book"
	compressutil "github.com/f0d0r/margaret-ebook-library/internal/compress"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
	"github.com/f0d0r/margaret-ebook-library/internal/util"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

// buildFb3Resources assembles body, images, reading order and cover
// without decompressing anything.
func buildFb3Resources(zr *zip.Reader, body *zip.File, images []imageRef, media *mediaMap, coverPath string, maxSize int64) ([]*book.Resource, []book.ReadingOrderItem, *book.Resource) {
	if maxSize <= 0 {
		maxSize = config.DefaultConfig().MaxResourceSize
	}

	bodyRes := lazyZipResource("body", body, mediatype.FB3Body, maxSize)
	all := []*book.Resource{bodyRes}
	readingOrder := []book.ReadingOrderItem{{Resource: bodyRes, Linear: true}}

	var cover *book.Resource
	for _, img := range images {
		f := resolvePart(zr, path.Dir(body.Name), img.target)
		if f == nil {
			continue
		}
		res := lazyZipResource(img.id, f, media.mediaFor(f.Name), maxSize)
		all = append(all, res)
		if cover == nil && coverPath != "" && strings.EqualFold(f.Name, coverPath) {
			res.Properties = "cover-image"
			cover = res
		}
	}
	return all, readingOrder, cover
}

// lazyZipResource wraps a ZIP entry as a lazily-opened resource.
func lazyZipResource(id string, f *zip.File, mediaType string, maxSize int64) *book.Resource {
	href := f.Name
	return &book.Resource{
		Id:           id,
		Name:         path.Base(href),
		MediaType:    mediaType,
		Href:         href,
		ResolvedHref: util.CleanHref(href),
		Size:         int64(f.UncompressedSize64),
		Open: func() (io.ReadCloser, error) {
			return compressutil.OpenLimited(f, maxSize)
		},
	}
}
