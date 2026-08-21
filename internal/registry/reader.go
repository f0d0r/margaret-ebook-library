package registry

import (
	"github.com/f0d0r/margaret-ebook-library/pkg/model"
)

type Reader interface {
	Supports(b model.Blob) bool
	Read(b model.Blob) (*model.Ebook, error)
}
