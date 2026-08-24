package tools

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"reflect"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/converter"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

// FingerprintResult holds content fingerprints. Fields are populated only
// for the hashers requested via options.
type FingerprintResult struct {
	MinHash    []uint64 // nil if MinHash not requested
	SimHash    uint64
	HasSimHash bool
}

// FingerprintHandle is returned by OpenReadingOrderWithFingerprint.
// It holds the hashers that are fed via the returned ReadCloser.
// Call Result() after the stream has been consumed (EOF) and closed.
type FingerprintHandle struct {
	minHasher *MinHasher
	simHasher *SimHasher
}

func (h *FingerprintHandle) Result() FingerprintResult {
	var r FingerprintResult
	if h.minHasher != nil {
		r.MinHash = h.minHasher.Signature()
	}
	if h.simHasher != nil {
		r.SimHash = h.simHasher.Sum64()
		r.HasSimHash = true
	}
	return r
}

type fingerprintConfig struct {
	minHash   *MinHashConfig
	simHash   *simHashConfig
	delimiter string
}

type simHashConfig struct {
	shingleSize int
}

// FingerprintOption configures fingerprinting.
type FingerprintOption func(*fingerprintConfig)

// WithMinHash enables MinHash with the given config. Zero values fall back to defaults (128, 5).
func WithMinHash(cfg MinHashConfig) FingerprintOption {
	return func(c *fingerprintConfig) {
		cc := cfg
		if cc.NumHashes <= 0 {
			cc.NumHashes = 128
		}
		if cc.ShingleSize <= 0 {
			cc.ShingleSize = 5
		}
		c.minHash = &cc
	}
}

// WithSimHash enables SimHash with default shingle size 5.
func WithSimHash() FingerprintOption {
	return func(c *fingerprintConfig) {
		c.simHash = &simHashConfig{shingleSize: 5}
	}
}

// WithSimHashShingleSize enables SimHash with a custom shingle size.
func WithSimHashShingleSize(k int) FingerprintOption {
	return func(c *fingerprintConfig) {
		if k <= 0 {
			k = 5
		}
		c.simHash = &simHashConfig{shingleSize: k}
	}
}

func resolveFingerprintConfig(opts []FingerprintOption) fingerprintConfig {
	cfg := fingerprintConfig{delimiter: "\n"}
	for _, o := range opts {
		o(&cfg)
	}
	// If neither hasher requested, default to both (helpful for simple callers).
	if cfg.minHash == nil && cfg.simHash == nil {
		cfg.minHash = &MinHashConfig{NumHashes: 128, ShingleSize: 5}
		cfg.simHash = &simHashConfig{shingleSize: 5}
	}
	return cfg
}

// FingerprintContent computes fingerprints over the book's plain-text reading order.
// It injects "\n" between spine chapters and streams with a single pass per
// requested hasher (fan-out via MultiWriter). The book's resources are reopened
// lazily; no extra buffering of the whole book is performed.
func FingerprintContent(ctx context.Context, b book.Book, opts ...FingerprintOption) (FingerprintResult, error) {
	if isNilBook(b) {
		return FingerprintResult{}, fmt.Errorf("book is nil")
	}
	rs := b.Resources()
	if rs == nil {
		return FingerprintResult{}, fmt.Errorf("resource set is nil")
	}
	cfg := resolveFingerprintConfig(opts)

	var minHasher *MinHasher
	var simHasher *SimHasher
	var writers []io.Writer
	if cfg.minHash != nil {
		minHasher = newMinHasherWithConfig(*cfg.minHash)
		writers = append(writers, minHasher)
	}
	if cfg.simHash != nil {
		simHasher = NewSimHasher(WithSimShingleSize(cfg.simHash.shingleSize))
		writers = append(writers, simHasher)
	}
	if len(writers) == 0 {
		return FingerprintResult{}, fmt.Errorf("no hasher requested")
	}
	var mw io.Writer
	if len(writers) == 1 {
		mw = writers[0]
	} else {
		mw = io.MultiWriter(writers...)
	}

	rc, err := openDelimitedReadingOrder(ctx, rs, mediatype.PlainText, cfg.delimiter)
	if err != nil {
		return FingerprintResult{}, err
	}
	defer func() { _ = rc.Close() }()

	tee := io.TeeReader(rc, mw)
	if _, err := io.Copy(io.Discard, tee); err != nil {
		return FingerprintResult{}, err
	}

	var res FingerprintResult
	if minHasher != nil {
		res.MinHash = minHasher.Signature()
	}
	if simHasher != nil {
		res.SimHash = simHasher.Sum64()
		res.HasSimHash = true
	}
	return res, nil
}

// OpenReadingOrderWithFingerprint returns a ReadCloser over the reading order
// plain-text stream that simultaneously feeds the requested fingerprint hashers.
// The caller must consume the stream (e.g. io.Copy) and then call handle.Result()
// to obtain the fingerprints. A "\n" delimiter is injected between chapters.
//
// The returned ReadCloser must be closed by the caller.
func OpenReadingOrderWithFingerprint(ctx context.Context, rs *book.ResourceSet, target string, opts ...FingerprintOption) (io.ReadCloser, *FingerprintHandle, error) {
	if rs == nil {
		return nil, nil, fmt.Errorf("resource set is nil")
	}
	cfg := resolveFingerprintConfig(opts)

	var minHasher *MinHasher
	var simHasher *SimHasher
	var writers []io.Writer
	if cfg.minHash != nil {
		minHasher = newMinHasherWithConfig(*cfg.minHash)
		writers = append(writers, minHasher)
	}
	if cfg.simHash != nil {
		simHasher = NewSimHasher(WithSimShingleSize(cfg.simHash.shingleSize))
		writers = append(writers, simHasher)
	}
	// If for some reason no writer (should not happen due to default), just open raw.
	if len(writers) == 0 {
		rc, err := openDelimitedReadingOrder(ctx, rs, target, cfg.delimiter)
		if err != nil {
			return nil, nil, err
		}
		return rc, &FingerprintHandle{}, nil
	}
	var mw io.Writer
	if len(writers) == 1 {
		mw = writers[0]
	} else {
		mw = io.MultiWriter(writers...)
	}

	rc, err := openDelimitedReadingOrder(ctx, rs, target, cfg.delimiter)
	if err != nil {
		return nil, nil, err
	}
	handle := &FingerprintHandle{minHasher: minHasher, simHasher: simHasher}
	frc := &fingerprintingReadCloser{rc: rc, tee: io.TeeReader(rc, mw), handle: handle}
	return frc, handle, nil
}

type fingerprintingReadCloser struct {
	rc     io.ReadCloser
	tee    io.Reader
	handle *FingerprintHandle
}

func (f *fingerprintingReadCloser) Read(p []byte) (int, error) {
	return f.tee.Read(p)
}

func (f *fingerprintingReadCloser) Close() error {
	return f.rc.Close()
}

func isNilBook(b book.Book) bool {
	if b == nil {
		return true
	}
	v := reflect.ValueOf(b)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return true
	}
	return false
}

// openDelimitedReadingOrder is like book.ResourceSet.OpenReadingOrderAs but
// injects a delimiter between linear spine items.
func openDelimitedReadingOrder(ctx context.Context, rs *book.ResourceSet, target string, delimiter string) (io.ReadCloser, error) {
	to := mediatype.Normalize(target)
	if to == "" {
		return nil, fmt.Errorf("target MIME is empty")
	}
	var linear []*book.Resource
	for _, it := range rs.ReadingOrder() {
		if it.Linear && it.Resource != nil {
			linear = append(linear, it.Resource)
		}
	}
	if len(linear) == 0 {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}
	return &delimitedMultiReadCloser{
		ctx:       ctx,
		resources: linear,
		target:    to,
		delimiter: delimiter,
		reg:       converter.DefaultRegistry,
		index:     0,
	}, nil
}

type delimitedMultiReadCloser struct {
	ctx              context.Context
	resources        []*book.Resource
	target           string
	delimiter        string
	reg              *converter.Registry
	index            int
	current          io.ReadCloser
	delimiterPending []byte
	delimiterPos     int
	closed           bool
}

func (m *delimitedMultiReadCloser) Read(p []byte) (int, error) {
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	if len(p) == 0 {
		return 0, nil
	}
	for {
		if len(m.delimiterPending) > 0 {
			n := copy(p, m.delimiterPending[m.delimiterPos:])
			m.delimiterPos += n
			if m.delimiterPos >= len(m.delimiterPending) {
				m.delimiterPending = nil
				m.delimiterPos = 0
			}
			if n > 0 {
				return n, nil
			}
		}
		if m.current == nil {
			if m.index >= len(m.resources) {
				return 0, io.EOF
			}
			rc, err := m.resources[m.index].OpenAsWithRegistry(m.ctx, m.reg, m.target)
			if err != nil {
				return 0, err
			}
			m.current = rc
		}
		n, err := m.current.Read(p)
		if err == io.EOF {
			_ = m.current.Close()
			m.current = nil
			m.index++
			if m.index < len(m.resources) && m.delimiter != "" {
				m.delimiterPending = []byte(m.delimiter)
				m.delimiterPos = 0
			}
			if n > 0 {
				return n, nil
			}
			continue
		}
		return n, err
	}
}

func (m *delimitedMultiReadCloser) Close() error {
	if m.closed {
		return nil
	}
	m.closed = true
	if m.current != nil {
		err := m.current.Close()
		m.current = nil
		return err
	}
	return nil
}
