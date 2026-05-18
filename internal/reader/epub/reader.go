package epub

import (
	"fmt"

	"faun.projects/margaret/margaret-ebook-library/internal/detector"
	"faun.projects/margaret/margaret-ebook-library/pkg/model"
)

type EPUBReader struct{}

func (r *EPUBReader) Supports(fileType detector.FileType) bool {
	return fileType == detector.EPUB
}

func (r *EPUBReader) ReadMetadata(path string) (*model.Metadata, error) {
	fmt.Printf("[epub-reader] reading metadata from: %s\n", path)

	return &model.Metadata{
		Title: "Dummy EPUB Book",
	}, nil
}
