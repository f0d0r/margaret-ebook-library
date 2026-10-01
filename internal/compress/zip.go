// Package compress provides format-independent helpers for working with
// compressed data: ZIP archive handling (signature sniffing, opening a blob
// as an archive, locating entries by name, reading entries with
// decompression-bomb protection) and general-purpose decompressors (e.g.
// LZX) shared by the e-book format readers instead of reimplemented per
// format.
package compress

import (
	stdzip "archive/zip"
	"fmt"
	"io"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/util"
)

// IsZip reports whether the leading bytes look like a ZIP archive. Valid ZIP
// files can start with one of several PK headers (local file header, end of
// central directory, data descriptor).
func IsZip(head []byte) bool {
	if len(head) < 4 || head[0] != 0x50 || head[1] != 0x4B {
		return false
	}
	return (head[2] == 0x03 && head[3] == 0x04) ||
		(head[2] == 0x05 && head[3] == 0x06) ||
		(head[2] == 0x07 && head[3] == 0x08)
}

// Open builds a ZIP reader from the blob.
func Open(b book.Blob) (*stdzip.Reader, error) {
	size, err := b.Size()
	if err != nil {
		return nil, err
	}
	return stdzip.NewReader(b, size)
}

// Find locates an entry by its exact path within the archive, or returns nil.
func Find(zr *stdzip.Reader, name string) *stdzip.File {
	for _, file := range zr.File {
		if file.Name == name {
			return file
		}
	}
	return nil
}

// ReadHead decompresses at most max bytes of an entry. Longer entries are
// silently truncated, which is what content sniffing needs.
func ReadHead(f *stdzip.File, max int64) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()

	return io.ReadAll(io.LimitReader(rc, max))
}

// OpenLimited opens an entry as a streaming reader, refusing entries that
// declare more than max decompressed bytes. The returned stream additionally
// surfaces an error if the entry actually yields more than max bytes,
// protecting against zip-bomb style archives that lie about their size.
// max must be positive; callers resolve it from their configuration.
func OpenLimited(f *stdzip.File, max int64) (io.ReadCloser, error) {
	if f.UncompressedSize64 > uint64(max) {
		return nil, fmt.Errorf("%w: %q declares %d bytes (limit %d)", book.ErrLimitExceeded, f.Name, f.UncompressedSize64, max)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	return &readCloser{Reader: util.LimitReader(rc, max), Closer: rc}, nil
}

// readCloser combines a size-limited reader with the closer of the underlying
// zip entry so callers can close the underlying file after streaming.
type readCloser struct {
	io.Reader
	io.Closer
}
