package integration

import (
    "testing"
    "faun.projects/margaret/margaret-ebook-library/pkg/ebook/epub"
)

func TestRead(t *testing.T) {
    book, err := epub.Read("example.epub")
    if err != nil {
        t.Errorf("Error reading ebook: %v", err)
    }
    if book.Title == "" {
        t.Errorf("Expected a title, got empty string")
    }
}
