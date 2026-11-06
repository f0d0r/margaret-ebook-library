package tools

import (
	"context"
	"strings"

	"github.com/f0d0r/margaret-ebook-library/book"
)

// FingerprintProcessor feeds the plain-text reading order into the requested
// fingerprint hashers. Result must be called after ScanReadingOrder returns.
type FingerprintProcessor struct {
	hashers *fingerprintHashers
}

// NewFingerprintProcessor builds a processor for the fingerprint options.
// With no options, both MinHash and SimHash are enabled.
func NewFingerprintProcessor(opts ...FingerprintOption) (*FingerprintProcessor, error) {
	cfg := resolveFingerprintConfig(opts)
	hashers, err := newFingerprintHashers(cfg)
	if err != nil {
		return nil, err
	}
	return &FingerprintProcessor{hashers: hashers}, nil
}

// Process implements TextProcessor.
func (p *FingerprintProcessor) Process(chunk []byte) error {
	_, err := p.hashers.w.Write(chunk)
	return err
}

// Result returns the computed fingerprints.
func (p *FingerprintProcessor) Result() FingerprintResult {
	return p.hashers.result()
}

// TextHead captures the beginning of the reading order as UTF-8 text. It
// implements HeadProcessor.
type TextHead struct {
	limit int
	text  string
}

// NewTextHead creates a processor that keeps at most limit bytes.
func NewTextHead(limit int) *TextHead {
	return &TextHead{limit: limit}
}

// HeadLimit implements HeadProcessor.
func (p *TextHead) HeadLimit() int { return p.limit }

// ProcessHead implements HeadProcessor.
func (p *TextHead) ProcessHead(head []byte) error {
	if p.limit > 0 && len(head) > p.limit {
		head = head[:p.limit]
	}
	p.text = strings.ToValidUTF8(string(head), "")
	return nil
}

// Text returns the captured head.
func (p *TextHead) Text() string { return p.text }

// ISBNProcessor extracts at most limit book-valid ISBNs from the reading-order
// head and, when the head has none, from non-linear and manifest-only textual
// resources. It implements HeadProcessor and FallbackProcessor. Results are
// available after ScanReadingOrder returns.
type ISBNProcessor struct {
	maxBytes int
	limit    int
	out      []ISBN
	seen     map[string]struct{}
}

// NewISBNProcessor creates an ISBN processor. maxBytes <= 0 uses the default
// 64 KiB head; limit <= 0 means no limit.
func NewISBNProcessor(maxBytes, limit int) *ISBNProcessor {
	if maxBytes <= 0 {
		maxBytes = defaultISBNScanBytes
	}
	return &ISBNProcessor{
		maxBytes: maxBytes,
		limit:    limit,
		seen:     make(map[string]struct{}),
	}
}

// HeadLimit implements HeadProcessor.
func (p *ISBNProcessor) HeadLimit() int { return p.maxBytes }

// ProcessHead implements HeadProcessor.
func (p *ISBNProcessor) ProcessHead(head []byte) error {
	if len(head) > p.maxBytes {
		head = head[:p.maxBytes]
	}
	p.add(FindISBNs(strings.ToValidUTF8(string(head), "")))
	return nil
}

// NeedsFallback implements FallbackProcessor.
func (p *ISBNProcessor) NeedsFallback() bool { return !p.done() }

// ProcessFallback implements FallbackProcessor. It scans non-linear spine
// items and then the remaining manifest-only textual resources.
func (p *ISBNProcessor) ProcessFallback(ctx context.Context, rs *book.ResourceSet, linear map[*book.Resource]struct{}) error {
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
		text, err := readResourceHead(ctx, res, p.maxBytes)
		if err != nil {
			if ctx.Err() != nil {
				return false, ctx.Err()
			}
			return false, nil
		}
		p.add(FindISBNs(text))
		return p.done(), nil
	}

	for _, item := range rs.ReadingOrder() {
		if item.Linear {
			continue
		}
		if done, err := scan(item.Resource); err != nil {
			return err
		} else if done {
			return nil
		}
	}
	for _, res := range rs.All() {
		if done, err := scan(res); err != nil {
			return err
		} else if done {
			return nil
		}
	}
	return nil
}

// Result returns the unique ISBNs found so far.
func (p *ISBNProcessor) Result() []ISBN {
	return append([]ISBN(nil), p.out...)
}

func (p *ISBNProcessor) add(found []ISBN) {
	for _, isbn := range found {
		isbn13, ok := isbn.ToISBN13()
		if !ok {
			continue
		}
		key := isbn13.String()
		if _, exists := p.seen[key]; exists {
			continue
		}
		p.seen[key] = struct{}{}
		p.out = append(p.out, isbn)
		if p.done() {
			return
		}
	}
}

func (p *ISBNProcessor) done() bool {
	return p.limit > 0 && len(p.out) >= p.limit
}
