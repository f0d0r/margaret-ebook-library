package tools

import (
	"errors"
	"math/bits"
)

// SimHasher is a streaming SimHash calculator. It implements io.Writer.
// Write normalized text, then call Sum64() after EOF to obtain the 64-bit fingerprint.
type SimHasher struct {
	proc        *wordShingler
	vec         [64]int32
	count       int
	finalized   bool
	shingleSize int
}

// SimHashOption configures a SimHasher.
type SimHashOption func(*SimHasher)

func WithSimShingleSize(k int) SimHashOption {
	return func(s *SimHasher) {
		if k > 0 {
			s.shingleSize = k
		}
	}
}

// NewSimHasher creates a SimHasher with default shingle size 5.
func NewSimHasher(opts ...SimHashOption) *SimHasher {
	sh := &SimHasher{shingleSize: 5}
	for _, o := range opts {
		o(sh)
	}
	if sh.shingleSize <= 0 {
		sh.shingleSize = 5
	}
	sh.proc = newWordShingler(sh.shingleSize, sh.onShingle)
	return sh
}

func (s *SimHasher) onShingle(shingle string) {
	h := fnv64(shingle)
	for i := range 64 {
		if (h>>uint(i))&1 == 1 {
			s.vec[i]++
		} else {
			s.vec[i]--
		}
	}
	s.count++
}

// Write implements io.Writer.
func (s *SimHasher) Write(p []byte) (int, error) {
	if s.finalized {
		return 0, errors.New("simhasher already finalized: do not write after Sum64()")
	}
	return s.proc.Write(p)
}

// Sum64 returns the 64-bit SimHash fingerprint. It flushes pending state.
func (s *SimHasher) Sum64() uint64 {
	if !s.finalized {
		s.proc.Flush()
		s.finalized = true
	}
	var fp uint64
	for i := range 64 {
		if s.vec[i] > 0 {
			fp |= 1 << uint(i)
		}
	}
	return fp
}

// Hamming returns the Hamming distance between two 64-bit SimHashes.
func Hamming(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

// Hamming returns the Hamming distance between two finalized SimHashers.
func (s *SimHasher) Hamming(other *SimHasher) int {
	return Hamming(s.Sum64(), other.Sum64())
}

// Similarity returns similarity in [0,1] as (64 - Hamming)/64.
func Similarity(a, b uint64) float64 {
	return float64(64-Hamming(a, b)) / 64.0
}
