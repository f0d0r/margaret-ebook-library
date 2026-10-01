package book

// FileType represents a supported e-book file format.
type FileType string

const (
	// EPUB represents the Electronic Publication format (.epub).
	EPUB FileType = "epub"

	// MOBI represents the Mobipocket e-book format (.mobi).
	MOBI FileType = "mobi"

	// FB2 represents the FictionBook 2 e-book format (.fb2).
	FB2 FileType = "fb2"

	// LIT represents the Microsoft Reader e-book format (.lit).
	LIT FileType = "lit"
)
