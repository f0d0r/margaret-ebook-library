package compress

import (
	"bytes"
	stdzlib "compress/zlib"
	"errors"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
)

func zlibEncode(t *testing.T, s string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := stdzlib.NewWriter(&buf)
	if _, err := w.Write([]byte(s)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDecompressZlib(t *testing.T) {
	plain := string(bytes.Repeat([]byte("The quick brown fox jumps over the lazy dog. "), 8))
	enc := zlibEncode(t, plain)

	t.Run("round trip unlimited", func(t *testing.T) {
		got, err := DecompressZlib(enc, -1)
		if err != nil {
			t.Fatalf("DecompressZlib() error: %v", err)
		}
		if string(got) != plain {
			t.Errorf("DecompressZlib() = %q, want original", got)
		}
	})

	t.Run("exact budget passes", func(t *testing.T) {
		if _, err := DecompressZlib(enc, int64(len(plain))); err != nil {
			t.Errorf("DecompressZlib() error: %v, want nil", err)
		}
	})

	t.Run("over budget fails", func(t *testing.T) {
		if _, err := DecompressZlib(enc, int64(len(plain))-1); !errors.Is(err, book.ErrLimitExceeded) {
			t.Errorf("DecompressZlib() error = %v, want ErrLimitExceeded", err)
		}
		if _, err := DecompressZlib(enc, 0); !errors.Is(err, book.ErrLimitExceeded) {
			t.Errorf("DecompressZlib() error = %v, want ErrLimitExceeded", err)
		}
	})

	t.Run("empty payload with zero budget", func(t *testing.T) {
		got, err := DecompressZlib(zlibEncode(t, ""), 0)
		if err != nil {
			t.Fatalf("DecompressZlib() error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("DecompressZlib() = %q, want empty", got)
		}
	})

	t.Run("corrupt input", func(t *testing.T) {
		for name, data := range map[string][]byte{
			"empty":     nil,
			"not zlib":  []byte("not a zlib stream"),
			"truncated": enc[:len(enc)/2],
		} {
			if _, err := DecompressZlib(data, -1); !errors.Is(err, book.ErrCorrupt) {
				t.Errorf("%s: DecompressZlib() error = %v, want ErrCorrupt", name, err)
			}
		}
	})
}
