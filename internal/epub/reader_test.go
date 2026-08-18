package epub

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/pkg/model"
)

func TestGetCover(t *testing.T) {
	tmpDir := t.TempDir()
	reader := NewEpubReader(model.DefaultConfig())

	tests := []struct {
		name              string
		epubContent       []testFile
		shouldFindCover   bool
		expectedID        string
		expectedName      string
		expectedMediaType string
		expectedSize      int64
	}{
		{
			name: "Cover found by cover-image property",
			epubContent: []testFile{
				{name: "mimetype", content: "application/epub+zip", method: zip.Store},
				{name: "META-INF/container.xml", content: createContainerXML("OEBPS/content.opf"), method: zip.Deflate},
				{name: "OEBPS/content.opf", content: createOPFWithCoverImage("cover_id"), method: zip.Deflate},
				{name: "OEBPS/cover.jpg", content: string(createTestImageData()), method: zip.Deflate},
			},
			shouldFindCover:   true,
			expectedID:        "cover_id",
			expectedName:      "cover.jpg",
			expectedMediaType: "image/jpeg",
			expectedSize:      int64(len(createTestImageData())),
		},
		{
			name: "Cover found by cover meta element",
			epubContent: []testFile{
				{name: "mimetype", content: "application/epub+zip", method: zip.Store},
				{name: "META-INF/container.xml", content: createContainerXML("OEBPS/content.opf"), method: zip.Deflate},
				{name: "OEBPS/content.opf", content: createOPFWithCoverMeta("cover_id"), method: zip.Deflate},
				{name: "OEBPS/cover.png", content: string(createTestImageData()), method: zip.Deflate},
			},
			shouldFindCover:   true,
			expectedID:        "cover_id",
			expectedName:      "cover.png",
			expectedMediaType: "image/png",
			expectedSize:      int64(len(createTestImageData())),
		},
		{
			name: "No cover found - no property or meta",
			epubContent: []testFile{
				{name: "mimetype", content: "application/epub+zip", method: zip.Store},
				{name: "META-INF/container.xml", content: createContainerXML("OEBPS/content.opf"), method: zip.Deflate},
				{name: "OEBPS/content.opf", content: createOPF("Test Book"), method: zip.Deflate},
			},
			shouldFindCover: false,
		},
		{
			name: "Cover item referenced but file missing from ZIP",
			epubContent: []testFile{
				{name: "mimetype", content: "application/epub+zip", method: zip.Store},
				{name: "META-INF/container.xml", content: createContainerXML("OEBPS/content.opf"), method: zip.Deflate},
				{name: "OEBPS/content.opf", content: createOPFWithCoverImage("cover_id"), method: zip.Deflate},
			},
			shouldFindCover: false,
		},
		{
			name: "Cover meta references non-existent item",
			epubContent: []testFile{
				{name: "mimetype", content: "application/epub+zip", method: zip.Store},
				{name: "META-INF/container.xml", content: createContainerXML("OEBPS/content.opf"), method: zip.Deflate},
				{name: "OEBPS/content.opf", content: createOPFWithCoverMeta("nonexistent_id"), method: zip.Deflate},
			},
			shouldFindCover: false,
		},
		{
			name: "Prefer cover-image property over cover meta",
			epubContent: []testFile{
				{name: "mimetype", content: "application/epub+zip", method: zip.Store},
				{name: "META-INF/container.xml", content: createContainerXML("OEBPS/content.opf"), method: zip.Deflate},
				{name: "OEBPS/content.opf", content: createOPFWithBothCovers("cover_image_id", "cover_meta_id"), method: zip.Deflate},
				{name: "OEBPS/cover_image.jpg", content: string(createTestImageData()), method: zip.Deflate},
				{name: "OEBPS/cover_meta.png", content: string(createTestImageData()), method: zip.Deflate},
			},
			shouldFindCover:   true,
			expectedID:        "cover_image_id",
			expectedName:      "cover_image.jpg",
			expectedMediaType: "image/jpeg",
			expectedSize:      int64(len(createTestImageData())),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			epubPath := filepath.Join(tmpDir, tt.name+".epub")
			createTestZIP(t, epubPath, tt.epubContent)

			// Parse the EPUB to get the OPF Package
			b := model.NewPathBlob(epubPath)
			size, err := b.Size()
			if err != nil {
				t.Fatalf("failed to stat epub: %v", err)
			}
			zr, err := zip.NewReader(b, size)
			if err != nil {
				t.Fatalf("failed to open epub: %v", err)
			}

			container, err := reader.ocfReader.Read(zr)
			if err != nil {
				t.Fatalf("failed to read container: %v", err)
			}

			pkg, err := reader.opfReader.Read(zr, container)
			if err != nil {
				t.Fatalf("failed to read opf: %v", err)
			}

			// Get the cover
			cover := reader.cover(zr, pkg)

			if tt.shouldFindCover {
				if cover == nil {
					t.Errorf("expected to find cover, but got nil")
					return
				}
				if cover.Id != tt.expectedID {
					t.Errorf("got cover ID %q, want %q", cover.Id, tt.expectedID)
				}
				if cover.Name != tt.expectedName {
					t.Errorf("got cover name %q, want %q", cover.Name, tt.expectedName)
				}
				if cover.MediaType != tt.expectedMediaType {
					t.Errorf("got cover media type %q, want %q", cover.MediaType, tt.expectedMediaType)
				}
				if cover.Size != tt.expectedSize {
					t.Errorf("got cover size %d, want %d", cover.Size, tt.expectedSize)
				}
				if cover.Open == nil {
					t.Errorf("expected Open function to be set")
				}
			} else {
				if cover != nil {
					t.Errorf("expected nil cover, but got: %+v", cover)
				}
			}
		})
	}
}

// Helper functions for GetCover tests

func createTestImageData() []byte {
	// Create minimal valid JPEG data (just enough to be recognizable)
	return []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00" +
		"\xff\xdb\x00C\x00\x08\x06\x06\x07\x06\x05\x08\x07\x07\x07\t\t\x08\n\x0c\x14\r" +
		"\x0c\x0b\x0b\x0c\x19\x12\x13\x0f\x14\x1d\x1a\x1f\x1e\x1d\x1a\x1c\x1c $.' \",#\x1c" +
		"\x1c(7),01444\x1f'9=82<.342\xff\xc0\x00\x0b\x08\x00\x01\x00\x01\x01\x11\x00\xff\xc4" +
		"\x00\x1f\x00\x00\x01\x05\x01\x01\x01\x01\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x01" +
		"\x02\x03\x04\x05\x06\x07\x08\t\n\x0b\xff\xda\x00\x08\x01\x01\x00\x00?\x00\xff\xd9")
}

func createOPFWithCoverImage(coverID string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>
  </metadata>
  <manifest>
    <item id="` + coverID + `" media-type="image/jpeg" href="cover.jpg" properties="cover-image" />
  </manifest>
</package>`
}

func createOPFWithCoverMeta(coverID string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>
    <meta name="cover" content="` + coverID + `" />
  </metadata>
  <manifest>
    <item id="` + coverID + `" media-type="image/png" href="cover.png" />
  </manifest>
</package>`
}

func createOPFWithBothCovers(coverImageID, coverMetaID string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>
    <meta name="cover" content="` + coverMetaID + `" />
  </metadata>
  <manifest>
    <item id="` + coverImageID + `" media-type="image/jpeg" href="cover_image.jpg" properties="cover-image" />
    <item id="` + coverMetaID + `" media-type="image/png" href="cover_meta.png" />
  </manifest>
</package>`
}

func TestEpubReaderSupportsPreservesPosition(t *testing.T) {
	tmpDir := t.TempDir()
	reader := NewEpubReader(model.DefaultConfig())

	validPath := filepath.Join(tmpDir, "valid.epub")
	createValidTestEPUB(t, validPath, "My Test Book")

	f, err := os.Open(validPath)
	if err != nil {
		t.Fatalf("failed to open epub: %v", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Seek(10, io.SeekCurrent); err != nil {
		t.Fatalf("failed to seek: %v", err)
	}

	b, err := model.NewFileBlob(f)
	if err != nil {
		t.Fatalf("NewFileBlob() error: %v", err)
	}
	if !reader.Supports(b) {
		t.Error("Supports() = false at non-zero position, want true")
	}

	posAfter, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		t.Fatalf("failed to get current position: %v", err)
	}
	if posAfter != 10 {
		t.Errorf("file position changed: before=10 after=%d", posAfter)
	}
}

func TestEpubReaderSupports(t *testing.T) {
	tmpDir := t.TempDir()
	reader := NewEpubReader(model.DefaultConfig())

	validPath := filepath.Join(tmpDir, "valid.epub")
	createValidTestEPUB(t, validPath, "My Test Book")

	if !reader.Supports(model.NewPathBlob(validPath)) {
		t.Error("Supports() = false for valid EPUB path blob, want true")
	}
}

func TestReadZipFileRejectsOversizedEntry(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	fw, err := w.CreateHeader(&zip.FileHeader{Name: "cover.jpg", Method: zip.Deflate})
	if err != nil {
		t.Fatalf("CreateHeader() error: %v", err)
	}
	chunk := make([]byte, 1024*1024)
	if _, err := fw.Write(bytes.Repeat(chunk, 51)); err != nil { // 51 MB of zeros
		t.Fatalf("Write() error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("zip.NewReader() error: %v", err)
	}
	if len(zr.File) != 1 {
		t.Fatalf("got %d files, want 1", len(zr.File))
	}

	if rc, err := NewEpubReader(model.DefaultConfig()).openZipFile(zr.File[0]); err == nil {
		t.Fatalf("openZipFile() expected error for oversized entry, got nil")
	} else if rc != nil {
		_ = rc.Close()
	}
}

func TestReadZipFile_ConfigOverrideRejectsEntry(t *testing.T) {
	imgData := createTestImageData()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	fw, err := w.CreateHeader(&zip.FileHeader{Name: "cover.jpg", Method: zip.Store})
	if err != nil {
		t.Fatalf("CreateHeader() error: %v", err)
	}
	if _, err := fw.Write(imgData); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("zip.NewReader() error: %v", err)
	}

	cfg := model.DefaultConfig()
	cfg.MaxCoverSize = 1
	reader := NewEpubReader(cfg)

	if rc, err := reader.openZipFile(zr.File[0]); err == nil {
		t.Fatalf("openZipFile() expected error with reduced MaxCoverSize, got nil")
	} else if rc != nil {
		_ = rc.Close()
	}
}

func TestReadZipFileReadsNormalEntry(t *testing.T) {
	imgData := createTestImageData()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	fw, err := w.CreateHeader(&zip.FileHeader{Name: "cover.jpg", Method: zip.Store})
	if err != nil {
		t.Fatalf("CreateHeader() error: %v", err)
	}
	if _, err := fw.Write(imgData); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("zip.NewReader() error: %v", err)
	}

	rc, err := NewEpubReader(model.DefaultConfig()).openZipFile(zr.File[0])
	if err != nil {
		t.Fatalf("openZipFile() unexpected error: %v", err)
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll() unexpected error: %v", err)
	}
	if string(data) != string(imgData) {
		t.Errorf("openZipFile() returned %d bytes, want %d", len(data), len(imgData))
	}
}
