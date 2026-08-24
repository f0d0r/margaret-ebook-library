// Package tools provides non-core utilities outside the classic ebook scope:
// file hashing and streaming content fingerprints (MinHash, SimHash) that
// operate on the normalized plain-text reading order via io.Writer/TeeReader.
package tools

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/f0d0r/margaret-ebook-library/book"
)

// CalculateFileHash calculates the sha256 hash of the file at the specified path.
func CalculateFileHash(path string) (string, error) {
	return CalculateFileHashFromBlob(book.NewPathBlob(path))
}

// CalculateFileHashFromBlob calculates the sha256 hash of a random-access
// source by reading it in chunks, so it works for local files as well as
// archive entries and remote sources without materializing a temporary file.
// It never modifies the source's position.
func CalculateFileHashFromBlob(b book.Blob) (string, error) {
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
