package ebook

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/f0d0r/margaret-ebook-library/internal/registry"
	"github.com/f0d0r/margaret-ebook-library/pkg/errs"
	"github.com/f0d0r/margaret-ebook-library/pkg/model"
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
	return readMetadata(model.NewPathBlob(path), path)
}

// ReadMetadataFromBlob reads ebook metadata from a random-access source such
// as a local file, an e-book entry inside an archive, or a remote source.
//
// It selects the appropriate reader implementation from the internal registry
// based on the file format detected purely from the contents. The selected
// reader is then used to extract and return the metadata.
//
// The caller retains ownership of the blob and must keep it usable until the
// returned metadata (including any cover data) has been consumed. The file
// position of the source is never modified.
//
// If no suitable reader is found for the format, it returns an error wrapping
// [errs.ErrUnsupportedFormat].
func ReadMetadataFromBlob(b model.Blob) (model.Metadata, error) {
	return readMetadata(b, "")
}

// ReadMetadataFromFile reads ebook metadata from an already-open file.
//
// It is a convenience wrapper around [ReadMetadataFromBlob]. The caller
// retains ownership of the file and must keep it open until the returned
// metadata (including any cover data) has been consumed.
func ReadMetadataFromFile(f *os.File) (model.Metadata, error) {
	b, err := model.NewFileBlob(f)
	if err != nil {
		return model.Metadata{}, fmt.Errorf("failed to inspect file %s: %w", f.Name(), err)
	}
	return readMetadata(b, f.Name())
}

func readMetadata(b model.Blob, name string) (model.Metadata, error) {
	r := registry.New()
	reader, err := r.ReaderForBlob(b)
	if err != nil {
		return model.Metadata{}, fmt.Errorf("failed to get reader for %s: %w", name, err)
	}
	metadata, err := reader.ReadMetadata(b)
	if err != nil {
		return model.Metadata{}, fmt.Errorf("failed to read metadata for %s: %w", name, err)
	}
	return *metadata, nil
}

// CalculateFileHash calculates the sha256 hash of the file at the specified path.
func CalculateFileHash(path string) (string, error) {
	return CalculateFileHashFromBlob(model.NewPathBlob(path))
}

// CalculateFileHashFromBlob calculates the sha256 hash of a random-access
// source by reading it in chunks, so it works for local files as well as
// archive entries and remote sources without materializing a temporary file.
// It never modifies the source's position.
func CalculateFileHashFromBlob(b model.Blob) (string, error) {
	size, err := b.Size()
	if err != nil {
		return "", fmt.Errorf("failed to get file size: %w", err)
	}

	h := sha256.New()
	buf := make([]byte, 32*1024)
	for off := int64(0); off < size; {
		n, err := b.ReadAt(buf, off)
		if n > 0 {
			_, _ = h.Write(buf[:n])
		}
		off += int64(n)
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read: %w", err)
		}
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
