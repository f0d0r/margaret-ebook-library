package registry

import (
	"github.com/f0d0r/margaret-ebook-library/pkg/model"
)

type Reader interface {
	Supports(b model.Blob) bool
	ReadMetadata(b model.Blob) (*model.Metadata, error)
}
