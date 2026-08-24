package tools

import (
	"errors"
	"hash/fnv"
)

// MinHashConfig configures a MinHasher.
type MinHashConfig struct {
	NumHashes   int
	ShingleSize int
	Seed        uint64
}

// MinHashOption configures a MinHasher created via NewMinHasher.
type MinHashOption func(*MinHashConfig)

func WithNumHashes(n int) MinHashOption { return func(c *MinHashConfig) { c.NumHashes = n } }
func WithShingleSize(k int) MinHashOption {
	return func(c *MinHashConfig) { c.ShingleSize = k }
}
func WithSeed(s uint64) MinHashOption { return func(c *MinHashConfig) { c.Seed = s } }

// MinHasher is a streaming MinHash calculator. It implements io.Writer.
// Write normalized text, then call Signature() after EOF to obtain the signature.
// It is not safe for concurrent use and should not be written to after Signature().
type MinHasher struct {
	cfg       MinHashConfig
	seeds     []uint64
	sig       []uint64
	proc      *wordShingler
	finalized bool
}

// NewMinHasher creates a MinHasher with defaults (NumHashes=128, ShingleSize=5).
func NewMinHasher(opts ...MinHashOption) *MinHasher {
	cfg := MinHashConfig{NumHashes: 128, ShingleSize: 5}
	for _, o := range opts {
		o(&cfg)
	}
	return newMinHasherWithConfig(cfg)
}

func newMinHasherWithConfig(cfg MinHashConfig) *MinHasher {
	if cfg.NumHashes <= 0 {
		cfg.NumHashes = 128
	}
	if cfg.ShingleSize <= 0 {
		cfg.ShingleSize = 5
	}
	seeds := generateSeeds(cfg.NumHashes, cfg.Seed)
	sig := make([]uint64, cfg.NumHashes)
	for i := range sig {
		sig[i] = ^uint64(0)
	}
	mh := &MinHasher{cfg: cfg, seeds: seeds, sig: sig}
	mh.proc = newWordShingler(cfg.ShingleSize, mh.onShingle)
	return mh
}

func (m *MinHasher) onShingle(s string) {
	h := fnv64(s)
	for i, seed := range m.seeds {
		mixed := h ^ seed
		// extra mixing to avalanche
		mixed ^= mixed >> 33
		mixed *= 0xff51afd7ed558ccd
		mixed ^= mixed >> 33
		mixed *= 0xc4ceb9fe1a85ec53
		mixed ^= mixed >> 33
		if mixed < m.sig[i] {
			m.sig[i] = mixed
		}
	}
}

// Write implements io.Writer. It processes text in a streaming fashion,
// normalizing (NFKC, lower, whitespace collapse) and shingling with k words.
func (m *MinHasher) Write(p []byte) (int, error) {
	if m.finalized {
		return 0, errors.New("minhasher already finalized: do not write after Signature()")
	}
	return m.proc.Write(p)
}

// Signature returns the MinHash signature. It flushes any pending word/shingle.
// The returned slice is a copy. Do not write after calling Signature.
func (m *MinHasher) Signature() []uint64 {
	if !m.finalized {
		m.proc.Flush()
		m.finalized = true
	}
	out := make([]uint64, len(m.sig))
	copy(out, m.sig)
	return out
}

// Jaccard estimates Jaccard similarity between two signatures.
// Signatures must have the same length. Empty signatures (len 0) return 1 if both empty.
func Jaccard(a, b []uint64) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	if len(a) != len(b) || len(a) == 0 {
		return 0.0
	}
	matches := 0
	for i := range a {
		if a[i] == b[i] {
			matches++
		}
	}
	return float64(matches) / float64(len(a))
}

// JaccardOf is a helper that computes Jaccard for two MinHashers after finalization.
func (m *MinHasher) Jaccard(other *MinHasher) float64 {
	return Jaccard(m.Signature(), other.Signature())
}

func fnv64(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

func generateSeeds(n int, seed uint64) []uint64 {
	if seed == 0 {
		seed = 0x517cc1b727220a95
	}
	seeds := make([]uint64, n)
	x := seed
	for i := range n {
		x += 0x9e3779b97f4a7c15
		z := x
		z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
		z = (z ^ (z >> 27)) * 0x94d049bb133111eb
		z ^= z >> 31
		if z == 0 {
			z = 0x9e3779b97f4a7c15 + uint64(i)*0x85ebca6b
		}
		seeds[i] = z
	}
	return seeds
}
