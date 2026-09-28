package zip

import (
	stdzip "archive/zip"
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
)

func TestIsZip(t *testing.T) {
	tests := []struct {
		name string
		head []byte
		want bool
	}{
		{"local file header", []byte{0x50, 0x4B, 0x03, 0x04}, true},
		{"end of central directory", []byte{0x50, 0x4B, 0x05, 0x06}, true},
		{"data descriptor", []byte{0x50, 0x4B, 0x07, 0x08}, true},
		{"central directory header is not an archive start", []byte{0x50, 0x4B, 0x01, 0x02}, false},
		{"too short", []byte{0x50, 0x4B, 0x03}, false},
		{"empty", nil, false},
		{"plain text", []byte("hello"), false},
		{"xml", []byte("<?xml"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsZip(tt.head); got != tt.want {
				t.Errorf("IsZip() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpen(t *testing.T) {
	t.Run("valid archive", func(t *testing.T) {
		b := book.NewBytesBlob(buildZip(t, []stdzipEntry{{"a.txt", "hello"}}))
		zr, err := Open(b)
		if err != nil {
			t.Fatalf("Open() error: %v", err)
		}
		if len(zr.File) != 1 {
			t.Errorf("len(Files) = %d, want 1", len(zr.File))
		}
	})

	t.Run("empty blob", func(t *testing.T) {
		if _, err := Open(book.NewBytesBlob(nil)); err == nil {
			t.Error("Open() expected error for empty blob, got nil")
		}
	})

	t.Run("truncated header", func(t *testing.T) {
		if _, err := Open(book.NewBytesBlob([]byte{0x50, 0x4B, 0x03, 0x04})); err == nil {
			t.Error("Open() expected error for truncated archive, got nil")
		}
	})

	t.Run("non-zip", func(t *testing.T) {
		if _, err := Open(book.NewBytesBlob([]byte("not a zip file"))); err == nil {
			t.Error("Open() expected error for non-zip blob, got nil")
		}
	})
}

func TestFind(t *testing.T) {
	b := book.NewBytesBlob(buildZip(t, []stdzipEntry{
		{"dir/a.txt", "a"},
		{"b.txt", "b"},
	}))
	zr, err := Open(b)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}

	if f := Find(zr, "b.txt"); f == nil {
		t.Error("Find() = nil for present entry, want file")
	}
	if f := Find(zr, "missing.txt"); f != nil {
		t.Errorf("Find() = %v for missing entry, want nil", f.Name)
	}
	if f := Find(zr, "B.TXT"); f != nil {
		t.Error("Find() matched different case, want exact match")
	}
}

func TestReadHead(t *testing.T) {
	content := "0123456789abcdef"
	b := book.NewBytesBlob(buildZip(t, []stdzipEntry{{"a.txt", content}}))
	zr, err := Open(b)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}

	head, err := ReadHead(zr.File[0], 10)
	if err != nil {
		t.Fatalf("ReadHead() error: %v", err)
	}
	if string(head) != content[:10] {
		t.Errorf("ReadHead() = %q, want truncated %q", string(head), content[:10])
	}

	full, err := ReadHead(zr.File[0], 1024)
	if err != nil {
		t.Fatalf("ReadHead() error: %v", err)
	}
	if string(full) != content {
		t.Errorf("ReadHead() = %q, want full %q", string(full), content)
	}
}

func TestOpenLimited(t *testing.T) {
	content := "hello limited world"
	b := book.NewBytesBlob(buildZip(t, []stdzipEntry{{"cover.bin", content}}))
	zr, err := Open(b)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}

	t.Run("reads entry within limit", func(t *testing.T) {
		rc, err := OpenLimited(zr.File[0], 1024)
		if err != nil {
			t.Fatalf("OpenLimited() error: %v", err)
		}
		defer func() { _ = rc.Close() }()
		data, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("ReadAll() error: %v", err)
		}
		if string(data) != content {
			t.Errorf("ReadAll() = %q, want %q", string(data), content)
		}
	})

	t.Run("rejects entry declaring more than limit", func(t *testing.T) {
		rc, err := OpenLimited(zr.File[0], 5)
		if err == nil {
			_ = rc.Close()
			t.Fatal("OpenLimited() expected error for oversized entry, got nil")
		}
		if !errors.Is(err, book.ErrLimitExceeded) {
			t.Errorf("OpenLimited() error = %v, want ErrLimitExceeded", err)
		}
	})
}

type stdzipEntry struct {
	name    string
	content string
}

// buildZip packs entries into an in-memory zip archive.
func buildZip(t *testing.T, entries []stdzipEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := stdzip.NewWriter(&buf)
	for _, e := range entries {
		fw, err := w.Create(e.name)
		if err != nil {
			t.Fatalf("Create(%q) error: %v", e.name, err)
		}
		if _, err := fw.Write([]byte(e.content)); err != nil {
			t.Fatalf("Write(%q) error: %v", e.name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
	return buf.Bytes()
}
