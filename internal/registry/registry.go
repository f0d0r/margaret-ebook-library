package registry

import (
	"faun.projects/margaret/margaret-ebook-library/internal/epub"
	"faun.projects/margaret/margaret-ebook-library/internal/mobi"
	"faun.projects/margaret/margaret-ebook-library/pkg/errs"
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

func (r *Registry) GetReader(path string) (Reader, error) {
	for _, reader := range r.readers {
		if reader.Supports(path) {
			return reader, nil
		}
	}
	return nil, errs.ErrUnsupportedFormat
}
