package mobi

import (
	"testing"
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
			out = append(out, input[i])
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
		dec, err := DecompressPalmDoc(enc, -1)
		if err != nil {
			t.Fatalf("DecompressPalmDoc() error: %v", err)
		}
		if string(dec) != tc {
			t.Errorf("round-trip %q: got %q", tc, string(dec))
		}
	}
}

func TestDecompressPalmDocLiteralRun(t *testing.T) {
	in := []byte{3, 'a', 'b', 'c', 2, 'x', 'y'}
	got, err := DecompressPalmDoc(in, -1)
	if err != nil {
		t.Fatalf("DecompressPalmDoc() error: %v", err)
	}
	if string(got) != "abcxy" {
		t.Errorf("DecompressPalmDoc() = %q, want %q", got, "abcxy")
	}
}

func TestDecompressPalmDocSpaceVariant(t *testing.T) {
	in := []byte{0xC1, 0xC2, 0xC3}
	got, err := DecompressPalmDoc(in, -1)
	if err != nil {
		t.Fatalf("DecompressPalmDoc() error: %v", err)
	}
	if string(got) != " A B C" {
		t.Errorf("DecompressPalmDoc() = %q, want %q", got, " A B C")
	}
}

func TestDecompressPalmDocOverlap(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{"progressive pattern", []byte{'a', 'b', 0x80, 0x12}, "abababa"},
		{"distance one run", []byte{'a', 0x80, 0x0F}, "aaaaaaaaaaa"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecompressPalmDoc(tt.in, -1)
			if err != nil {
				t.Fatalf("DecompressPalmDoc() error: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("DecompressPalmDoc() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDecompressPalmDocLenient(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{"truncated literal run", []byte{5, 'a'}, "a"},
		{"missing back-reference byte", []byte{0x80}, ""},
		{"distance exceeds output", []byte{0x80, 0x08}, ""},
		{"zero distance emits zero bytes", []byte{0x80, 0x00}, "\x00\x00\x00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecompressPalmDoc(tt.in, -1)
			if err != nil {
				t.Fatalf("DecompressPalmDoc() error: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("DecompressPalmDoc() = %q, want %q", got, tt.want)
			}
		})
	}
}
