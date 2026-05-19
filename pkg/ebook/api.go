package ebook

import (
	"fmt"
	"strings"

	"faun.projects/margaret/margaret-ebook-library/internal/reader/registry"
	"faun.projects/margaret/margaret-ebook-library/pkg/errs"
	"faun.projects/margaret/margaret-ebook-library/pkg/model"
)

func ReadMetadata(path string) (*model.Metadata, error) {
	fmt.Println("[api] ReadMetadata called")

	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errs.ErrUnsupportedFormat
	}

	registry := registry.New()
	reader, err := registry.Get(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get reader for %s: %w", path, err)
	}
	return reader.ReadMetadata(path)
}
