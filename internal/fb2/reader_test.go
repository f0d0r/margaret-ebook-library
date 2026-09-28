package fb2

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
)

const minimalFB2 = `<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0" xmlns:l="http://www.w3.org/1999/xlink">
  <description><title-info><book-title>Test</book-title></title-info></description>
  <body><section><p>Hello</p></section></body>
</FictionBook>`

func TestFb2ReaderSupportsPlain(t *testing.T) {
	reader := NewFb2Reader(config.DefaultConfig())

	tests := []struct {
		name    string
		content []byte
		want    bool
	}{
		{"minimal", []byte(minimalFB2), true},
		{"leading whitespace and comment", []byte("\n  <!-- hello -->\n" + minimalFB2), true},
		{"utf8 BOM", append([]byte{0xEF, 0xBB, 0xBF}, []byte(minimalFB2)...), true},
		{"windows-1251 declaration", []byte("<?xml version=\"1.0\" encoding=\"windows-1251\"?>\n<FictionBook><body/></FictionBook>"), true},
		{"2.1 namespace", []byte(`<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.1"><body/></FictionBook>`), true},
		{"no namespace", []byte(`<FictionBook><description/><body><p>x</p></body></FictionBook>`), true},
		{"namespace prefix", []byte(`<fb:FictionBook xmlns:fb="http://www.gribuser.ru/xml/fictionbook/2.0"><fb:body/></fb:FictionBook>`), true},
		{"doctype before root", []byte("<!DOCTYPE FictionBook SYSTEM \"fictionbook.dtd\">\n<FictionBook><body/></FictionBook>"), true},
		{"processing instruction before root", []byte("<?xml-stylesheet type=\"text/css\" href=\"x.css\"?>\n<FictionBook><body/></FictionBook>"), true},
		{"stray NUL bytes", bytes.ReplaceAll([]byte(minimalFB2), []byte("Book"), []byte("Bo\x00ok")), true},
		{"utf16le", encodeUTF16LE(minimalFB2), true},
		{"self-closing root", []byte(`<FictionBook/>`), true},
		{"root with attributes across lines", []byte("<FictionBook\n xmlns=\"http://www.gribuser.ru/xml/fictionbook/2.0\">\n<body/>\n</FictionBook>"), true},

		{"empty", nil, false},
		{"plain text", []byte("just some text"), false},
		{"html", []byte("<html><body><p>hi</p></body></html>"), false},
		{"xml without FictionBook", []byte(`<?xml version="1.0"?><library><book>x</book></library>`), false},
		{"prose mention is not a tag", []byte("<p>I love FictionBook format</p>"), false},
		{"longer tag name", []byte("<FictionBookmark>"), false},
		{"lowercase tag", []byte("<fictionbook><body/></fictionbook>"), false},
		{"closing tag only", []byte("<!-- x --></FictionBook>"), false},
		{"truncated zip header", []byte{0x50, 0x4B, 0x03, 0x04, 0x00}, false},
		{"mobi magic", append(make([]byte, 60), []byte("BOOKMOBI")...), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reader.Supports(book.NewBytesBlob(tt.content)); got != tt.want {
				t.Errorf("Supports() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFb2ReaderSupportsZipped(t *testing.T) {
	reader := NewFb2Reader(config.DefaultConfig())

	tests := []struct {
		name  string
		files []zipEntry
		want  bool
	}{
		{"single fb2 entry", []zipEntry{{"book.fb2", minimalFB2}}, true},
		{"uppercase extension", []zipEntry{{"BOOK.FB2", minimalFB2}}, true},
		{"nested fb2 entry", []zipEntry{{"books/book.fb2", minimalFB2}}, true},
		{"no fb2 name falls back to first entry", []zipEntry{{"book.xml", minimalFB2}}, true},
		{"readme plus fb2", []zipEntry{{"readme.txt", "hello"}, {"book.fb2", minimalFB2}}, true},
		{"broken first fb2 rejects even with valid second", []zipEntry{{"a.fb2", "not xml at all"}, {"b.fb2", minimalFB2}}, false},
		{"only sixth fb2 valid still rejects", []zipEntry{
			{"1.fb2", "x"}, {"2.fb2", "x"}, {"3.fb2", "x"}, {"4.fb2", "x"}, {"5.fb2", "x"}, {"6.fb2", minimalFB2},
		}, false},
		{"fallback sniffs beyond first unnamed entry", []zipEntry{
			{"a.txt", "hello"}, {"b.txt", "hello"}, {"c.xml", minimalFB2},
		}, true},
		{"fallback stops after five unnamed entries", []zipEntry{
			{"1.txt", "x"}, {"2.txt", "x"}, {"3.txt", "x"}, {"4.txt", "x"}, {"5.txt", "x"}, {"6.txt", "x"}, {"book.xml", minimalFB2},
		}, false},
		{"directory entries skipped", []zipEntry{{"dir/", ""}, {"book.fb2", minimalFB2}}, true},
		{"directory not counted against fallback cap", []zipEntry{{"dir/", ""}, {"a.txt", "x"}, {"book.xml", minimalFB2}}, true},
		{"epub-like zip is not fb2", []zipEntry{
			{"mimetype", "application/epub+zip"},
			{"META-INF/container.xml", `<container version="1.0"/>`},
		}, false},
		{"zip of html", []zipEntry{{"index.html", "<html><body>hi</body></html>"}}, false},
		{"zip of prose mentioning fb2", []zipEntry{{"note.fb2", "<p>FictionBook is great</p>"}}, false},
		{"empty zip", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reader.Supports(buildZipBlob(t, tt.files)); got != tt.want {
				t.Errorf("Supports() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFb2ReaderSupportsPreservesPosition(t *testing.T) {
	tmpDir := t.TempDir()
	reader := NewFb2Reader(config.DefaultConfig())

	path := filepath.Join(tmpDir, "book.fb2")
	if err := os.WriteFile(path, []byte(minimalFB2), 0644); err != nil {
		t.Fatalf("failed to write FB2 file: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Seek(10, io.SeekCurrent); err != nil {
		t.Fatalf("failed to seek: %v", err)
	}

	b, err := book.NewFileBlob(f)
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

func TestFb2ReaderRead(t *testing.T) {
	reader := NewFb2Reader(config.DefaultConfig())

	checkBook := func(t *testing.T, ebook book.Book) {
		t.Helper()
		if ebook.FileType() != book.FB2 {
			t.Errorf("FileType() = %q, want %q", ebook.FileType(), book.FB2)
		}
		if ebook.Version() != "2.0" {
			t.Errorf("Version() = %q, want %q", ebook.Version(), "2.0")
		}
		md := ebook.Metadata()
		if md.Title != "Test Book" {
			t.Errorf("Title = %q, want %q", md.Title, "Test Book")
		}
		if len(md.Authors) != 2 || md.Authors[0] != "John Ronald Doe" || md.Authors[1] != "jdoe" {
			t.Errorf("Authors = %q, want [John Ronald Doe jdoe]", md.Authors)
		}
		if md.Description != wantAnnotation {
			t.Errorf("Description = %q, want %q", md.Description, wantAnnotation)
		}
		if len(md.Languages) != 1 || md.Languages[0] != "hu" {
			t.Errorf("Languages = %q, want [hu]", md.Languages)
		}
		if ebook.Resources() == nil {
			t.Error("Resources() = nil, want non-nil")
		}
	}

	t.Run("plain blob", func(t *testing.T) {
		ebook, err := reader.Read(book.NewBytesBlob([]byte(metadataFB2)))
		if err != nil {
			t.Fatalf("Read() error: %v", err)
		}
		checkBook(t, ebook)
	})

	t.Run("zipped blob", func(t *testing.T) {
		blob := buildZipBlob(t, []zipEntry{{"readme.txt", "hi"}, {"book.fb2", metadataFB2}})
		ebook, err := reader.Read(blob)
		if err != nil {
			t.Fatalf("Read() error: %v", err)
		}
		checkBook(t, ebook)
	})

	t.Run("non-fb2 blob errors", func(t *testing.T) {
		if _, err := reader.Read(book.NewBytesBlob([]byte("<html><body>hi</body></html>"))); err == nil {
			t.Error("Read() expected error for non-FB2 blob, got nil")
		}
	})

	t.Run("empty archive errors", func(t *testing.T) {
		if _, err := reader.Read(buildZipBlob(t, nil)); err == nil {
			t.Error("Read() expected error for empty archive, got nil")
		}
	})
}

type zipEntry struct {
	name    string
	content string
}

// buildZipBlob packs entries (in order) into an in-memory zip blob.
func buildZipBlob(t *testing.T, entries []zipEntry) book.Blob {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		fw, err := w.Create(e.name)
		if err != nil {
			t.Fatalf("Create(%q) error: %v", e.name, err)
		}
		if _, err := fw.Write([]byte(e.content)); err != nil {
			t.Fatalf("Write(%q) error: %v", e.name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
	return book.NewBytesBlob(buf.Bytes())
}

// encodeUTF16LE encodes s as UTF-16LE with BOM (ASCII subset only;
// sufficient for tag sniffing, which is encoding-agnostic ASCII).
func encodeUTF16LE(s string) []byte {
	out := []byte{0xFF, 0xFE}
	for i := 0; i < len(s); i++ {
		out = append(out, s[i], 0x00)
	}
	return out
}
