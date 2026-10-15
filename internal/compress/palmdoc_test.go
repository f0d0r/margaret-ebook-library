package compress

import (
	"errors"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
)

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

func TestDecompressPalmDocLimit(t *testing.T) {
	in := []byte{'a', 'b', 'c'}
	if _, err := DecompressPalmDoc(in, 2); !errors.Is(err, book.ErrLimitExceeded) {
		t.Errorf("DecompressPalmDoc() error = %v, want ErrLimitExceeded", err)
	}
	got, err := DecompressPalmDoc(in, 3)
	if err != nil {
		t.Fatalf("DecompressPalmDoc() error: %v", err)
	}
	if string(got) != "abc" {
		t.Errorf("DecompressPalmDoc() = %q, want %q", got, "abc")
	}
}
