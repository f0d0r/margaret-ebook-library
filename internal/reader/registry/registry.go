package registry

import (
	"faun.projects/margaret/margaret-ebook-library/internal/detector"
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
			&epub.EPUBReader{},
			&mobi.MOBIReader{},
		},
	}
}

func (r *Registry) Get(fileType detector.FileType) (reader.Reader, error) {
	for _, reader := range r.readers {
		if reader.Supports(fileType) {
			return reader, nil
		}
	}
	return nil, errs.ErrUnsupportedFormat
}
