package lrf

import "github.com/f0d0r/margaret-ebook-library/book"

type lrfBook struct {
	metadata  book.Metadata
	resources *book.ResourceSet
	version   string
}

func (b *lrfBook) Metadata() book.Metadata {
	return b.metadata
}

func (b *lrfBook) Resources() *book.ResourceSet {
	return b.resources
}

func (b *lrfBook) FileType() book.FileType {
	return book.LRF
}

func (b *lrfBook) Version() string {
	return b.version
}
