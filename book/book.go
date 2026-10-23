package book

import "strings"

// Book represents a parsed e-book (EPUB, MOBI, etc.).
// It provides access to normalized metadata, embedded resources, format type,
// and format version.
type Book interface {
	// Metadata returns the core normalized metadata of the e-book.
	Metadata() Metadata

	// Resources returns the embedded resources (manifest and spine reading order).
	Resources() *ResourceSet

	// FileType returns the e-book format type (EPUB, MOBI, etc.).
	FileType() FileType

	// Version returns the format version declared by the e-book:
	// OPF package version (e.g. "3.0") for EPUB, MOBI header version (e.g. "6" or "8") for MOBI.
	Version() string
}

// Identifier is an identifier declared by the e-book metadata.
type Identifier struct {
	// Scheme names the identifier system (e.g. "ISBN", "DOI", "UUID",
	// "calibre"). It is empty when the format does not state one and no
	// scheme prefix is present.
	Scheme string

	// Value is the identifier with any detected scheme prefix removed and
	// surrounding whitespace trimmed. It is not otherwise validated or
	// normalized.
	Value string
}

// Metadata represents the extracted core information of an e-book.
type Metadata struct {
	// Title is the main title of the e-book.
	Title string

	// Authors are the creators or writers of the book.
	Authors []string

	// Description is a brief summary or description of the book.
	Description string

	// Subjects are the subject headings or keywords declared by the e-book
	// metadata, in source order.
	Subjects []string

	// Languages is the language(s) of the book.
	Languages []string

	// Identifiers are the identifiers declared by the e-book metadata, in
	// source order. Scheme prefixes are moved to Identifier.Scheme;
	Identifiers []Identifier

	// Publisher is the publisher declared by the e-book metadata, with
	// surrounding whitespace trimmed.
	Publisher string

	// PublicationDate is the raw publication date declared by the e-book
	// metadata. It is returned as declared, without parsing or validation,
	// so it may contain arbitrary or malformed text.
	PublicationDate string
}

// Identifier returns the first identifier with the given scheme, matched
// case-insensitively. It returns false when no such identifier exists.
func (m Metadata) Identifier(scheme string) (string, bool) {
	for _, id := range m.Identifiers {
		if id.Value != "" && strings.EqualFold(id.Scheme, scheme) {
			return id.Value, true
		}
	}
	return "", false
}
