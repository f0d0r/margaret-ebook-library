package model

// Metadata represents the extracted core information of an e-book.
// It normalized data across different e-book formats like EPUB, MOBI, etc.
type Metadata struct {
	// Title is the main title of the e-book.
	Title string

	// Author is the creator or writer of the book. 
	// If multiple authors exist, they are typically joined by commas.
	Author string

	// FileType indicates the original format from which this metadata was parsed.
	FileType FileType
}
