package sections

import (
	"errors"
	"io"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
)

func TestOpen(t *testing.T) {
	data := []byte("0123456789abcdef")
	b := book.NewBytesBlob(data)

	t.Run("full read", func(t *testing.T) {
		rc, err := Open(b, 4, 6, 100, 100, "seg")
		if err != nil {
			t.Fatalf("Open() error: %v", err)
		}
		defer func() { _ = rc.Close() }()
		got, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("ReadAll() error: %v", err)
		}
		if string(got) != "456789" {
			t.Errorf("ReadAll() = %q, want %q", got, "456789")
		}
	})

	t.Run("resource limit exceeded", func(t *testing.T) {
		if _, err := Open(b, 0, 16, 8, 100, "seg"); !errors.Is(err, book.ErrLimitExceeded) {
			t.Errorf("Open() error = %v, want ErrLimitExceeded", err)
		}
	})

	t.Run("record limit exceeded", func(t *testing.T) {
		if _, err := Open(b, 0, 16, 100, 8, "seg"); !errors.Is(err, book.ErrLimitExceeded) {
			t.Errorf("Open() error = %v, want ErrLimitExceeded", err)
		}
	})

	t.Run("zero limits disable enforcement", func(t *testing.T) {
		rc, err := Open(b, 0, 16, 0, 0, "seg")
		if err != nil {
			t.Fatalf("Open() error: %v", err)
		}
		defer func() { _ = rc.Close() }()
		got, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("ReadAll() error: %v", err)
		}
		if string(got) != string(data) {
			t.Errorf("ReadAll() = %q, want full blob", got)
		}
	})
}
