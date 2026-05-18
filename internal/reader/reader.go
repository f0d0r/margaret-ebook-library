package reader

import (
	"faun.projects/margaret/margaret-ebook-library/internal/detector"
	"faun.projects/margaret/margaret-ebook-library/pkg/model"
)

type Reader interface {
	Supports(fileType detector.FileType) bool
	ReadMetadata(path string) (*model.Metadata, error)
}
