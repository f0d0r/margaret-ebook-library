package fb2

import domain "github.com/f0d0r/margaret-ebook-library/book"

// fb2Book is a parsed FB2 e-book implementing domain.Book.
// Callers should use the Book interface via the root ebook package.
type fb2Book struct {
	metadata  domain.Metadata
	resources *domain.ResourceSet
	version   string
}

func (b *fb2Book) Metadata() domain.Metadata {
	return b.metadata
}

func (b *fb2Book) Resources() *domain.ResourceSet {
	return b.resources
}

func (b *fb2Book) FileType() domain.FileType {
	return domain.FB2
}

func (b *fb2Book) Version() string {
	return b.version
}
