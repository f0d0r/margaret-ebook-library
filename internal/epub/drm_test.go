package epub

import (
	"archive/zip"
	"errors"
	"path/filepath"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
)

const encryptionAdobeDRM = `<?xml version="1.0" encoding="UTF-8"?>
<encryption xmlns="http://www.w3.org/2001/04/xmlenc#">
  <EncryptedData>
    <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes256-cbc" />
    <CipherData><CipherReference URI="OEBPS/ch1.xhtml" /></CipherData>
  </EncryptedData>
</encryption>`

const encryptionFontObfuscationOnly = `<?xml version="1.0" encoding="UTF-8"?>
<encryption xmlns="http://www.w3.org/2001/04/xmlenc#">
  <EncryptedData>
    <EncryptionMethod Algorithm="http://ns.adobe.com/pdf/enc#RC" />
    <CipherData><CipherReference URI="OEBPS/font.otf" /></CipherData>
  </EncryptedData>
  <EncryptedData>
    <EncryptionMethod Algorithm="http://www.idpf.org/2008/embedding" />
    <CipherData><CipherReference URI="OEBPS/font2.otf" /></CipherData>
  </EncryptedData>
</encryption>`

const encryptionMixedDRM = `<?xml version="1.0" encoding="UTF-8"?>
<encryption xmlns="http://www.w3.org/2001/04/xmlenc#">
  <EncryptedData>
    <EncryptionMethod Algorithm="http://ns.adobe.com/pdf/enc#RC" />
    <CipherData><CipherReference URI="OEBPS/font.otf" /></CipherData>
  </EncryptedData>
  <EncryptedData>
    <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes128-cbc" />
    <CipherData><CipherReference URI="OEBPS/ch1.xhtml" /></CipherData>
  </EncryptedData>
</encryption>`

func TestAdobeDRMRejected(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "drm.epub")
	createKepub(t, p, "property", []testFile{
		{name: "META-INF/encryption.xml", content: encryptionAdobeDRM, method: zip.Deflate},
	})
	_, err := NewEpubReader(config.DefaultConfig()).Read(book.NewPathBlob(p))
	if !errors.Is(err, book.ErrDRM) {
		t.Errorf("Read() error = %v, want ErrDRM", err)
	}
}

func TestAdobeMixedObfuscationAndDRMRejected(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "mixed.epub")
	createKepub(t, p, "property", []testFile{
		{name: "META-INF/encryption.xml", content: encryptionMixedDRM, method: zip.Deflate},
	})
	_, err := NewEpubReader(config.DefaultConfig()).Read(book.NewPathBlob(p))
	if !errors.Is(err, book.ErrDRM) {
		t.Errorf("Read() error = %v, want ErrDRM", err)
	}
}

func TestFontObfuscationOnlyAccepted(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "obfuscated.epub")
	createKepub(t, p, "property", []testFile{
		{name: "META-INF/encryption.xml", content: encryptionFontObfuscationOnly, method: zip.Deflate},
	})
	ebook, err := NewEpubReader(config.DefaultConfig()).Read(book.NewPathBlob(p))
	if err != nil {
		t.Fatalf("Read() error = %v, want nil (font obfuscation is not DRM)", err)
	}
	if ebook.FileType() != book.EPUB {
		t.Errorf("FileType = %q, want EPUB", ebook.FileType())
	}
}

func TestMalformedEncryptionXMLRejected(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "malformed.epub")
	createKepub(t, p, "property", []testFile{
		{name: "META-INF/encryption.xml", content: "<encryption><unclosed", method: zip.Deflate},
	})
	_, err := NewEpubReader(config.DefaultConfig()).Read(book.NewPathBlob(p))
	if !errors.Is(err, book.ErrDRM) {
		t.Errorf("Read() error = %v, want ErrDRM", err)
	}
}

// A leftover rights.xml from a DeDRM tool with intact koboSpan content
// must NOT be rejected — rights.xml alone is not definitive (Calibre).
func TestKoboRightsXMLLeftoverAccepted(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "leftover.kepub.epub")
	createKepub(t, p, "property", []testFile{
		{name: "rights.xml", content: "<rights>leftover</rights>", method: zip.Deflate},
	})
	ebook, err := NewEpubReader(config.DefaultConfig()).Read(book.NewPathBlob(p))
	if err != nil {
		t.Fatalf("Read() error = %v, want nil (leftover rights.xml with valid content)", err)
	}
	if ebook.Metadata().Title != "Kepub Test Book" {
		t.Errorf("Title = %q", ebook.Metadata().Title)
	}
}

func TestKoboEmptyRightsXMLAccepted(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "empty.kepub.epub")
	createKepub(t, p, "property", []testFile{
		{name: "rights.xml", content: "", method: zip.Store},
	})
	if _, err := NewEpubReader(config.DefaultConfig()).Read(book.NewPathBlob(p)); err != nil {
		t.Errorf("Read() error = %v, want nil (empty rights.xml)", err)
	}
}

func TestKoboEncryptedContentRejected(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "kdrm.kepub.epub")
	createKepubWithChapter(t, p, "property", "\x00\x01\x02binary-encrypted-payload\xff\xfe\x00",
		[]testFile{
			{name: "rights.xml", content: "<rights><kdrm/></rights>", method: zip.Deflate},
		})
	_, err := NewEpubReader(config.DefaultConfig()).Read(book.NewPathBlob(p))
	if !errors.Is(err, book.ErrDRM) {
		t.Errorf("Read() error = %v, want ErrDRM", err)
	}
}

// Uppercase <HTML> is legal HTML5: with rights.xml present it must not be
// mistaken for encrypted content.
func TestKoboUppercaseHTMLAccepted(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "upper.kepub.epub")
	createKepubWithChapter(t, p, "property",
		`<HTML xmlns="http://www.w3.org/1999/xhtml"><HEAD><TITLE>Up</TITLE></HEAD><BODY><P>Uppercase markup.</P></BODY></HTML>`,
		[]testFile{
			{name: "rights.xml", content: "<rights>leftover</rights>", method: zip.Deflate},
		})
	if _, err := NewEpubReader(config.DefaultConfig()).Read(book.NewPathBlob(p)); err != nil {
		t.Errorf("Read() error = %v, want nil (uppercase HTML is not DRM)", err)
	}
}

// A directory entry named rights.xml must be ignored by the lookup.
func TestKoboRightsXMLDirectoryIgnored(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "direntry.kepub.epub")
	createKepub(t, p, "property", []testFile{
		{name: "x/rights.xml/", content: "", method: zip.Store},
	})
	if _, err := NewEpubReader(config.DefaultConfig()).Read(book.NewPathBlob(p)); err != nil {
		t.Errorf("Read() error = %v, want nil (rights.xml directory is not DRM)", err)
	}
}

// An emptied encryption.xml carries no encrypted content and must not be
// treated as DRM (e.g. left behind by a DeDRM tool).
func TestEmptyEncryptionXMLAccepted(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "empty-enc.epub")
	createKepub(t, p, "property", []testFile{
		{name: "META-INF/encryption.xml", content: "", method: zip.Store},
	})
	if _, err := NewEpubReader(config.DefaultConfig()).Read(book.NewPathBlob(p)); err != nil {
		t.Errorf("Read() error = %v, want nil (empty encryption.xml)", err)
	}
}

// Binary spine WITHOUT rights.xml is just a broken book, not DRM.
func TestBinarySpineWithoutRightsXMLNotDRM(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "binary.epub")
	createKepubWithChapter(t, p, "property", "\x00\x01\x02binary-payload\xff\xfe\x00", nil)
	ebook, err := NewEpubReader(config.DefaultConfig()).Read(book.NewPathBlob(p))
	if err != nil {
		t.Fatalf("Read() error = %v, want nil (no rights.xml, no DRM)", err)
	}
	if ebook.FileType() != book.EPUB {
		t.Errorf("FileType = %q, want EPUB", ebook.FileType())
	}
}
