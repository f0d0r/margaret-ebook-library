package main

import (
	"fmt"
	"os"
	"strings"

	"faun.projects/margaret/margaret-ebook-library/pkg/ebook"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Expected a file path after the command, e.g. ./margaret-ebook-library <ebook file>")
		return
	}

	path := os.Args[1]
	bookMeta, err := ebook.ReadMetadata(path)
	if err != nil {
		fmt.Println("Error reading ebook:", err)
		return
	}

	fmt.Printf("Book Title: %s\n", bookMeta.Title)
	fmt.Printf("Authors: %v\n", strings.Join(bookMeta.Authors, ", "))
	fmt.Printf("Description: %s\n", bookMeta.Description)
}
