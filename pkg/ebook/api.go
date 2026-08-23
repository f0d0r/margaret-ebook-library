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

// Read reads an ebook (metadata and resources) from the specified file path.
//
// It trims leading and trailing whitespace from the provided path and rejects
// empty paths with [errs.ErrUnsupportedFormat]. It selects the appropriate
// reader implementation from the internal registry based on the file format.
// The selected reader is then used to extract and return the ebook.
//
// An optional [Option] can be passed to override the library-wide safety
// limits; when omitted, the defaults of [model.DefaultConfig] are used.
//
// If no suitable reader is found for the format, it returns an error wrapping
// [errs.ErrUnsupportedFormat].
func Read(path string, opts ...Option) (model.Ebook, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return model.Ebook{}, errs.ErrUnsupportedFormat
	}
	return read(model.NewPathBlob(path), path, opts)
}

// ReadFromBlob reads an ebook (metadata and resources) from a random-access
// source such as a local file, an e-book entry inside an archive, or a remote
// source.
//
// It selects the appropriate reader implementation from the internal registry
// based on the file format detected purely from the contents. The selected
// reader is then used to extract and return the ebook.
//
// An optional [Option] can be passed to override the library-wide safety
// limits; when omitted, the defaults of [model.DefaultConfig] are used.
//
// The caller retains ownership of the blob and must keep it usable until
// all resource data (including the cover) has been consumed.
// The file position of the source is never modified.
//
// If no suitable reader is found for the format, it returns an error wrapping
// [errs.ErrUnsupportedFormat].
func ReadFromBlob(b model.Blob, opts ...Option) (model.Ebook, error) {
	return read(b, "", opts)
}

// ReadFromFile reads an ebook (metadata and resources) from an already-open
// file.
//
// It is a convenience wrapper around [ReadFromBlob]. The caller retains
// ownership of the file and must keep it open until all resource data
// (including the cover) has been consumed.
//
// An optional [Option] can be passed to override the library-wide safety
// limits; when omitted, the defaults of [model.DefaultConfig] are used.
func ReadFromFile(f *os.File, opts ...Option) (model.Ebook, error) {
	b, err := model.NewFileBlob(f)
	if err != nil {
		return model.Ebook{}, fmt.Errorf("failed to inspect file %s: %w", f.Name(), err)
	}
	return read(b, f.Name(), opts)
}

func read(b model.Blob, name string, opts []Option) (model.Ebook, error) {
	r := registry.New(resolveOptions(opts))
	reader, err := r.ReaderForBlob(b)
	if err != nil {
		return model.Ebook{}, fmt.Errorf("failed to get reader for %s: %w", name, err)
	}
	ebook, err := reader.Read(b)
	if err != nil {
		return model.Ebook{}, fmt.Errorf("failed to read ebook for %s: %w", name, err)
	}
	return *ebook, nil
}

// Config is a convenience alias for [model.Config], the library-wide safety
// limits used internally to configure the format readers.
type Config = model.Config

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
