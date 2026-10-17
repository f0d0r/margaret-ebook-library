package epub

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
	"golang.org/x/text/encoding/charmap"
)

// utf16BE encodes s as UTF-16BE with a BOM (as written by Windows tools).
func utf16BE(s string) string {
	var out []byte
	out = append(out, 0xFE, 0xFF)
	for _, u := range utf16.Encode([]rune(s)) {
		out = append(out, byte(u>>8), byte(u))
	}
	return string(out)
}

// TestEpubReadUTF16Entries covers the real-world EPUB layout where
// container.xml, the OPF package and the content documents are all stored
// as BOM-carrying UTF-16BE with encoding="UTF-16" declarations.
func TestEpubReadUTF16Entries(t *testing.T) {
	containerXML := `<?xml version="1.0" encoding="UTF-16"?>
<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`
	opusOPF := `<?xml version="1.0" encoding="UTF-16"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>UTF16 Könyv</dc:title>
  </metadata>
  <manifest>
    <item id="ch1" media-type="application/xhtml+xml" href="ch1.xhtml" />
  </manifest>
  <spine>
    <itemref idref="ch1" />
  </spine>
</package>`
	chapter := `<?xml version="1.0" encoding="UTF-16"?>` +
		`<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Глава</title></head>` +
		`<body><p>Привет, мир!</p></body></html>`
	chapterRaw := utf16BE(chapter)

	epubPath := filepath.Join(t.TempDir(), "utf16.epub")
	createTestZIP(t, epubPath, []testFile{
		{name: "mimetype", content: "application/epub+zip", method: zip.Store},
		{name: "META-INF/container.xml", content: utf16BE(containerXML), method: zip.Deflate},
		{name: "OEBPS/content.opf", content: utf16BE(opusOPF), method: zip.Deflate},
		{name: "OEBPS/ch1.xhtml", content: chapterRaw, method: zip.Deflate},
	})

	ebook, err := NewEpubReader(config.DefaultConfig()).Read(book.NewPathBlob(epubPath))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if got := ebook.Metadata().Title; got != "UTF16 Könyv" {
		t.Errorf("Title = %q, want UTF16 Könyv", got)
	}

	ro := ebook.Resources().ReadingOrder()
	if len(ro) != 1 {
		t.Fatalf("len(ReadingOrder) = %d, want 1", len(ro))
	}
	ch1 := ro[0].Resource
	if ch1.Charset != "utf-16be" {
		t.Errorf("Charset = %q, want utf-16be", ch1.Charset)
	}
	data, err := ch1.Data()
	if err != nil {
		t.Fatalf("Data() error: %v", err)
	}
	if !bytes.Equal(data, []byte(chapterRaw)) {
		t.Errorf("stored bytes not byte-identical (len got %d, want %d)", len(data), len(chapterRaw))
	}

	rc, err := ch1.OpenAs(context.Background(), mediatype.PlainText)
	if err != nil {
		t.Fatalf("OpenAs(plain text) error: %v", err)
	}
	text, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}
	if !strings.Contains(string(text), "Привет, мир!") {
		t.Errorf("decoded text %q missing Привет, мир!", text)
	}
	if strings.HasPrefix(string(text), "\uFEFF") {
		t.Error("decoded text still carries a BOM")
	}
}

func TestEpubReadReportsEntryCharsets(t *testing.T) {
	opfContent := `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Charset Book</dc:title>
  </metadata>
  <manifest>
    <item id="ch1" media-type="application/xhtml+xml" href="ch1.xhtml" />
    <item id="ch2" media-type="application/xhtml+xml" href="ch2.xhtml" />
    <item id="pic" media-type="image/png" href="pic.png" />
  </manifest>
  <spine>
    <itemref idref="ch1" />
    <itemref idref="ch2" />
  </spine>
</package>`

	cp1251Chapter, err := charmap.Windows1251.NewEncoder().Bytes([]byte(
		`<?xml version="1.0" encoding="windows-1251"?>` +
			`<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Глава</title></head>` +
			`<body><p>Привет, мир!</p></body></html>`))
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	utf8Chapter := `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>Plain</p></body></html>`

	epubPath := filepath.Join(t.TempDir(), "charset.epub")
	createTestZIP(t, epubPath, []testFile{
		{name: "mimetype", content: "application/epub+zip", method: zip.Store},
		{name: "META-INF/container.xml", content: createContainerXML("OEBPS/content.opf"), method: zip.Deflate},
		{name: "OEBPS/content.opf", content: opfContent, method: zip.Deflate},
		{name: "OEBPS/ch1.xhtml", content: string(cp1251Chapter), method: zip.Deflate},
		{name: "OEBPS/ch2.xhtml", content: utf8Chapter, method: zip.Deflate},
		{name: "OEBPS/pic.png", content: "\x89PNG\r\n\x1a\nfakepngdata", method: zip.Deflate},
	})

	ebook, err := NewEpubReader(config.DefaultConfig()).Read(book.NewPathBlob(epubPath))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	byID := make(map[string]*book.Resource)
	for _, r := range ebook.Resources().All() {
		byID[r.Id] = r
	}

	ch1 := byID["ch1"]
	if ch1 == nil {
		t.Fatal("ch1 resource missing")
	}
	if ch1.Charset != "windows-1251" {
		t.Errorf("ch1 Charset = %q, want windows-1251", ch1.Charset)
	}
	data, err := ch1.Data()
	if err != nil {
		t.Fatalf("ch1 Data() error: %v", err)
	}
	if !bytes.Equal(data, cp1251Chapter) {
		t.Errorf("ch1 stored bytes not byte-identical:\n got %q\nwant %q", data, cp1251Chapter)
	}
	if bytes.Contains(data, []byte("Привет")) {
		t.Error("ch1 stored bytes were silently transcoded to UTF-8")
	}
	rc, err := ch1.OpenAs(context.Background(), mediatype.PlainText)
	if err != nil {
		t.Fatalf("ch1 OpenAs(plain text) error: %v", err)
	}
	text, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("ch1 ReadAll error: %v", err)
	}
	if !strings.Contains(string(text), "Привет, мир!") {
		t.Errorf("ch1 decoded text %q missing Привет, мир!", text)
	}

	if got := byID["ch2"].Charset; got != book.CharsetUTF8 {
		t.Errorf("ch2 Charset = %q, want utf-8", got)
	}
	if got := byID["pic"].Charset; got != book.CharsetBinary {
		t.Errorf("pic Charset = %q, want binary", got)
	}
}
