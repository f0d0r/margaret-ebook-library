package model

// Metadata represents the extracted core information of an e-book.
// It normalized data across different e-book formats like EPUB, MOBI, etc.
type Metadata struct {
	// Title is the main title of the e-book.
	Title string

	// Authors are the creators or writers of the book.
	Authors []string

	// Description is a brief summary or description of the book.
	Description string

	// Languages is the language(s) of the book.
	Languages []string

	// Cover is the cover image of the book.
	Cover *Resource

	// FileType indicates the original format from which this metadata was parsed.
	FileType FileType
}

type Resource struct {
	Id        string
	Name      string
	MediaType string
	Size      int
	Data      func() ([]byte, error)
}
