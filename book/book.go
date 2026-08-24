package book

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

// Metadata represents the extracted core information of an e-book.
type Metadata struct {
	// Title is the main title of the e-book.
	Title string

	// Authors are the creators or writers of the book.
	Authors []string

	// Description is a brief summary or description of the book.
	Description string

	// Languages is the language(s) of the book.
	Languages []string
}
