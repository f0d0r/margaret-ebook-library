package registry

import (
	"github.com/f0d0r/margaret-ebook-library/internal/epub"
	"github.com/f0d0r/margaret-ebook-library/internal/mobi"
	"github.com/f0d0r/margaret-ebook-library/pkg/errs"
	"github.com/f0d0r/margaret-ebook-library/pkg/model"
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

// ReaderForBlob selects a reader based on the file contents.
func (r *Registry) ReaderForBlob(b model.Blob) (Reader, error) {
	for _, reader := range r.readers {
		if reader.Supports(b) {
			return reader, nil
		}
	}
	return nil, errs.ErrUnsupportedFormat
}
