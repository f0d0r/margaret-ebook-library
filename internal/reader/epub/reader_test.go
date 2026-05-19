package epub

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

type testFile struct {
	name    string
	content string
	method  uint16
}

func TestSupports(t *testing.T) {
	tmpDir := t.TempDir()

	validEPUBPath := filepath.Join(tmpDir, "valid.epub")
	createTestZIP(t, validEPUBPath, []testFile{
		{name: "mimetype", content: "application/epub+zip", method: zip.Store},
		{name: "OEBPS/content.opf", content: "<package></package>", method: zip.Deflate},
	})

	regularZIPPath := filepath.Join(tmpDir, "regular.zip")
	createTestZIP(t, regularZIPPath, []testFile{
		{name: "hello.txt", content: "hello world", method: zip.Deflate},
	})

	wrongOrderPath := filepath.Join(tmpDir, "wrong_order.epub")
	createTestZIP(t, wrongOrderPath, []testFile{
		{name: "OEBPS/content.opf", content: "<package></package>", method: zip.Deflate},
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
		t.Run(tt.name, rangeTest(tt.filePath, tt.want))
	}
}

func rangeTest(path string, want bool) func(t *testing.T) {
	return func(t *testing.T) {
		reader := &EpubReader{}
		if got := reader.Supports(path); got != want {
			t.Errorf("EPUBReader.Supports() = %v, want %v (file: %s)", got, want, path)
		}
	}
}

func createTestZIP(t *testing.T, path string, files []testFile) {
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create zip file: %v", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer func() {
		if err := w.Close(); err != nil {
			t.Fatalf("failed to close zip writer: %v", err)
		}
	}()

	for _, file := range files {
		header := &zip.FileHeader{
			Name:   file.name,
			Method: file.method,
		}
		fw, err := w.CreateHeader(header)
		if err != nil {
			t.Fatalf("failed to create zip entry header: %v", err)
		}
		_, err = fw.Write([]byte(file.content))
		if err != nil {
			t.Fatalf("failed to write zip entry content: %v", err)
		}
	}
}
