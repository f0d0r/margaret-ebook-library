package mobi

import (
	"fmt"

	"faun.projects/margaret/margaret-ebook-library/internal/detector"
	"faun.projects/margaret/margaret-ebook-library/pkg/model"
)

type MOBIReader struct{}

func (r *MOBIReader) Supports(fileType detector.FileType) bool {
	return fileType == detector.MOBI
}

func (r *MOBIReader) ReadMetadata(path string) (*model.Metadata, error) {
	fmt.Printf("[mobi-reader] reading metadata from: %s\n", path)

	return &model.Metadata{
		Title: "Dummy MOBI Book",
	}, nil
}
