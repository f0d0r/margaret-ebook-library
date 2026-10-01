package lit

import (
	"fmt"
	"io"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
)

// litMagic is the 8-byte LIT signature at offset 0 ("ITOLITLS").
// See litheaders.c: lit_magic_string in the convertlit reference.
const litMagic = "ITOLITLS"

// minLitSize is the primary header size (PRI_SIZE) from litheaders.c.
// lit_i_read_headers rejects anything smaller, so Supports does too.
const minLitSize = 40

type LitReader struct {
	cfg config.Config
}

// NewLitReader creates a new LIT reader instance with the given
// safety limits. A zero Config is normalized to [config.DefaultConfig] so
// that a hand-built struct cannot silently disable the limits.
func NewLitReader(cfg config.Config) *LitReader {
	return &LitReader{cfg: cfg.Normalize()}
}

// Supports reports whether b looks like a LIT file. It checks only the
// 8-byte "ITOLITLS" magic at offset 0 (plus the 40-byte minimum size), so
// detection is a single Size call and a single 8-byte ReadAt with no
// decompression. Stricter validation (version, header lengths) is left to
// Read.
func (r *LitReader) Supports(b book.Blob) bool {
	size, err := b.Size()
	if err != nil || size < minLitSize {
		return false
	}

	var buf [8]byte
	n, err := b.ReadAt(buf[:], 0)
	if err != nil && err != io.EOF {
		return false
	}
	if n < len(buf) {
		return false
	}

	return string(buf[:]) == litMagic
}

// Read is not yet implemented; it currently reports a parse failure so
// callers get an explicit error through the normal API.
func (r *LitReader) Read(b book.Blob) (book.Book, error) {
	return nil, fmt.Errorf("lit Read not yet implemented: %w", book.ErrParseFailed)
}
