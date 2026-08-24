package epub

import domain "github.com/f0d0r/margaret-ebook-library/book"

// epubBook represents a parsed EPUB e-book implementing domain.Book.
// It is an internal implementation detail; callers should use the Book interface via the root ebook package.
type epubBook struct {
	metadata   domain.Metadata
	resources  *domain.ResourceSet
	version    string
	packageDoc Package
}

func (b *epubBook) Metadata() domain.Metadata {
	return b.metadata
}

func (b *epubBook) Resources() *domain.ResourceSet {
	return b.resources
}

func (b *epubBook) FileType() domain.FileType {
	return domain.EPUB
}

func (b *epubBook) Version() string {
	return b.version
}

// Package returns the format-specific parsed OPF package document.
func (b *epubBook) Package() Package {
	return b.packageDoc
}
