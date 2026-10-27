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

// FindBookISBNs returns the validated ISBNs found in the book's text
// resources. It scans the head of the linear reading order first, then the
// head of each non-linear spine item, then the head of each manifest-only
// textual resource. Unreadable optional resources are skipped; context
// cancellation is returned.
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

	out := make([]ISBN, 0, 1)
	seen := make(map[string]struct{})
	add := func(found []ISBN) bool {
		for _, isbn := range found {
			isbn13, ok := isbn.ToISBN13()
			if !ok {
				continue
			}
			key := isbn13.String()
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, isbn)
			if cfg.limit > 0 && len(out) >= cfg.limit {
				return true
			}
		}
		return false
	}

	head, err := readReadingOrderHead(ctx, rs, cfg.maxBytes)
	if err != nil {
		return nil, err
	}
	if add(ExtractISBNs(head)) {
		return out, nil
	}

	linear := make(map[*book.Resource]struct{})
	for _, item := range rs.ReadingOrder() {
		if item.Linear && item.Resource != nil {
			linear[item.Resource] = struct{}{}
		}
	}
	scanned := make(map[*book.Resource]struct{})
	scan := func(res *book.Resource) (bool, error) {
		if res == nil {
			return false, nil
		}
		if _, ok := linear[res]; ok {
			return false, nil
		}
		if _, ok := scanned[res]; ok {
			return false, nil
		}
		scanned[res] = struct{}{}
		if !canConvertToText(res.MediaType) {
			return false, nil
		}
		text, err := readResourceHead(ctx, res, cfg.maxBytes)
		if err != nil {
			if ctx.Err() != nil {
				return false, ctx.Err()
			}
			return false, nil
		}
		return add(ExtractISBNs(text)), nil
	}

	for _, item := range rs.ReadingOrder() {
		if item.Linear {
			continue
		}
		done, err := scan(item.Resource)
		if err != nil {
			return nil, err
		}
		if done {
			return out, nil
		}
	}
	for _, res := range rs.All() {
		done, err := scan(res)
		if err != nil {
			return nil, err
		}
		if done {
			return out, nil
		}
	}
	return out, nil
}

func readReadingOrderHead(ctx context.Context, rs *book.ResourceSet, limit int) (string, error) {
	rc, err := openDelimitedReadingOrder(ctx, rs, mediatype.PlainText, "\n")
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
