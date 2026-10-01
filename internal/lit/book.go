package lit

import (
	"github.com/f0d0r/margaret-ebook-library/book"
)

// litVersionString is the reported format version: the LIT primary header
// version, which both reference implementations validate as 1. OEB 1.0.1
// packages carry no version attribute of their own.
const litVersionString = "1"

// litBook represents a parsed LIT e-book implementing book.Book.
type litBook struct {
	metadata  book.Metadata
	resources *book.ResourceSet
	version   string
}

func (b *litBook) Metadata() book.Metadata {
	return b.metadata
}

func (b *litBook) Resources() *book.ResourceSet {
	return b.resources
}

func (b *litBook) FileType() book.FileType {
	return book.LIT
}

func (b *litBook) Version() string {
	return b.version
}
