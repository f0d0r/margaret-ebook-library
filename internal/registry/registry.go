package registry

import (
	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/chm"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
	"github.com/f0d0r/margaret-ebook-library/internal/epub"
	"github.com/f0d0r/margaret-ebook-library/internal/ereader"
	"github.com/f0d0r/margaret-ebook-library/internal/fb2"
	"github.com/f0d0r/margaret-ebook-library/internal/fb3"
	"github.com/f0d0r/margaret-ebook-library/internal/lit"
	"github.com/f0d0r/margaret-ebook-library/internal/mobi"
)

type Registry struct {
	readers []Reader
}

func New(cfg ...config.Config) *Registry {
	c := config.DefaultConfig()
	if len(cfg) > 0 {
		c = cfg[0]
	}
	return &Registry{
		readers: []Reader{
			mobi.NewMobiReader(c),
			ereader.NewEreaderReader(c),
			epub.NewEpubReader(c),
			fb3.NewFb3Reader(c),
			fb2.NewFb2Reader(c),
			lit.NewLitReader(c),
			chm.NewChmReader(c),
		},
	}
}

// ReaderForBlob selects a reader based on the file contents.
func (r *Registry) ReaderForBlob(b book.Blob) (Reader, error) {
	for _, reader := range r.readers {
		if reader.Supports(b) {
			return reader, nil
		}
	}
	return nil, book.ErrUnsupportedFormat
}
