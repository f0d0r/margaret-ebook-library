package registry

import (
	"git.home/margaret/margaret-ebook-library/pkg/model"
)

type Reader interface {
	Supports(path string) bool
	ReadMetadata(path string) (*model.Metadata, error)
}
