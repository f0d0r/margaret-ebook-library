package tools

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
)

func TestCalculateFileHashFromBlob(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "book.bin")
	content := []byte("hello world")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	want := fmt.Sprintf("%x", sha256.Sum256(content))
	got, err := CalculateFileHashFromBlob(book.NewPathBlob(path))
	if err != nil {
		t.Fatalf("CalculateFileHashFromBlob() error: %v", err)
	}
	if got != want {
		t.Errorf("hash = %q, want %q", got, want)
	}
}

func TestCalculateFileHashFromBlobPreservesPosition(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "book.bin")
	if err := os.WriteFile(path, []byte("hello world"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		t.Fatalf("failed to seek: %v", err)
	}

	b, err := book.NewFileBlob(f)
	if err != nil {
		t.Fatalf("NewFileBlob() error: %v", err)
	}
	if _, err := CalculateFileHashFromBlob(b); err != nil {
		t.Fatalf("CalculateFileHashFromBlob() error: %v", err)
	}

	posAfter, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		t.Fatalf("failed to get current position: %v", err)
	}
	if posAfter != int64(len("hello world")) {
		t.Errorf("file position changed: before=%d after=%d", len("hello world"), posAfter)
	}
}

func TestCalculateFileHash(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "book.bin")
	content := []byte("hello world")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	want := fmt.Sprintf("%x", sha256.Sum256(content))
	got, err := CalculateFileHash(path)
	if err != nil {
		t.Fatalf("CalculateFileHash() error: %v", err)
	}
	if got != want {
		t.Errorf("hash = %q, want %q", got, want)
	}
}
