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

func TestGetCover(t *testing.T) {
	tmpDir := t.TempDir()
	reader := NewEpubReader()

	tests := []struct {
		name           string
		epubContent    []testFile
		shouldFindCover bool
		expectedID     string
		expectedName   string
		expectedMediaType string
		expectedSize   int
	}{
		{
			name: "Cover found by cover-image property",
			epubContent: []testFile{
				{name: "mimetype", content: "application/epub+zip", method: zip.Store},
				{name: "META-INF/container.xml", content: createContainerXML("OEBPS/content.opf"), method: zip.Deflate},
				{name: "OEBPS/content.opf", content: createOPFWithCoverImage("cover_id"), method: zip.Deflate},
				{name: "OEBPS/cover.jpg", content: string(createTestImageData()), method: zip.Deflate},
			},
			shouldFindCover: true,
			expectedID:      "cover_id",
			expectedName:    "cover.jpg",
			expectedMediaType: "image/jpeg",
			expectedSize:    len(createTestImageData()),
		},
		{
			name: "Cover found by cover meta element",
			epubContent: []testFile{
				{name: "mimetype", content: "application/epub+zip", method: zip.Store},
				{name: "META-INF/container.xml", content: createContainerXML("OEBPS/content.opf"), method: zip.Deflate},
				{name: "OEBPS/content.opf", content: createOPFWithCoverMeta("cover_id"), method: zip.Deflate},
				{name: "OEBPS/cover.png", content: string(createTestImageData()), method: zip.Deflate},
			},
			shouldFindCover: true,
			expectedID:      "cover_id",
			expectedName:    "cover.png",
			expectedMediaType: "image/png",
			expectedSize:    len(createTestImageData()),
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
			shouldFindCover: true,
			expectedID:      "cover_image_id",
			expectedName:    "cover_image.jpg",
			expectedMediaType: "image/jpeg",
			expectedSize:    len(createTestImageData()),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			epubPath := filepath.Join(tmpDir, tt.name+".epub")
			createTestZIP(t, epubPath, tt.epubContent)

			// Parse the EPUB to get the OPF Package
			zr, closer, err := reader.openZipReader(epubPath)
			if err != nil {
				t.Fatalf("failed to open epub: %v", err)
			}
			defer func() { _ = closer.Close() }()

			container, err := reader.ocfReader.Read(zr)
			if err != nil {
				t.Fatalf("failed to read container: %v", err)
			}

			pkg, err := reader.opfReader.Read(zr, container)
			if err != nil {
				t.Fatalf("failed to read opf: %v", err)
			}

			// Close for GetCover usage (it reopens the zip)
			_ = closer.Close()

			// Get the cover
			cover := reader.GetCover(epubPath, zr, pkg)

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
				if cover.GetData == nil {
					t.Errorf("expected GetData function to be set")
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
