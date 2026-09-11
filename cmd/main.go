package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	ebook "github.com/f0d0r/margaret-ebook-library"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
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

	bookMeta := book.Metadata()

	fmt.Printf("Book Title: %s\n", bookMeta.Title)
	fmt.Printf("Authors: %v\n", strings.Join(bookMeta.Authors, ", "))
	fmt.Printf("Description: %s\n", bookMeta.Description)
	fmt.Printf("Languages: %v\n", strings.Join(bookMeta.Languages, ", "))
	fmt.Printf("File Type: %s\n", book.FileType())
	fmt.Printf("Version: %s\n", book.Version())

	if book.Resources() != nil {
		if cover, ok := book.Resources().CoverImage(); ok {
			fmt.Printf("Cover: %s (%d bytes) media type: %s href: %s\n", cover.Name, cover.Size, cover.MediaType, cover.ResolvedHref)
		}
	}

	if book.Resources() != nil {
		fmt.Printf("\nResources: %d total, %d in reading order\n", book.Resources().Len(), len(book.Resources().ReadingOrder()))
		fmt.Printf("All resources (%d):\n", len(book.Resources().All()))
		for i, res := range book.Resources().All() {
			fmt.Printf("  [%d] %s (%s, %s, %d bytes) href=%s resolved=%s\n", i, res.Name, res.Id, res.MediaType, res.Size, res.Href, res.ResolvedHref)
		}
		fmt.Printf("\nReadingOrder (%d):\n", len(book.Resources().ReadingOrder()))
		r, err := book.Resources().OpenReadingOrderAs(context.Background(), mediatype.PlainText)
		if err != nil {
			fmt.Printf("Error opening reading order as plain text: %v\n", err)
			return
		}
		defer func() { _ = r.Close() }()
		plainText, err := io.ReadAll(r)
		if err != nil {
			fmt.Printf("Error reading plain text from reading order: %v\n", err)
			return
		}
		fmt.Println(string(plainText))
	}
}
