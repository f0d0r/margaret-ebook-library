package tools

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"reflect"
	"strings"

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
	hashers *fingerprintHashers
}

func (h *FingerprintHandle) Result() FingerprintResult {
	if h.hashers == nil {
		return FingerprintResult{}
	}
	return h.hashers.result()
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

type fingerprintHashers struct {
	min *MinHasher
	sim *SimHasher
	w   io.Writer
}

func newFingerprintHashers(cfg fingerprintConfig) (*fingerprintHashers, error) {
	h := &fingerprintHashers{}
	var writers []io.Writer
	if cfg.minHash != nil {
		h.min = newMinHasherWithConfig(*cfg.minHash)
		writers = append(writers, h.min)
	}
	if cfg.simHash != nil {
		h.sim = NewSimHasher(WithSimShingleSize(cfg.simHash.shingleSize))
		writers = append(writers, h.sim)
	}
	if len(writers) == 0 {
		return nil, fmt.Errorf("no hasher requested")
	}
	if len(writers) == 1 {
		h.w = writers[0]
	} else {
		h.w = io.MultiWriter(writers...)
	}
	return h, nil
}

func (h *fingerprintHashers) result() FingerprintResult {
	var r FingerprintResult
	if h.min != nil {
		r.MinHash = h.min.Signature()
	}
	if h.sim != nil {
		r.SimHash = h.sim.Sum64()
		r.HasSimHash = true
	}
	return r
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

	hashers, err := newFingerprintHashers(cfg)
	if err != nil {
		return FingerprintResult{}, err
	}

	rc, err := openDelimitedReadingOrder(ctx, rs, mediatype.PlainText, cfg.delimiter)
	if err != nil {
		return FingerprintResult{}, err
	}
	defer func() { _ = rc.Close() }()

	tee := io.TeeReader(rc, hashers.w)
	if _, err := io.Copy(io.Discard, tee); err != nil {
		return FingerprintResult{}, err
	}
	return hashers.result(), nil
}

// FingerprintReader computes fingerprints over the normalized plain-text stream
// read from r. It streams r without buffering the whole input and does not close
// r. If no hasher option is supplied, both MinHash (128 hashes, shingle size 5)
// and SimHash (shingle size 5) are computed; only explicitly requested hashers
// are populated in the result.
//
// Input is treated as plain text: normalization is NFKC plus lowercasing,
// whitespace collapse and word shingles; no HTML conversion is performed. Empty
// input yields an all-max MinHash signature and a zero SimHash (HasSimHash is
// still true when SimHash is requested). Input with fewer words than the shingle
// size is fingerprinted as a single shingle.
func FingerprintReader(r io.Reader, opts ...FingerprintOption) (FingerprintResult, error) {
	if r == nil {
		return FingerprintResult{}, fmt.Errorf("reader is nil")
	}
	cfg := resolveFingerprintConfig(opts)

	hashers, err := newFingerprintHashers(cfg)
	if err != nil {
		return FingerprintResult{}, err
	}
	if _, err := io.Copy(io.Discard, io.TeeReader(r, hashers.w)); err != nil {
		return FingerprintResult{}, err
	}
	return hashers.result(), nil
}

// FingerprintText is a convenience wrapper over FingerprintReader for plain-text
// input. See FingerprintReader for normalization, defaults and empty/short-input
// behavior.
func FingerprintText(text string, opts ...FingerprintOption) (FingerprintResult, error) {
	return FingerprintReader(strings.NewReader(text), opts...)
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

	hashers, err := newFingerprintHashers(cfg)
	if err != nil {
		return nil, nil, err
	}

	rc, err := openDelimitedReadingOrder(ctx, rs, target, cfg.delimiter)
	if err != nil {
		return nil, nil, err
	}
	handle := &FingerprintHandle{hashers: hashers}
	frc := &fingerprintingReadCloser{rc: rc, tee: io.TeeReader(rc, hashers.w), handle: handle}
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
	if v.Kind() == reflect.Pointer && v.IsNil() {
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
