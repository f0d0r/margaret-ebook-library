package detector

import "fmt"

type FileType string

const (
	EPUB   FileType = "epub"
	MOBI   FileType = "mobi"
	UNKNOWN FileType = "unknown"
)

func Detect(path string) FileType {
	fmt.Printf("[detector] inspecting file: %s\n", path)

	// dummy logic
	return EPUB
}