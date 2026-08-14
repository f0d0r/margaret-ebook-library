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

func New(cfg ...model.Config) *Registry {
	c := model.DefaultConfig()
	if len(cfg) > 0 {
		c = cfg[0]
	}
	return &Registry{
		readers: []Reader{
			epub.NewEpubReaderWithConfig(c),
			mobi.NewMobiReaderWithConfig(c),
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
