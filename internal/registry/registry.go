package registry

import (
	"github.com/f0d0r/margaret-ebook-library/internal/epub"
	"github.com/f0d0r/margaret-ebook-library/internal/mobi"
	"github.com/f0d0r/margaret-ebook-library/pkg/errs"
)

type Registry struct {
	readers []Reader
}

func New() *Registry {
	return &Registry{
		readers: []Reader{
			&epub.EpubReader{},
			&mobi.MobiReader{},
		},
	}
}

func (r *Registry) ReaderFor(path string) (Reader, error) {
	for _, reader := range r.readers {
		if reader.Supports(path) {
			return reader, nil
		}
	}
	return nil, errs.ErrUnsupportedFormat
}
