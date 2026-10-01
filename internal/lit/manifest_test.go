package lit

import (
	"errors"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
)

// TestSizedString covers the length-prefixed string reader shared by every
// /manifest field, including the corrupt shapes it must reject rather than
// silently truncate.
func TestSizedString(t *testing.T) {
	for _, tt := range []struct {
		name    string
		in      []byte
		zpad    bool
		want    string
		rest    []byte
		wantErr bool
	}{
		{name: "empty body", in: []byte{0x02, 'a', 'b'}, want: "ab"},
		{name: "zero length", in: []byte{0x00, 0x07}, want: "", rest: []byte{0x07}},
		{name: "multibyte body", in: []byte{0x02, 0xC3, 0xB1, 'x'}, want: "ñx"},
		{name: "trailing bytes kept", in: []byte{0x01, 'a', 'z'}, want: "a", rest: []byte{'z'}},
		{
			name:    "truncated body",
			in:      []byte{0x04, 'a', 'b'},
			wantErr: true,
		},
		{
			name:    "truncated multibyte body",
			in:      []byte{0x01, 0xC3},
			wantErr: true,
		},
		{
			name:    "no length byte",
			in:      nil,
			wantErr: true,
		},
		{
			name:    "invalid length rune",
			in:      []byte{0xFF, 'a'},
			wantErr: true,
		},
		{
			name:    "invalid body rune",
			in:      []byte{0x01, 0xFF},
			wantErr: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, rest, err := sizedString(tt.in, tt.zpad)
			if tt.wantErr {
				if !errors.Is(err, book.ErrCorrupt) {
					t.Fatalf("err = %v, want ErrCorrupt", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
			if len(rest) != len(tt.rest) {
				t.Fatalf("rest = %q, want %q", rest, tt.rest)
			}
			for i := range rest {
				if rest[i] != tt.rest[i] {
					t.Fatalf("rest = %q, want %q", rest, tt.rest)
				}
			}
		})
	}
}

// TestSizedStringZPad checks the trailing-NUL consumption that the MIME
// fields of a /manifest entry carry.
func TestSizedStringZPad(t *testing.T) {
	got, rest, err := sizedString([]byte{0x02, 'a', 'b', 0x00, 'x'}, true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "ab" {
		t.Errorf("got %q, want ab", got)
	}
	if string(rest) != "x" {
		t.Errorf("rest = %q, want x", rest)
	}
	// Without zpad the NUL stays in the remainder.
	_, rest, err = sizedString([]byte{0x02, 'a', 'b', 0x00, 'x'}, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(rest) != "\x00x" {
		t.Errorf("rest = %q, want NUL+x", rest)
	}
}
