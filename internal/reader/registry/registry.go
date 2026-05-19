package registry

import (
	"faun.projects/margaret/margaret-ebook-library/internal/reader"
	"faun.projects/margaret/margaret-ebook-library/internal/reader/epub"
	"faun.projects/margaret/margaret-ebook-library/internal/reader/mobi"
	"faun.projects/margaret/margaret-ebook-library/pkg/errs"
)

type Registry struct {
	readers []reader.Reader
}

func New() *Registry {
	return &Registry{
		readers: []reader.Reader{
			&epub.EpubReader{},
			&mobi.MobiReader{},
		},
	}
}

func (r *Registry) Get(path string) (reader.Reader, error) {
	for _, reader := range r.readers {
		if reader.Supports(path) {
			return reader, nil
		}
	}
	return nil, errs.ErrUnsupportedFormat
}
