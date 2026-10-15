package mobi

import domain "github.com/f0d0r/margaret-ebook-library/book"

// mobiBook is the domain.Book implementation for MOBI.
type mobiBook struct {
	metadata  domain.Metadata
	resources *domain.ResourceSet
	version   string
	mobiDoc   *Mobi
	// isPalmDoc drives FileType by container ident, not the Mobi.Type header field (like calibre).
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
