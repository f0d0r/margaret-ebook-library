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

func TestReadMetadataAuthors(t *testing.T) {
	tmpDir := t.TempDir()
	reader := &EpubReader{}

	tests := []struct {
		name             string
		opfContent       string
		expectedAuthors  []string
		shouldError      bool
	}{
		{
			name:             "Single author",
			opfContent:       createOPFWithSingleAuthor("Jane Austen"),
			expectedAuthors:  []string{"Jane Austen"},
			shouldError:      false,
		},
		{
			name:             "Multiple authors",
			opfContent:       createOPFWithMultipleAuthors([]string{"Stephen King", "Peter Straub"}),
			expectedAuthors:  []string{"Stephen King", "Peter Straub"},
			shouldError:      false,
		},
		{
			name:             "No authors",
			opfContent:       createOPFWithoutAuthors(),
			expectedAuthors:  []string{},
			shouldError:      false,
		},
		{
			name:             "Author with whitespace",
			opfContent:       createOPFWithSingleAuthor("   J.K. Rowling   "),
			expectedAuthors:  []string{"J.K. Rowling"},
			shouldError:      false,
		},
		{
			name:             "Empty author element",
			opfContent:       createOPFWithEmptyAuthor(),
			expectedAuthors:  []string{},
			shouldError:      false,
		},
		{
			name:             "Mixed empty and non-empty authors",
			opfContent:       createOPFWithMixedAuthors([]string{"Author One", "", "Author Two"}),
			expectedAuthors:  []string{"Author One", "Author Two"},
			shouldError:      false,
		},
		{
			name:             "Author with aut role",
			opfContent:       createOPFWithAuthorRole("George Orwell", "aut"),
			expectedAuthors:  []string{"George Orwell"},
			shouldError:      false,
		},
		{
			name:             "Author with file-as attribute",
			opfContent:       createOPFWithAuthorFileAs("George Orwell", "Orwell, George"),
			expectedAuthors:  []string{"George Orwell"},
			shouldError:      false,
		},
		{
			name:             "Multiple creators - only authors included",
			opfContent:       createOPFWithCreatorsMultipleRoles([]creatorInfo{
				{name: "Isaac Asimov", role: "aut"},
				{name: "Ralph Macchio", role: "ill"},
				{name: "Penthouse Press", role: "pbl"},
			}),
			expectedAuthors:  []string{"Isaac Asimov"},
			shouldError:      false,
		},
		{
			name:             "Author with dc prefix",
			opfContent:       createOPFWithAuthorDcPrefix("Haruki Murakami"),
			expectedAuthors:  []string{"Haruki Murakami"},
			shouldError:      false,
		},
		{
			name:             "Creator with no role (defaults to author)",
			opfContent:       createOPFWithCreatorsMultipleRoles([]creatorInfo{
				{name: "Main Author", role: ""},
				{name: "Illustrator", role: "ill"},
			}),
			expectedAuthors:  []string{"Main Author"},
			shouldError:      false,
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

			if len(metadata.Authors) != len(tt.expectedAuthors) {
				t.Errorf("got %d authors, want %d. Got: %v, Want: %v",
					len(metadata.Authors), len(tt.expectedAuthors), metadata.Authors, tt.expectedAuthors)
				return
			}

			for i, author := range metadata.Authors {
				if author != tt.expectedAuthors[i] {
					t.Errorf("author %d: got %q, want %q", i, author, tt.expectedAuthors[i])
				}
			}
		})
	}
}

func TestReadMetadataDescription(t *testing.T) {
	tmpDir := t.TempDir()
	reader := &EpubReader{}

	tests := []struct {
		name                string
		opfContent          string
		expectedDescription string
		shouldError         bool
	}{
		{
			name:                "Simple description",
			opfContent:          createOPFWithDescription("A great adventure story"),
			expectedDescription: "A great adventure story",
			shouldError:         false,
		},
		{
			name:                "No description",
			opfContent:          createOPFWithoutDescription(),
			expectedDescription: "",
			shouldError:         false,
		},
		{
			name:                "Description with whitespace",
			opfContent:          createOPFWithDescription("   A book with spaces   "),
			expectedDescription: "A book with spaces",
			shouldError:         false,
		},
		{
			name:                "Empty description element",
			opfContent:          createOPFWithEmptyDescription(),
			expectedDescription: "",
			shouldError:         false,
		},
		{
			name:                "Multiple descriptions - first non-empty wins",
			opfContent:          createOPFWithMultipleDescriptions([]string{"", "First Description", "Second Description"}),
			expectedDescription: "First Description",
			shouldError:         false,
		},
		{
			name:                "Description with HTML tags",
			opfContent:          createOPFWithDescription("This is <b>bold</b> text"),
			expectedDescription: "This is  text",
			shouldError:         false,
		},
		{
			name:                "Description with HTML entities",
			opfContent:          createOPFWithDescription("Pride &amp; Prejudice"),
			expectedDescription: "Pride & Prejudice",
			shouldError:         false,
		},
		{
			name:                "Description with newlines",
			opfContent:          createOPFWithDescription("Line one\nLine two\nLine three"),
			expectedDescription: "Line one\nLine two\nLine three",
			shouldError:         false,
		},
		{
			name:                "Description with dc prefix",
			opfContent:          createOPFWithDescriptionDcPrefix("A sci-fi novel"),
			expectedDescription: "A sci-fi novel",
			shouldError:         false,
		},
		{
			name:                "Long description",
			opfContent:          createOPFWithDescription("This is a long description that spans multiple sentences. It describes the plot, characters, and themes of the book in detail. The reader should get a good understanding of what the book is about from reading this description."),
			expectedDescription: "This is a long description that spans multiple sentences. It describes the plot, characters, and themes of the book in detail. The reader should get a good understanding of what the book is about from reading this description.",
			shouldError:         false,
		},
		{
			name:                "Description with multiple spaces",
			opfContent:          createOPFWithDescription("A    tale  of   great    adventure"),
			expectedDescription: "A    tale  of   great    adventure",
			shouldError:         false,
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

			if metadata.Description != tt.expectedDescription {
				t.Errorf("got description %q, want %q", metadata.Description, tt.expectedDescription)
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

// Author helper functions for tests

func createOPFWithSingleAuthor(author string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>
    <dc:creator>` + author + `</dc:creator>
  </metadata>
</package>`
}

func createOPFWithMultipleAuthors(authors []string) string {
	metadataContent := `  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>`
	for _, author := range authors {
		metadataContent += "\n    <dc:creator>" + author + "</dc:creator>"
	}
	metadataContent += "\n  </metadata>"

	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
` + metadataContent + `
</package>`
}

func createOPFWithoutAuthors() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>
  </metadata>
</package>`
}

func createOPFWithEmptyAuthor() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>
    <dc:creator></dc:creator>
  </metadata>
</package>`
}

func createOPFWithMixedAuthors(authors []string) string {
	metadataContent := `  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>`
	for _, author := range authors {
		metadataContent += "\n    <dc:creator>" + author + "</dc:creator>"
	}
	metadataContent += "\n  </metadata>"

	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
` + metadataContent + `
</package>`
}

func createOPFWithAuthorRole(author, role string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>
    <dc:creator role="` + role + `">` + author + `</dc:creator>
  </metadata>
</package>`
}

func createOPFWithAuthorFileAs(author, fileAs string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>
    <dc:creator file-as="` + fileAs + `">` + author + `</dc:creator>
  </metadata>
</package>`
}

type creatorInfo struct {
	name string
	role string
}

func createOPFWithCreatorsMultipleRoles(creators []creatorInfo) string {
	metadataContent := `  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>`
	for _, creator := range creators {
		metadataContent += "\n    <dc:creator role=\"" + creator.role + "\">" + creator.name + "</dc:creator>"
	}
	metadataContent += "\n  </metadata>"

	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
` + metadataContent + `
</package>`
}

func createOPFWithAuthorDcPrefix(author string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" version="3.0">
  <metadata>
    <dc:title>Test Book</dc:title>
    <dc:creator>` + author + `</dc:creator>
  </metadata>
</package>`
}

// Description helper functions for tests

func createOPFWithDescription(description string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>
    <dc:description>` + description + `</dc:description>
  </metadata>
</package>`
}

func createOPFWithoutDescription() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>
  </metadata>
</package>`
}

func createOPFWithEmptyDescription() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>
    <dc:description></dc:description>
  </metadata>
</package>`
}

func createOPFWithMultipleDescriptions(descriptions []string) string {
	metadataContent := `  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>`
	for _, desc := range descriptions {
		metadataContent += "\n    <dc:description>" + desc + "</dc:description>"
	}
	metadataContent += "\n  </metadata>"

	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
` + metadataContent + `
</package>`
}

func createOPFWithDescriptionDcPrefix(description string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" version="3.0">
  <metadata>
    <dc:title>Test Book</dc:title>
    <dc:description>` + description + `</dc:description>
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
