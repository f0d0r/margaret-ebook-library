package compress

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"

	"github.com/f0d0r/margaret-ebook-library/book"
)

// DecompressZlib decompresses a raw zlib stream with bomb protection: output
// beyond remain bytes fails with book.ErrLimitExceeded. A negative remain
// disables the limit. Corrupt input fails with book.ErrCorrupt.
func DecompressZlib(data []byte, remain int64) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("compress: zlib invalid header: %w", book.ErrCorrupt)
	}
	defer func() { _ = r.Close() }()
	if remain < 0 {
		out, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("compress: zlib corrupt stream: %w", book.ErrCorrupt)
		}
		return out, nil
	}
	out, err := io.ReadAll(io.LimitReader(r, remain+1))
	if err != nil {
		return nil, fmt.Errorf("compress: zlib corrupt stream: %w", book.ErrCorrupt)
	}
	if int64(len(out)) > remain {
		return nil, book.ErrLimitExceeded
	}
	return out, nil
}
