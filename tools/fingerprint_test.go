package tools

import (
	"errors"
	"io"
	"strings"
	"testing"
)

const fingerprintSample = "The quick brown fox jumps over the lazy dog"

func TestFingerprintText_Defaults(t *testing.T) {
	fp, err := FingerprintText(fingerprintSample)
	if err != nil {
		t.Fatalf("FingerprintText failed: %v", err)
	}
	if len(fp.MinHash) != 128 {
		t.Fatalf("MinHash len %d want 128", len(fp.MinHash))
	}
	if !fp.HasSimHash {
		t.Fatalf("HasSimHash should be true")
	}
}

func TestFingerprintText_OnlyMinHash(t *testing.T) {
	fp, err := FingerprintText(fingerprintSample, WithMinHash(MinHashConfig{NumHashes: 16, ShingleSize: 2}))
	if err != nil {
		t.Fatalf("FingerprintText failed: %v", err)
	}
	if len(fp.MinHash) != 16 {
		t.Fatalf("MinHash len %d want 16", len(fp.MinHash))
	}
	if fp.HasSimHash || fp.SimHash != 0 {
		t.Fatalf("SimHash should not be populated, got %#x (HasSimHash=%v)", fp.SimHash, fp.HasSimHash)
	}
}

func TestFingerprintText_OnlySimHash(t *testing.T) {
	fp, err := FingerprintText(fingerprintSample, WithSimHashShingleSize(3))
	if err != nil {
		t.Fatalf("FingerprintText failed: %v", err)
	}
	if fp.MinHash != nil {
		t.Fatalf("MinHash should be nil, got len %d", len(fp.MinHash))
	}
	if !fp.HasSimHash {
		t.Fatalf("HasSimHash should be true")
	}
}

func TestFingerprintText_MatchesHashers(t *testing.T) {
	mh := NewMinHasher(WithShingleSize(3), WithNumHashes(16))
	sh := NewSimHasher(WithSimShingleSize(3))
	mustWrite(t, mh, []byte(fingerprintSample))
	mustWrite(t, sh, []byte(fingerprintSample))

	fp, err := FingerprintText(fingerprintSample,
		WithMinHash(MinHashConfig{NumHashes: 16, ShingleSize: 3}),
		WithSimHashShingleSize(3),
	)
	if err != nil {
		t.Fatalf("FingerprintText failed: %v", err)
	}
	if !equalSigs(fp.MinHash, mh.Signature()) {
		t.Fatalf("MinHash mismatch: got %v want %v", fp.MinHash, mh.Signature())
	}
	if fp.SimHash != sh.Sum64() {
		t.Fatalf("SimHash mismatch: got %#x want %#x", fp.SimHash, sh.Sum64())
	}
}

func TestFingerprintReader_StreamingMatchesText(t *testing.T) {
	opts := []FingerprintOption{
		WithMinHash(MinHashConfig{NumHashes: 16, ShingleSize: 2}),
		WithSimHash(),
	}
	want, err := FingerprintText(fingerprintSample, opts...)
	if err != nil {
		t.Fatalf("FingerprintText failed: %v", err)
	}
	got, err := FingerprintReader(&chunkReader{data: []byte(fingerprintSample), chunk: 3}, opts...)
	if err != nil {
		t.Fatalf("FingerprintReader failed: %v", err)
	}
	if !equalSigs(got.MinHash, want.MinHash) {
		t.Fatalf("MinHash mismatch: got %v want %v", got.MinHash, want.MinHash)
	}
	if got.SimHash != want.SimHash || got.HasSimHash != want.HasSimHash {
		t.Fatalf("SimHash mismatch: got (%#x, %v) want (%#x, %v)",
			got.SimHash, got.HasSimHash, want.SimHash, want.HasSimHash)
	}
}

func TestFingerprintReader_Nil(t *testing.T) {
	if _, err := FingerprintReader(nil); err == nil {
		t.Fatalf("nil reader should error")
	}
}

func TestFingerprintReader_PropagatesError(t *testing.T) {
	errBoom := errors.New("boom")
	if _, err := FingerprintReader(errorReader{errBoom}); !errors.Is(err, errBoom) {
		t.Fatalf("got %v, want %v", err, errBoom)
	}
}

func TestFingerprintText_Empty(t *testing.T) {
	fp, err := FingerprintText("", WithMinHash(MinHashConfig{NumHashes: 8}), WithSimHash())
	if err != nil {
		t.Fatalf("FingerprintText failed: %v", err)
	}
	if len(fp.MinHash) != 8 {
		t.Fatalf("MinHash len %d want 8", len(fp.MinHash))
	}
	for i, v := range fp.MinHash {
		if v != ^uint64(0) {
			t.Fatalf("empty MinHash[%d] = %#x, want max", i, v)
		}
	}
	if fp.SimHash != 0 || !fp.HasSimHash {
		t.Fatalf("empty SimHash = (%#x, %v), want (0, true)", fp.SimHash, fp.HasSimHash)
	}
}

func TestFingerprintText_ShortInput(t *testing.T) {
	text := "hello world"

	mh := NewMinHasher()
	mustWrite(t, mh, []byte(text))
	sh := NewSimHasher()
	mustWrite(t, sh, []byte(text))

	fp, err := FingerprintText(text)
	if err != nil {
		t.Fatalf("FingerprintText failed: %v", err)
	}
	if !equalSigs(fp.MinHash, mh.Signature()) {
		t.Fatalf("short MinHash mismatch: got %v want %v", fp.MinHash, mh.Signature())
	}
	if fp.SimHash != sh.Sum64() {
		t.Fatalf("short SimHash mismatch: got %#x want %#x", fp.SimHash, sh.Sum64())
	}
}

func TestFingerprintText_Normalizes(t *testing.T) {
	// NFKC full-width letters, mixed case and irregular whitespace collapse.
	raw := "Ｆｏｏ \t BAR\n\nbaz"
	normalized := "foo bar baz"

	rawFP, err := FingerprintText(raw, WithMinHash(MinHashConfig{NumHashes: 8, ShingleSize: 2}), WithSimHash())
	if err != nil {
		t.Fatalf("FingerprintText(raw) failed: %v", err)
	}
	normFP, err := FingerprintText(normalized, WithMinHash(MinHashConfig{NumHashes: 8, ShingleSize: 2}), WithSimHash())
	if err != nil {
		t.Fatalf("FingerprintText(normalized) failed: %v", err)
	}
	if !equalSigs(rawFP.MinHash, normFP.MinHash) {
		t.Fatalf("MinHash mismatch: raw %v normalized %v", rawFP.MinHash, normFP.MinHash)
	}
	if rawFP.SimHash != normFP.SimHash {
		t.Fatalf("SimHash mismatch: raw %#x normalized %#x", rawFP.SimHash, normFP.SimHash)
	}
}

func TestFingerprintReader_DoesNotClose(t *testing.T) {
	r := &trackingReadCloser{Reader: strings.NewReader(fingerprintSample)}
	if _, err := FingerprintReader(r); err != nil {
		t.Fatalf("FingerprintReader failed: %v", err)
	}
	if r.closed {
		t.Fatalf("FingerprintReader must not close the reader")
	}
}

type chunkReader struct {
	data  []byte
	chunk int
	off   int
}

func (r *chunkReader) Read(p []byte) (int, error) {
	if r.off >= len(r.data) {
		return 0, io.EOF
	}
	n := r.chunk
	if n <= 0 {
		n = 1
	}
	if n > len(p) {
		n = len(p)
	}
	if r.off+n > len(r.data) {
		n = len(r.data) - r.off
	}
	copy(p, r.data[r.off:r.off+n])
	r.off += n
	return n, nil
}

type errorReader struct{ err error }

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

type trackingReadCloser struct {
	io.Reader
	closed bool
}

func (r *trackingReadCloser) Close() error {
	r.closed = true
	return nil
}
