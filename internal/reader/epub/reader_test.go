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
		t.Run(tt.name, rangeTest(tt.filePath, tt.want))
	}
}

func TestReadMetadataTitle(t *testing.T) {
	tmpDir := t.TempDir()
	reader := &EpubReader{}

	tests := []struct {
		name          string
		opfContent    string
		expectedTitle string
		shouldError   bool
	}{
		{
			name:          "Normal title",
			opfContent:    createOPF("My Amazing Book"),
			expectedTitle: "My Amazing Book",
			shouldError:   false,
		},
		{
			name:          "Title with dc prefix",
			opfContent:    createOPFWithPrefix("dc", "Book with DC Prefix"),
			expectedTitle: "Book with DC Prefix",
			shouldError:   false,
		},
		{
			name:          "Title with custom prefix",
			opfContent:    createOPFWithPrefix("custom", "Book with Custom Prefix"),
			expectedTitle: "Book with Custom Prefix",
			shouldError:   false,
		},
		{
			name:          "No title",
			opfContent:    createOPFWithoutTitle(),
			expectedTitle: "",
			shouldError:   false,
		},
		{
			name:          "Multiple titles - first wins",
			opfContent:    createOPFWithMultipleTitles([]string{"First Title", "Second Title"}),
			expectedTitle: "First Title",
			shouldError:   false,
		},
		{
			name:          "Empty title element",
			opfContent:    createOPFWithEmptyTitle(),
			expectedTitle: "",
			shouldError:   false,
		},
		{
			name:          "Title with attributes",
			opfContent:    createOPFWithTitleAttributes("Title with ID", "ch1", "en"),
			expectedTitle: "Title with ID",
			shouldError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			epubPath := filepath.Join(tmpDir, tt.name+".epub")
			createValidTestEPUBWithContent(t, epubPath, tt.opfContent)

			metadata, err := reader.ReadMetadata(epubPath)

			if tt.shouldError && err == nil {
				t.Errorf("expected error but got none")
				return
			}
			if !tt.shouldError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if metadata.Title != tt.expectedTitle {
				t.Errorf("got title %q, want %q", metadata.Title, tt.expectedTitle)
			}
		})
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

func createOPF(title string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>` + title + `</dc:title>
  </metadata>
</package>`
}

func createOPFWithPrefix(prefix, title string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:` + prefix + `="http://purl.org/dc/elements/1.1/">
    <` + prefix + `:title>` + title + `</` + prefix + `:title>
  </metadata>
</package>`
}

func createOPFWithoutTitle() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:creator>Some Author</dc:creator>
  </metadata>
</package>`
}

func createOPFWithMultipleTitles(titles []string) string {
	metadataContent := `  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">`
	for _, title := range titles {
		metadataContent += "\n    <dc:title>" + title + "</dc:title>"
	}
	metadataContent += "\n  </metadata>"

	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
` + metadataContent + `
</package>`
}

func createOPFWithEmptyTitle() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title></dc:title>
  </metadata>
</package>`
}

func createOPFWithTitleAttributes(title, id, lang string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title id="` + id + `" lang="` + lang + `">` + title + `</dc:title>
  </metadata>
</package>`
}

func createContainerXML(opfPath string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0">
  <rootfiles>
    <rootfile full-path="` + opfPath + `" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`
}

// createValidTestEPUB creates a minimal but valid EPUB structure with proper container and OPF files
func createValidTestEPUB(t *testing.T, path string, title string) {
	opfPath := "OEBPS/content.opf"
	opfContent := createOPF(title)
	containerContent := createContainerXML(opfPath)

	createTestZIP(t, path, []testFile{
		{name: "mimetype", content: "application/epub+zip", method: zip.Store},
		{name: "META-INF/container.xml", content: containerContent, method: zip.Deflate},
		{name: opfPath, content: opfContent, method: zip.Deflate},
	})
}

// createValidTestEPUBWithContent creates a valid EPUB with custom OPF content
func createValidTestEPUBWithContent(t *testing.T, path string, opfContent string) {
	opfPath := "OEBPS/content.opf"
	containerContent := createContainerXML(opfPath)

	createTestZIP(t, path, []testFile{
		{name: "mimetype", content: "application/epub+zip", method: zip.Store},
		{name: "META-INF/container.xml", content: containerContent, method: zip.Deflate},
		{name: opfPath, content: opfContent, method: zip.Deflate},
	})
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
