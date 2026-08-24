package registry

import (
	"github.com/f0d0r/margaret-ebook-library/book"
)

type Reader interface {
	Supports(b book.Blob) bool
	Read(b book.Blob) (book.Book, error)
}
