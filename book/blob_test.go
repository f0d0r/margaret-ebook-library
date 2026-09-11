package book

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBytesBlob(t *testing.T) {
	data := []byte("hello blob world")
	b := NewBytesBlob(data)

	size, err := b.Size()
	if err != nil {
		t.Fatalf("Size() error = %v", err)
	}
	if size != int64(len(data)) {
		t.Errorf("Size() = %d, want %d", size, len(data))
	}

	buf := make([]byte, 5)
	n, err := b.ReadAt(buf, 0)
	if err != nil {
		t.Fatalf("ReadAt(0) error = %v", err)
	}
	if n != 5 || string(buf) != "hello" {
		t.Errorf("ReadAt(0) = %q, want %q", string(buf[:n]), "hello")
	}

	n, err = b.ReadAt(buf, 6)
	if err != nil {
		t.Fatalf("ReadAt(6) error = %v", err)
	}
	if n != 5 || string(buf) != "blob " {
		t.Errorf("ReadAt(6) = %q, want %q", string(buf[:n]), "blob ")
	}

	// Empty bytes blob
	empty := NewBytesBlob(nil)
	sz, err := empty.Size()
	if err != nil || sz != 0 {
		t.Errorf("empty Size() = %d, err = %v, want 0, nil", sz, err)
	}
}

func TestReaderAtBlob(t *testing.T) {
	content := []byte("reader at content")
	ra := bytes.NewReader(content)
	b := NewReaderAtBlob(ra, int64(len(content)))

	size, err := b.Size()
	if err != nil {
		t.Fatalf("Size() error = %v", err)
	}
	if size != int64(len(content)) {
		t.Errorf("Size() = %d, want %d", size, len(content))
	}

	buf := make([]byte, 6)
	n, err := b.ReadAt(buf, 0)
	if err != nil {
		t.Fatalf("ReadAt(0) error = %v", err)
	}
	if n != 6 || string(buf) != "reader" {
		t.Errorf("ReadAt(0) = %q, want %q", string(buf[:n]), "reader")
	}

	n, err = b.ReadAt(buf, 10)
	if err != nil {
		t.Fatalf("ReadAt(10) error = %v", err)
	}
	if n != 6 || string(buf) != "conten" {
		t.Errorf("ReadAt(10) = %q, want %q", string(buf[:n]), "conten")
	}
}

func TestPathBlob(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")
	content := []byte("path blob content for testing")
	if err := os.WriteFile(filePath, content, 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	b := NewPathBlob(filePath)

	size, err := b.Size()
	if err != nil {
		t.Fatalf("Size() error = %v", err)
	}
	if size != int64(len(content)) {
		t.Errorf("Size() = %d, want %d", size, len(content))
	}

	buf := make([]byte, 4)
	n, err := b.ReadAt(buf, 0)
	if err != nil {
		t.Fatalf("ReadAt(0) error = %v", err)
	}
	if n != 4 || string(buf) != "path" {
		t.Errorf("ReadAt(0) = %q, want %q", string(buf[:n]), "path")
	}

	// Non-existent path
	nonExistent := NewPathBlob(filepath.Join(tmpDir, "does-not-exist.bin"))
	_, err = nonExistent.Size()
	if err == nil {
		t.Errorf("Size() on non-existent file should fail, got nil")
	}
	_, err = nonExistent.ReadAt(buf, 0)
	if err == nil {
		t.Errorf("ReadAt() on non-existent file should fail, got nil")
	}
}

func TestFileBlob(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "fileblob.bin")
	content := []byte("file blob data stream")
	if err := os.WriteFile(filePath, content, 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = f.Close() }()

	b, err := NewFileBlob(f)
	if err != nil {
		t.Fatalf("NewFileBlob error = %v", err)
	}

	size, err := b.Size()
	if err != nil {
		t.Fatalf("Size() error = %v", err)
	}
	if size != int64(len(content)) {
		t.Errorf("Size() = %d, want %d", size, len(content))
	}

	buf := make([]byte, 4)
	n, err := b.ReadAt(buf, 0)
	if err != nil {
		t.Fatalf("ReadAt(0) error = %v", err)
	}
	if n != 4 || string(buf) != "file" {
		t.Errorf("ReadAt(0) = %q, want %q", string(buf[:n]), "file")
	}

	// Closed file should fail Stat on NewFileBlob
	fClosed, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	_ = fClosed.Close()

	_, err = NewFileBlob(fClosed)
	if err == nil {
		t.Errorf("NewFileBlob on closed file should return error, got nil")
	}
}

func TestResource_Data(t *testing.T) {
	// nil resource
	var nilRes *Resource
	_, err := nilRes.Data()
	if err == nil || !strings.Contains(err.Error(), "not openable") {
		t.Errorf("Data() on nil resource want 'not openable' error, got %v", err)
	}

	// resource with nil Open func
	emptyRes := &Resource{}
	_, err = emptyRes.Data()
	if err == nil || !strings.Contains(err.Error(), "not openable") {
		t.Errorf("Data() with nil Open want 'not openable' error, got %v", err)
	}

	// Open returns error
	errOpenRes := &Resource{
		Open: func() (io.ReadCloser, error) {
			return nil, errors.New("open failure")
		},
	}
	_, err = errOpenRes.Data()
	if err == nil || !strings.Contains(err.Error(), "open failure") {
		t.Errorf("Data() want open failure error, got %v", err)
	}

	// Open succeeds
	content := "resource content bytes"
	okRes := &Resource{
		Open: func() (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(content)), nil
		},
	}
	data, err := okRes.Data()
	if err != nil {
		t.Fatalf("Data() unexpected error: %v", err)
	}
	if string(data) != content {
		t.Errorf("Data() = %q, want %q", string(data), content)
	}
}

func TestResourceSet_Len(t *testing.T) {
	var nilRS *ResourceSet
	if l := nilRS.Len(); l != 0 {
		t.Errorf("nil ResourceSet.Len() = %d, want 0", l)
	}

	emptyRS := NewResourceSet(nil, nil, nil)
	if l := emptyRS.Len(); l != 0 {
		t.Errorf("empty ResourceSet.Len() = %d, want 0", l)
	}

	populatedRS := NewResourceSet([]*Resource{
		{Id: "1", ResolvedHref: "1.html"},
		{Id: "2", ResolvedHref: "2.html"},
	}, nil, nil)
	if l := populatedRS.Len(); l != 2 {
		t.Errorf("populated ResourceSet.Len() = %d, want 2", l)
	}
}
