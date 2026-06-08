package epub

import (
	"encoding/xml"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadMetadataTitle(t *testing.T) {
	tmpDir := t.TempDir()
	reader := NewEpubReader()

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
	reader := NewEpubReader()

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
	reader := NewEpubReader()

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

func TestReadMetadataLanguages(t *testing.T) {
	tmpDir := t.TempDir()
	reader := NewEpubReader()

	tests := []struct {
		name              string
		opfContent        string
		expectedLanguages []string
		shouldError       bool
	}{
		{
			name:              "Single language - en",
			opfContent:        createOPFWithLanguages([]string{"en"}),
			expectedLanguages: []string{"en"},
			shouldError:       false,
		},
		{
			name:              "Single language - hu",
			opfContent:        createOPFWithLanguages([]string{"hu"}),
			expectedLanguages: []string{"hu"},
			shouldError:       false,
		},
		{
			name:              "Language with region - en-US",
			opfContent:        createOPFWithLanguages([]string{"en-US"}),
			expectedLanguages: []string{"en-US"},
			shouldError:       false,
		},
		{
			name:              "Complex BCP 47 - zh-Hans-CN",
			opfContent:        createOPFWithLanguages([]string{"zh-Hans-CN"}),
			expectedLanguages: []string{"zh-Hans-CN"},
			shouldError:       false,
		},
		{
			name:              "Multiple languages",
			opfContent:        createOPFWithLanguages([]string{"en", "hu", "de"}),
			expectedLanguages: []string{"en", "hu", "de"},
			shouldError:       false,
		},
		{
			name:              "No languages",
			opfContent:        createOPFWithoutLanguages(),
			expectedLanguages: nil,
			shouldError:       false,
		},
		{
			name:              "Invalid language - und",
			opfContent:        createOPFWithLanguages([]string{"und"}),
			expectedLanguages: nil,
			shouldError:       false,
		},
		{
			name:              "Invalid language - english",
			opfContent:        createOPFWithLanguages([]string{"english"}),
			expectedLanguages: nil,
			shouldError:       false,
		},
		{
			name:              "Invalid language - magyar",
			opfContent:        createOPFWithLanguages([]string{"magyar"}),
			expectedLanguages: nil,
			shouldError:       false,
		},
		{
			name:              "Mixed valid and invalid languages",
			opfContent:        createOPFWithLanguages([]string{"en", "und", "hu", "english"}),
			expectedLanguages: []string{"en", "hu"},
			shouldError:       false,
		},
		{
			name:              "Empty language element",
			opfContent:        createOPFWithLanguages([]string{""}),
			expectedLanguages: nil,
			shouldError:       false,
		},
		{
			name:              "Languages with whitespace",
			opfContent:        createOPFWithLanguagesWhitespace([]string{"  en  ", "  hu  "}),
			expectedLanguages: []string{"en", "hu"},
			shouldError:       false,
		},
		{
			name:              "Language with dc prefix",
			opfContent:        createOPFWithLanguageDcPrefix("fr"),
			expectedLanguages: []string{"fr"},
			shouldError:       false,
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

			if len(metadata.Languages) != len(tt.expectedLanguages) {
				t.Errorf("got %d languages, want %d. Got: %v, Want: %v",
					len(metadata.Languages), len(tt.expectedLanguages), metadata.Languages, tt.expectedLanguages)
				return
			}

			for i, lang := range metadata.Languages {
				if lang != tt.expectedLanguages[i] {
					t.Errorf("language %d: got %q, want %q", i, lang, tt.expectedLanguages[i])
				}
			}
		})
	}
}

// OPF helper functions for tests

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

func createOPFWithLanguages(languages []string) string {
	metadataContent := `  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>`
	for _, lang := range languages {
		metadataContent += "\n    <dc:language>" + lang + "</dc:language>"
	}
	metadataContent += "\n  </metadata>"

	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
` + metadataContent + `
</package>`
}

func createOPFWithLanguagesWhitespace(languages []string) string {
	metadataContent := `  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>`
	for _, lang := range languages {
		metadataContent += "\n    <dc:language>" + lang + "</dc:language>"
	}
	metadataContent += "\n  </metadata>"

	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
` + metadataContent + `
</package>`
}

func createOPFWithoutLanguages() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>
  </metadata>
</package>`
}

func createOPFWithLanguageDcPrefix(language string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" version="3.0">
  <metadata>
    <dc:title>Test Book</dc:title>
    <dc:language>` + language + `</dc:language>
  </metadata>
</package>`
}

func TestGetMetaByName(t *testing.T) {
	opf := NewOpfReader()
	tests := []struct {
		name           string
		opfContent     string
		searchName     string
		expectedMeta   *Meta
		shouldFind     bool
	}{
		{
			name:       "Meta with matching name",
			opfContent: createOPFWithMeta("calibre", "content-value"),
			searchName: "calibre",
			expectedMeta: &Meta{
				Name:    "calibre",
				Content: "content-value",
			},
			shouldFind: true,
		},
		{
			name:       "Meta with matching name and property",
			opfContent: createOPFWithMetaProperty("identifier-type", "uuid", "uuid:12345"),
			searchName: "identifier-type",
			expectedMeta: &Meta{
				Name:     "identifier-type",
				Property: "uuid",
				Content:  "uuid:12345",
			},
			shouldFind: true,
		},
		{
			name:           "Meta not found",
			opfContent:     createOPFWithMeta("calibre", "value"),
			searchName:     "non-existent",
			expectedMeta:   nil,
			shouldFind:     false,
		},
		{
			name:           "Empty metadata - no metas",
			opfContent:     createOPF("Test Book"),
			searchName:     "calibre",
			expectedMeta:   nil,
			shouldFind:     false,
		},
		{
			name:       "Multiple metas - find specific one",
			opfContent: createOPFWithMultipleMetas([]MetaData{
				{Name: "meta1", Content: "value1"},
				{Name: "meta2", Content: "value2"},
				{Name: "meta3", Content: "value3"},
			}),
			searchName: "meta2",
			expectedMeta: &Meta{
				Name:    "meta2",
				Content: "value2",
			},
			shouldFind: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := parseOPFContent(t, tt.opfContent)
			result := opf.GetMetaByName(pkg, tt.searchName)

			if tt.shouldFind {
				if result == nil {
					t.Errorf("expected to find meta with name %q, but got nil", tt.searchName)
					return
				}
				if result.Name != tt.expectedMeta.Name {
					t.Errorf("got meta name %q, want %q", result.Name, tt.expectedMeta.Name)
				}
				if result.Content != tt.expectedMeta.Content {
					t.Errorf("got meta content %q, want %q", result.Content, tt.expectedMeta.Content)
				}
				if tt.expectedMeta.Property != "" && result.Property != tt.expectedMeta.Property {
					t.Errorf("got meta property %q, want %q", result.Property, tt.expectedMeta.Property)
				}
			} else {
				if result != nil {
					t.Errorf("expected nil, but got meta: %+v", result)
				}
			}
		})
	}
}

func TestGetItemById(t *testing.T) {
	opf := NewOpfReader()
	tests := []struct {
		name         string
		opfContent   string
		searchID     string
		expectedItem *Item
		shouldFind   bool
	}{
		{
			name:       "Item with matching ID",
			opfContent: createOPFWithManifestItem("item1", "text/html", "chapter1.html", ""),
			searchID:   "item1",
			expectedItem: &Item{
				ID:        "item1",
				MediaType: "text/html",
				Href:      "chapter1.html",
			},
			shouldFind: true,
		},
		{
			name:       "Item with matching ID and properties",
			opfContent: createOPFWithManifestItem("ncx", "application/x-dtbncx+xml", "toc.ncx", "nav"),
			searchID:   "ncx",
			expectedItem: &Item{
				ID:         "ncx",
				MediaType:  "application/x-dtbncx+xml",
				Href:       "toc.ncx",
				Properties: "nav",
			},
			shouldFind: true,
		},
		{
			name:         "Item not found",
			opfContent:   createOPFWithManifestItem("item1", "text/html", "chapter1.html", ""),
			searchID:     "non-existent",
			expectedItem: nil,
			shouldFind:   false,
		},
		{
			name:         "Empty manifest - no items",
			opfContent:   createOPF("Test Book"),
			searchID:     "item1",
			expectedItem: nil,
			shouldFind:   false,
		},
		{
			name:       "Multiple items - find specific one",
			opfContent: createOPFWithMultipleItems([]ItemData{
				{ID: "item1", MediaType: "text/html", Href: "chapter1.html", Properties: ""},
				{ID: "item2", MediaType: "text/html", Href: "chapter2.html", Properties: ""},
				{ID: "item3", MediaType: "image/jpeg", Href: "image.jpg", Properties: ""},
			}),
			searchID: "item2",
			expectedItem: &Item{
				ID:        "item2",
				MediaType: "text/html",
				Href:      "chapter2.html",
			},
			shouldFind: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := parseOPFContent(t, tt.opfContent)
			result := opf.GetItemById(pkg, tt.searchID)

			if tt.shouldFind {
				if result == nil {
					t.Errorf("expected to find item with ID %q, but got nil", tt.searchID)
					return
				}
				if result.ID != tt.expectedItem.ID {
					t.Errorf("got item ID %q, want %q", result.ID, tt.expectedItem.ID)
				}
				if result.MediaType != tt.expectedItem.MediaType {
					t.Errorf("got item media type %q, want %q", result.MediaType, tt.expectedItem.MediaType)
				}
				if result.Href != tt.expectedItem.Href {
					t.Errorf("got item href %q, want %q", result.Href, tt.expectedItem.Href)
				}
				if tt.expectedItem.Properties != "" && result.Properties != tt.expectedItem.Properties {
					t.Errorf("got item properties %q, want %q", result.Properties, tt.expectedItem.Properties)
				}
			} else {
				if result != nil {
					t.Errorf("expected nil, but got item: %+v", result)
				}
			}
		})
	}
}

func TestGetItemsByProperty(t *testing.T) {
	opf := NewOpfReader()
	tests := []struct {
		name             string
		opfContent       string
		searchProperty   string
		expectedItems    []Item
		expectedCount    int
	}{
		{
			name:           "Items with matching property",
			opfContent:     createOPFWithMultipleItems([]ItemData{
				{ID: "item1", MediaType: "text/html", Href: "chapter1.html", Properties: "svg"},
				{ID: "item2", MediaType: "text/html", Href: "chapter2.html", Properties: ""},
				{ID: "item3", MediaType: "image/svg+xml", Href: "image.svg", Properties: "svg"},
			}),
			searchProperty: "svg",
			expectedCount:  2,
		},
		{
			name:           "Single item with property",
			opfContent:     createOPFWithManifestItem("cover", "image/jpeg", "cover.jpg", "cover-image"),
			searchProperty: "cover-image",
			expectedCount:  1,
		},
		{
			name:           "Property with space in properties",
			opfContent:     createOPFWithMultipleItems([]ItemData{
				{ID: "item1", MediaType: "text/html", Href: "chapter1.html", Properties: "scripted remote-resources"},
				{ID: "item2", MediaType: "text/html", Href: "chapter2.html", Properties: "remote-resources"},
				{ID: "item3", MediaType: "text/html", Href: "chapter3.html", Properties: "scripted"},
			}),
			searchProperty: "remote-resources",
			expectedCount:  2,
		},
		{
			name:           "No items with property",
			opfContent:     createOPFWithMultipleItems([]ItemData{
				{ID: "item1", MediaType: "text/html", Href: "chapter1.html", Properties: ""},
				{ID: "item2", MediaType: "text/html", Href: "chapter2.html", Properties: ""},
			}),
			searchProperty: "nav",
			expectedCount:  0,
		},
		{
			name:           "Empty manifest",
			opfContent:     createOPF("Test Book"),
			searchProperty: "nav",
			expectedCount:  0,
		},
		{
			name:           "Property search is case-sensitive",
			opfContent:     createOPFWithManifestItem("item1", "text/html", "chapter1.html", "NAV"),
			searchProperty: "nav",
			expectedCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := parseOPFContent(t, tt.opfContent)
			results := opf.GetItemsByProperty(pkg, tt.searchProperty)

			if len(results) != tt.expectedCount {
				t.Errorf("got %d items with property %q, want %d", len(results), tt.searchProperty, tt.expectedCount)
				return
			}

			for _, item := range results {
				if !strings.Contains(item.Properties, tt.searchProperty) {
					t.Errorf("item %q has properties %q, but doesn't contain %q", item.ID, item.Properties, tt.searchProperty)
				}
			}
		})
	}
}

// Helper types and functions for tests

type MetaData struct {
	Name     string
	Property string
	Content  string
}

type ItemData struct {
	ID        string
	MediaType string
	Href      string
	Properties string
}

func parseOPFContent(t *testing.T, opfContent string) Package {
	var pkg Package
	if err := xml.Unmarshal([]byte(opfContent), &pkg); err != nil {
		t.Fatalf("failed to unmarshal OPF content: %v", err)
	}
	return pkg
}

func createOPFWithMeta(name, content string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>
    <meta name="` + name + `" content="` + content + `" />
  </metadata>
</package>`
}

func createOPFWithMetaProperty(name, property, content string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>
    <meta name="` + name + `" property="` + property + `" content="` + content + `" />
  </metadata>
</package>`
}

func createOPFWithMultipleMetas(metas []MetaData) string {
	metadataContent := `  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>`
	for _, meta := range metas {
		if meta.Property != "" {
			metadataContent += "\n    <meta name=\"" + meta.Name + "\" property=\"" + meta.Property + "\" content=\"" + meta.Content + "\" />"
		} else {
			metadataContent += "\n    <meta name=\"" + meta.Name + "\" content=\"" + meta.Content + "\" />"
		}
	}
	metadataContent += "\n  </metadata>"

	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
` + metadataContent + `
</package>`
}

func createOPFWithManifestItem(id, mediaType, href, properties string) string {
	propertiesAttr := ""
	if properties != "" {
		propertiesAttr = ` properties="` + properties + `"`
	}

	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>
  </metadata>
  <manifest>
    <item id="` + id + `" media-type="` + mediaType + `" href="` + href + `"` + propertiesAttr + ` />
  </manifest>
</package>`
}

func createOPFWithMultipleItems(items []ItemData) string {
	manifestContent := `  <manifest>`
	for _, item := range items {
		propertiesAttr := ""
		if item.Properties != "" {
			propertiesAttr = ` properties="` + item.Properties + `"`
		}
		manifestContent += "\n    <item id=\"" + item.ID + "\" media-type=\"" + item.MediaType + "\" href=\"" + item.Href + "\"" + propertiesAttr + " />"
	}
	manifestContent += "\n  </manifest>"

	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>
  </metadata>
` + manifestContent + `
</package>`
}
