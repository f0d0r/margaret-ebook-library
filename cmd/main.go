package main

import (
	"fmt"
	"os"
	"strings"

	"git.home/margaret/margaret-ebook-library/pkg/ebook"
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
	fmt.Printf("Languages: %v\n", strings.Join(bookMeta.Languages, ", "))

	if bookMeta.Cover != nil {
		fmt.Printf("Cover: %s (%d bytes) media type: %s\n", bookMeta.Cover.Name, bookMeta.Cover.Size, bookMeta.Cover.MediaType)

		/*coverBytes, err := bookMeta.Cover.Data()
		if err != nil {
			log.Fatalf("Hiba a borítókép adatainak beolvasásakor: %v", err)
		}
		outputPath := bookMeta.Cover.Name
		err = os.WriteFile(outputPath, coverBytes, 0644)
		if err != nil {
			log.Fatalf("Hiba a fájl mentésekor: %v", err)
		}
		fmt.Printf("Cover saved as: %s\n", outputPath)
		*/
	}
}
