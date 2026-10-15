package sections

import (
	"fmt"
	"io"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/util"
)

// Open returns a reader over blob[offset:offset+size], enforcing the
// given safety limits: sizes over either maximum fail with ErrLimitExceeded,
// and the stream additionally surfaces an error if it yields more than the
// effective limit.
func Open(b book.Blob, offset, size, maxResourceSize, maxRecordSize int64, name string) (io.ReadCloser, error) {
	if maxResourceSize > 0 && size > maxResourceSize {
		return nil, book.LimitError(name, size, maxResourceSize)
	}
	if maxRecordSize > 0 && size > maxRecordSize {
		return nil, fmt.Errorf("%w: %q %d bytes exceeds MaxRecordSize %d", book.ErrLimitExceeded, name, size, maxRecordSize)
	}
	sr := io.NewSectionReader(b, offset, size)
	limit := maxResourceSize
	if maxRecordSize > 0 && (limit <= 0 || maxRecordSize < limit) {
		limit = maxRecordSize
	}
	if limit > 0 {
		return io.NopCloser(util.LimitReader(sr, limit)), nil
	}
	return io.NopCloser(sr), nil
}
