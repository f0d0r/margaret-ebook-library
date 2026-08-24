package book

// FileType represents a supported e-book file format.
type FileType string

const (
	// EPUB represents the Electronic Publication format (.epub).
	EPUB FileType = "epub"

	// MOBI represents the Mobipocket e-book format (.mobi).
	MOBI FileType = "mobi"
)
