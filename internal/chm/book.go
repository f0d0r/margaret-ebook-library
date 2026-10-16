package chm

import (
	"github.com/f0d0r/margaret-ebook-library/book"
)

// chmBook represents a parsed CHM e-book implementing book.Book.
type chmBook struct {
	metadata  book.Metadata
	resources *book.ResourceSet
	version   string
}

func (b *chmBook) Metadata() book.Metadata {
	return b.metadata
}

func (b *chmBook) Resources() *book.ResourceSet {
	return b.resources
}

func (b *chmBook) FileType() book.FileType {
	return book.CHM
}

func (b *chmBook) Version() string {
	return b.version
}
