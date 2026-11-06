package tools

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/converter"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

const defaultISBNScanBytes = 64 * 1024

type isbnScanConfig struct {
	maxBytes int
	limit    int
}

// ISBNScanOption configures FindBookISBNs.
type ISBNScanOption func(*isbnScanConfig)

// WithISBNScanBytes sets how many bytes of plain text are inspected in the
// linear reading order and in each non-linear or manifest-only textual
// resource. Values <= 0 keep the default (64 KiB).
func WithISBNScanBytes(n int) ISBNScanOption {
	return func(c *isbnScanConfig) {
		if n > 0 {
			c.maxBytes = n
		}
	}
}

// WithISBNLimit caps the number of unique ISBNs returned. Values <= 0 mean no
// limit.
func WithISBNLimit(n int) ISBNScanOption {
	return func(c *isbnScanConfig) {
		if n > 0 {
			c.limit = n
		}
	}
}

// FindBookISBN scans a book's reading order and returns the first book-valid
// ISBN. It reports false when no book-valid ISBN is found.
func FindBookISBN(ctx context.Context, b book.Book) (ISBN, bool, error) {
	found, err := FindBookISBNs(ctx, b, WithISBNLimit(1))
	if err != nil || len(found) == 0 {
		return ISBN{}, false, err
	}
	return found[0], true, nil
}

// FindBookISBNs returns the book-valid ISBNs found in the book's text
// resources. It runs an ISBNProcessor through ScanReadingOrder: the head of the
// linear reading order is scanned first, then the head of each non-linear spine
// item, then the head of each manifest-only textual resource. Unreadable
// optional resources are skipped; context cancellation is returned.
func FindBookISBNs(ctx context.Context, b book.Book, opts ...ISBNScanOption) ([]ISBN, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if isNilBook(b) {
		return nil, fmt.Errorf("book is nil")
	}
	rs := b.Resources()
	if rs == nil {
		return nil, fmt.Errorf("resource set is nil")
	}
	cfg := isbnScanConfig{maxBytes: defaultISBNScanBytes}
	for _, opt := range opts {
		opt(&cfg)
	}
	proc := NewISBNProcessor(cfg.maxBytes, cfg.limit)
	if err := ScanReadingOrder(ctx, rs, proc); err != nil {
		return nil, err
	}
	return proc.Result(), nil
}

func readResourceHead(ctx context.Context, res *book.Resource, limit int) (string, error) {
	rc, err := res.OpenAsWithCharset(ctx, mediatype.PlainText, book.CharsetUTF8)
	if err != nil {
		return "", err
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(io.LimitReader(rc, int64(limit)))
	if err != nil {
		return "", err
	}
	return strings.ToValidUTF8(string(data), ""), nil
}

func canConvertToText(mime string) bool {
	_, ok := converter.DefaultRegistry.FindPath(mime, mediatype.PlainText)
	return ok
}
