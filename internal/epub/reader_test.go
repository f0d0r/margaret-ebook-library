package epub

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestSupports(t *testing.T) {
	tmpDir := t.TempDir()

	validEPUBPath := filepath.Join(tmpDir, "valid.epub")
	createValidTestEPUB(t, validEPUBPath, "My Test Book")

	regularZIPPath := filepath.Join(tmpDir, "regular.zip")
	createTestZIP(t, regularZIPPath, []testFile{
		{name: "hello.txt", content: "hello world", method: zip.Deflate},
	})

	wrongOrderPath := filepath.Join(tmpDir, "wrong_order.epub")
	createTestZIP(t, wrongOrderPath, []testFile{
		{name: "OEBPS/content.opf", content: createOPF("Test"), method: zip.Deflate},
		{name: "mimetype", content: "application/epub+zip", method: zip.Store},
	})

	wrongContentPath := filepath.Join(tmpDir, "wrong_content.epub")
	createTestZIP(t, wrongContentPath, []testFile{
		{name: "mimetype", content: "text/plain", method: zip.Store},
	})

	plainTextPath := filepath.Join(tmpDir, "text.txt")
	err := os.WriteFile(plainTextPath, []byte("This is a plain text file, not a ZIP."), 0644)
	if err != nil {
		t.Fatalf("failed to create plain text file: %v", err)
	}

	shortFilePath := filepath.Join(tmpDir, "short.dat")
	err = os.WriteFile(shortFilePath, []byte("PK\x03\x04short"), 0644)
	if err != nil {
		t.Fatalf("failed to create short file: %v", err)
	}

	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{"Valid EPUB file", validEPUBPath, true},
		{"Regular ZIP without mimetype", regularZIPPath, false},
		{"Wrong order with mimetype second", wrongOrderPath, false},
		{"Wrong mimetype content", wrongContentPath, false},
		{"Plain text file", plainTextPath, false},
		{"Too short file", shortFilePath, false},
		{"Non-existent file", filepath.Join(tmpDir, "does_not_exist.epub"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, supportsTest(tt.filePath, tt.want))
	}
}

func supportsTest(path string, want bool) func(t *testing.T) {
	return func(t *testing.T) {
		reader := NewEpubReader()
		if got := reader.Supports(path); got != want {
			t.Errorf("EPUBReader.Supports() = %v, want %v (file: %s)", got, want, path)
		}
	}
}
