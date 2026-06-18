package ebook

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"git.home/margaret/margaret-ebook-library/internal/registry"
	"git.home/margaret/margaret-ebook-library/pkg/errs"
	"git.home/margaret/margaret-ebook-library/pkg/model"
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
	path = strings.TrimSpace(path)
	if path == "" {
		return model.Metadata{}, errs.ErrUnsupportedFormat
	}

	r := registry.New()
	reader, err := r.ReaderFor(path)
	if err != nil {
		return model.Metadata{}, fmt.Errorf("failed to get reader for %s: %w", path, err)
	}
	metadata, err := reader.ReadMetadata(path)
	if err != nil {
		return model.Metadata{}, fmt.Errorf("failed to read metadata for %s: %w", path, err)
	}
	return *metadata, nil
}

// CalculateFileHash calculates the sha256 hash of the file at the specified path.
func CalculateFileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = file.Close() }()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to calculate hash: %w", err)
	}
	hashInBytes := hash.Sum(nil)
	hashString := hex.EncodeToString(hashInBytes)
	return hashString, nil
}
