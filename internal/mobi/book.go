package mobi

import domain "github.com/f0d0r/margaret-ebook-library/book"

// mobiBook represents a parsed MOBI e-book implementing domain.Book.
// It is an internal implementation detail; callers should use the Book interface via the root ebook package.
type mobiBook struct {
	metadata  domain.Metadata
	resources *domain.ResourceSet
	version   string
	mobiDoc   *Mobi
}

func (b *mobiBook) Metadata() domain.Metadata {
	return b.metadata
}

func (b *mobiBook) Resources() *domain.ResourceSet {
	return b.resources
}

func (b *mobiBook) FileType() domain.FileType {
	return domain.MOBI
}

func (b *mobiBook) Version() string {
	return b.version
}

// Mobi returns the format-specific parsed MOBI header.
func (b *mobiBook) Mobi() *Mobi {
	return b.mobiDoc
}
