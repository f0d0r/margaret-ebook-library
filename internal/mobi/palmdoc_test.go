package mobi

import (
	"testing"

	compressutil "github.com/f0d0r/margaret-ebook-library/internal/compress"
)

// palmDocEncode compresses input with a minimal PalmDOC LZ77 encoder. It is
// only used to build round-trip test vectors for the decompressor.
func palmDocEncode(input []byte) []byte {
	var out []byte
	i := 0
	n := len(input)
	for i < n {
		bestLen, bestDist := 0, 0
		maxLen := min(n-i, 10)
		maxDist := min(i, 2047)
		for d := 1; d <= maxDist; d++ {
			l := 0
			for l < maxLen && input[i+l] == input[i-d+l] {
				l++
			}
			if l >= 3 && l > bestLen {
				bestLen, bestDist = l, d
			}
		}
		if bestLen >= 3 {
			m := bestDist
			ln := bestLen
			out = append(out, byte(0x80|(m>>5)), byte(((m&0x1f)<<3)|(ln-3)))
			i += bestLen
		} else {
			b := input[i]
			if b < 0x09 || b >= 0x80 {
				// Bytes outside the literal range (and the escape
				// byte itself) travel as a 0x01 escape + raw byte.
				out = append(out, 0x01, b)
			} else {
				out = append(out, b)
			}
			i++
		}
	}
	return out
}

func TestDecompressPalmDocRoundTrip(t *testing.T) {
	cases := []string{
		"",
		"a",
		"Hello, world! This is a test of the PalmDOC compressor.",
		"aaaaaaaaaaaaaaaaaaaaaaaaaa",
		"abcabcabcabcabcabcabcabcabc",
		"The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog.",
		"Spaces and\nnewlines\tand more punctuation !?%$#@*()",
	}
	for _, tc := range cases {
		enc := palmDocEncode([]byte(tc))
		dec, err := compressutil.DecompressPalmDoc(enc, -1)
		if err != nil {
			t.Fatalf("DecompressPalmDoc() error: %v", err)
		}
		if string(dec) != tc {
			t.Errorf("round-trip %q: got %q", tc, string(dec))
		}
	}
}
