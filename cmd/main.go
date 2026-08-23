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

	if book.Resources != nil {
		if cover, ok := book.Resources.CoverImage(); ok {
			fmt.Printf("Cover: %s (%d bytes) media type: %s href: %s\n", cover.Name, cover.Size, cover.MediaType, cover.ResolvedHref)
		}
	}

	if book.Resources != nil {
		fmt.Printf("\nResources: %d total, %d in reading order\n", book.Resources.Len(), len(book.Resources.ReadingOrder()))
		fmt.Printf("All resources (%d):\n", len(book.Resources.All()))
		for i, res := range book.Resources.All() {
			fmt.Printf("  [%d] %s (%s, %s, %d bytes) href=%s resolved=%s\n", i, res.Name, res.Id, res.MediaType, res.Size, res.Href, res.ResolvedHref)
		}
		fmt.Printf("\nReadingOrder (%d):\n", len(book.Resources.ReadingOrder()))
		for i, it := range book.Resources.ReadingOrder() {
			fmt.Printf("\n[%d] %s (%s, %s, %d bytes) linear=%v href=%s\n", i, it.Resource.Name, it.Resource.Id, it.Resource.MediaType, it.Resource.Size, it.Linear, it.Resource.ResolvedHref)
			if isTextMediaType(it.Resource.MediaType) {
				rc, err := it.Resource.Open()
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
