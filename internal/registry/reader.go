package registry

import (
	"github.com/f0d0r/margaret-ebook-library/pkg/model"
)

type Reader interface {
	Supports(path string) bool
	ReadMetadata(path string) (*model.Metadata, error)
}
