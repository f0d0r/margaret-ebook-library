package ebook

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
)

func writeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create zip: %v", err)
	}
	w := zip.NewWriter(f)
	for name, content := range files {
		method := uint16(zip.Deflate)
		if name == "mimetype" {
			method = zip.Store
		}
		fw, err := w.CreateHeader(&zip.FileHeader{Name: name, Method: method})
		if err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write entry: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close zip: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("failed to close file: %v", err)
	}
}

const kepubAPIChapter = `<?xml version="1.0" encoding="utf-8"?>
<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Ch1</title></head><body>
<div id="book-columns"><div id="book-inner">
<p><span class="koboSpan" id="kobo.1.1">API kepub body.</span></p>
</div></div></body></html>`

func kepubAPIFiles() map[string]string {
	return map[string]string{
		"mimetype": "application/epub+zip",
		"META-INF/container.xml": `<?xml version="1.0" encoding="UTF-8"?>
<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`,
		"OEBPS/content.opf": `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>API Kepub Book</dc:title>
    <dc:creator>API Author</dc:creator>
    <dc:description>API description.</dc:description>
    <dc:language>en</dc:language>
  </metadata>
  <manifest>
    <item id="ch1" media-type="application/xhtml+xml" href="ch1.xhtml" />
    <item id="cover" media-type="image/jpeg" href="cover.jpg" properties="cover-image" />
  </manifest>
  <spine><itemref idref="ch1" /></spine>
</package>`,
		"OEBPS/ch1.xhtml": kepubAPIChapter,
		"OEBPS/cover.jpg": "fake-jpeg-data",
	}
}

// Public Read on a .kepub.epub filename returns a Book reporting EPUB.
func TestRead_KepubFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "book.kepub.epub")
	writeZip(t, path, kepubAPIFiles())

	b, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if b.FileType() != book.EPUB {
		t.Errorf("FileType = %q, want EPUB", b.FileType())
	}
	if b.Metadata().Title != "API Kepub Book" {
		t.Errorf("Title = %q", b.Metadata().Title)
	}
	if len(b.Resources().ReadingOrder()) != 1 {
		t.Errorf("len(ReadingOrder) = %d, want 1", len(b.Resources().ReadingOrder()))
	}
	if _, ok := b.Resources().CoverImage(); !ok {
		t.Error("CoverImage() missing")
	}
}

// Public Read surfaces DRM with ErrDRM for both EPUB and KEPUB shapes.
func TestRead_DRM(t *testing.T) {
	tmpDir := t.TempDir()

	adobe := kepubAPIFiles()
	adobe["META-INF/encryption.xml"] = `<?xml version="1.0" encoding="UTF-8"?>
<encryption xmlns="http://www.w3.org/2001/04/xmlenc#">
  <EncryptedData><EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes256-cbc" />
  <CipherData><CipherReference URI="OEBPS/ch1.xhtml" /></CipherData></EncryptedData>
</encryption>`
	adobePath := filepath.Join(tmpDir, "adobe.epub")
	writeZip(t, adobePath, adobe)
	if _, err := Read(adobePath); !errors.Is(err, ErrDRM) {
		t.Errorf("adobe DRM: Read() error = %v, want ErrDRM", err)
	}
	if _, err := Read(adobePath); !errors.Is(err, book.ErrDRM) {
		t.Errorf("adobe DRM: Read() error = %v, want book.ErrDRM", err)
	}

	kobo := kepubAPIFiles()
	kobo["OEBPS/ch1.xhtml"] = "\x00\x01\x02encrypted\xff\xfe"
	kobo["rights.xml"] = "<rights>locked</rights>"
	koboPath := filepath.Join(tmpDir, "kobo.kepub.epub")
	writeZip(t, koboPath, kobo)
	if _, err := Read(koboPath); !errors.Is(err, ErrDRM) {
		t.Errorf("kobo DRM: Read() error = %v, want ErrDRM", err)
	}
}
