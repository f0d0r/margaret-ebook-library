package mobi

import domain "github.com/f0d0r/margaret-ebook-library/book"

// mobiBook represents a parsed MOBI e-book implementing domain.Book.
// It is an internal implementation detail; callers should use the Book interface via the root ebook package.
type mobiBook struct {
	metadata  domain.Metadata
	resources *domain.ResourceSet
	version   string
	mobiDoc   *Mobi
	// isPalmDoc marks books read through the PalmDOC (TEXtREAd) path.
	// It drives FileType, not the Mobi.Type header field (which a
	// BOOKMOBI file could also set to the PalmDoc value), mirroring
	// calibre which distinguishes by container ident.
	isPalmDoc bool
}

func (b *mobiBook) Metadata() domain.Metadata {
	return b.metadata
}

func (b *mobiBook) Resources() *domain.ResourceSet {
	return b.resources
}

func (b *mobiBook) FileType() domain.FileType {
	if b.isPalmDoc {
		return domain.PALMDOC
	}
	return domain.MOBI
}

func (b *mobiBook) Version() string {
	return b.version
}

// Mobi returns the format-specific parsed MOBI header.
func (b *mobiBook) Mobi() *Mobi {
	return b.mobiDoc
}
