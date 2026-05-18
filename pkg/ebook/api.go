package ebook

import (
	"fmt"

	"faun.projects/margaret/margaret-ebook-library/internal/detector"
	"faun.projects/margaret/margaret-ebook-library/internal/reader/registry"
	"faun.projects/margaret/margaret-ebook-library/pkg/model"
)

func ReadMetadata(path string) (*model.Metadata, error) {
	fmt.Println("[api] ReadMetadata called")

	fileType := detector.Detect(path)
	fmt.Printf("[detect] detected format: %s\n", fileType)

	registry := registry.New()
	reader, err := registry.Get(fileType)
	if err != nil {
		return nil, err
	}
	return reader.ReadMetadata(path)
}
