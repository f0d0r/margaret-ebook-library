package ebook

import (
	"fmt"
	"strings"

	"faun.projects/margaret/margaret-ebook-library/internal/registry"
	"faun.projects/margaret/margaret-ebook-library/pkg/errs"
	"faun.projects/margaret/margaret-ebook-library/pkg/model"
)

// ReadMetadata reads ebook metadata from the specified file path.
//
// It trims leading and trailing whitespace from the provided path and rejects
// empty paths with [errs.ErrUnsupportedFormat]. It selects the appropriate
// reader implementation from the internal registry based on the file format.
// The selected reader is then used to extract and return the metadata.
//
// If no suitable reader is found for the format, it returns an error wrapping
// [errs.ErrUnsupportedFormat].
func ReadMetadata(path string) (model.Metadata, error) {
	fmt.Println("[api] ReadMetadata called")

	path = strings.TrimSpace(path)
	if path == "" {
		return model.Metadata{}, errs.ErrUnsupportedFormat
	}

	registry := registry.New()
	reader, err := registry.GetReader(path)
	if err != nil {
		return model.Metadata{}, fmt.Errorf("failed to get reader for %s: %w", path, err)
	}
	metadata, err := reader.ReadMetadata(path)
	if err != nil {
		return model.Metadata{}, fmt.Errorf("failed to read metadata for %s: %w", path, err)
	}
	return *metadata, nil
}
