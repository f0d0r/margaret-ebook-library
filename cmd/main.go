package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/f0d0r/margaret-ebook-library/pkg/ebook"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Expected a file path after the command, e.g. ./margaret-ebook-library <ebook file>")
		return
	}

	path := os.Args[1]
	book, err := ebook.Read(path)
	if err != nil {
		fmt.Println("Error reading ebook:", err)
		return
	}

	bookMeta := book.Metadata

	fmt.Printf("Book Title: %s\n", bookMeta.Title)
	fmt.Printf("Authors: %v\n", strings.Join(bookMeta.Authors, ", "))
	fmt.Printf("Description: %s\n", bookMeta.Description)
	fmt.Printf("Languages: %v\n", strings.Join(bookMeta.Languages, ", "))
	fmt.Printf("File Type: %s\n", book.FileType)
	fmt.Printf("Version: %s\n", book.Version)

	if bookMeta.Cover != nil {
		fmt.Printf("Cover: %s (%d bytes) media type: %s\n", bookMeta.Cover.Name, bookMeta.Cover.Size, bookMeta.Cover.MediaType)
	}

	fmt.Printf("\nContent (%d resources):\n", len(book.Content))
	for i, res := range book.Content {
		fmt.Printf("\n[%d] %s (%s, %s, %d bytes)\n", i, res.Name, res.Id, res.MediaType, res.Size)

		if isTextMediaType(res.MediaType) {
			rc, err := res.Open()
			if err != nil {
				fmt.Printf("  error opening: %v\n", err)
				continue
			}
			data, err := io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				fmt.Printf("  error reading: %v\n", err)
				continue
			}
			fmt.Printf("  %s\n", data)
		}
	}
}

// isTextMediaType reports whether a media type is text-based and thus worth
// printing to the console.
func isTextMediaType(mediaType string) bool {
	switch mediaType {
	case "application/xhtml+xml", "application/x-mobipocket-html", "text/html", "text/plain", "text/xml",
		"application/xml", "text/css", "text/javascript":
		return true
	}
	return false
}
