package epub

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
)

func TestReadIdentifiers(t *testing.T) {
	const opfContent = `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="pub-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Identifier Book</dc:title>
    <dc:identifier id="pub-id">urn:uuid:A1B0D67E-2E81-4DF5-9E67-A64CBE366809</dc:identifier>
    <dc:identifier id="isbn">urn:isbn:978-0-306-40615-7</dc:identifier>
    <meta refines="#isbn" property="identifier-type" scheme="onix:codelist5">15</meta>
    <dc:source id="src-id">urn:isbn:9780375704024</dc:source>
    <meta refines="#src-id" property="identifier-type" scheme="onix:codelist5">15</meta>
  </metadata>
</package>`

	epubPath := filepath.Join(t.TempDir(), "identifiers.epub")
	createValidTestEPUBWithContent(t, epubPath, opfContent)

	ebook, err := NewEpubReader(config.DefaultConfig()).Read(book.NewPathBlob(epubPath))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	want := []book.Identifier{
		{Scheme: "UUID", Value: "A1B0D67E-2E81-4DF5-9E67-A64CBE366809"},
		{Scheme: "ISBN", Value: "978-0-306-40615-7"},
		{Scheme: "ISBN", Value: "9780375704024"},
	}
	if got := ebook.Metadata().Identifiers; !reflect.DeepEqual(got, want) {
		t.Errorf("Identifiers = %+v, want %+v", got, want)
	}
	if isbn, ok := ebook.Metadata().Identifier("ISBN"); !ok || isbn != "978-0-306-40615-7" {
		t.Errorf("Identifier(ISBN) = %q, %v, want ISBN value, true", isbn, ok)
	}
}
