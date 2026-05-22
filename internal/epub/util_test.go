package epub

import (
	"archive/zip"
	"os"
	"testing"
)

type testFile struct {
	name    string
	content string
	method  uint16
}

// createTestZIP creates a ZIP file with the specified files
func createTestZIP(t *testing.T, path string, files []testFile) {
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create zip file: %v", err)
	}
	defer func() {
		closeErr := f.Close()
		if closeErr != nil {
			t.Fatalf("failed to close zip file: %v", closeErr)
		}
	}()

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

// createContainerXML generates a container.xml file content
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
