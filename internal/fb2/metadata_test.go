package fb2

import (
	"bytes"
	"strings"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"golang.org/x/text/encoding/charmap"
)

const metadataFB2 = `<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0" xmlns:l="http://www.w3.org/1999/xlink">
<description>
<title-info>
<genre>sf</genre>
<author><first-name>John</first-name><middle-name>Ronald</middle-name><last-name>Doe</last-name><nickname>ignored</nickname></author>
<author><nickname>jdoe</nickname></author>
<author></author>
<book-title>Test Book</book-title>
<annotation>
<p>First <emphasis>para</emphasis> and <a l:href="#n1">link</a>.</p>
<poem><stanza><v>line one</v><v>line two</v></stanza><text-author>Poet</text-author></poem>
<empty-line/>
<p>Second para.</p>
</annotation>
<lang>hu</lang>
</title-info>
</description>
<body><section><p>Hello</p></section></body>
</FictionBook>`

const wantAnnotation = "First para and link.\nline one\nline two\nPoet\n\nSecond para."

func parseBytes(t *testing.T, data []byte) (book.Metadata, string) {
	t.Helper()
	pm, err := parseMetadata(data)
	if err != nil {
		t.Fatalf("parseMetadata() error: %v", err)
	}
	return pm.metadata, pm.version
}

func parseBytesErr(t *testing.T, data []byte) error {
	t.Helper()
	_, err := parseMetadata(data)
	if err == nil {
		t.Fatal("parseMetadata() expected error, got nil")
	}
	return err
}

func TestParseMetadataFull(t *testing.T) {
	md, version := parseBytes(t, []byte(metadataFB2))

	if md.Title != "Test Book" {
		t.Errorf("Title = %q, want %q", md.Title, "Test Book")
	}
	wantAuthors := []string{"John Ronald Doe", "jdoe"}
	if strings.Join(md.Authors, "|") != strings.Join(wantAuthors, "|") {
		t.Errorf("Authors = %q, want %q", md.Authors, wantAuthors)
	}
	if md.Description != wantAnnotation {
		t.Errorf("Description = %q, want %q", md.Description, wantAnnotation)
	}
	if strings.Join(md.Languages, "|") != "hu" {
		t.Errorf("Languages = %q, want [hu]", md.Languages)
	}
	if version != "2.0" {
		t.Errorf("version = %q, want %q", version, "2.0")
	}
}

func TestParseMetadataFallbacks(t *testing.T) {
	wrap := func(desc string) []byte {
		return []byte(`<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0"><description>` + desc + `</description><body><p>x</p></body></FictionBook>`)
	}

	t.Run("title from src-title-info", func(t *testing.T) {
		desc := `<title-info><author><nickname>a</nickname></author><book-title>  </book-title><lang>en</lang></title-info>` +
			`<src-title-info><author><nickname>b</nickname></author><book-title>Src Title</book-title></src-title-info>`
		md, _ := parseBytes(t, wrap(desc))
		if md.Title != "Src Title" {
			t.Errorf("Title = %q, want %q", md.Title, "Src Title")
		}
	})

	t.Run("authors prefer title-info", func(t *testing.T) {
		desc := `<title-info><author><nickname>first</nickname></author><book-title>T</book-title><lang>en</lang></title-info>` +
			`<src-title-info><author><nickname>second</nickname></author></src-title-info>`
		md, _ := parseBytes(t, wrap(desc))
		if strings.Join(md.Authors, "|") != "first" {
			t.Errorf("Authors = %q, want [first]", md.Authors)
		}
	})

	t.Run("authors from src-title-info when title-info has none", func(t *testing.T) {
		desc := `<title-info><book-title>T</book-title><lang>en</lang></title-info>` +
			`<src-title-info><author><first-name>Src</first-name><last-name>Author</last-name></author></src-title-info>`
		md, _ := parseBytes(t, wrap(desc))
		if strings.Join(md.Authors, "|") != "Src Author" {
			t.Errorf("Authors = %q, want [Src Author]", md.Authors)
		}
	})

	t.Run("authors from document-info as last resort", func(t *testing.T) {
		desc := `<title-info><book-title>T</book-title><lang>en</lang></title-info>` +
			`<document-info><author><nickname>docmaker</nickname></author><date>2024-01-01</date><id>x</id><version>1</version></document-info>`
		md, _ := parseBytes(t, wrap(desc))
		if strings.Join(md.Authors, "|") != "docmaker" {
			t.Errorf("Authors = %q, want [docmaker]", md.Authors)
		}
	})

	t.Run("direct text author as fallback", func(t *testing.T) {
		desc := `<title-info><author>Lev Tolstoy</author><book-title>T</book-title><lang>en</lang></title-info>`
		md, _ := parseBytes(t, wrap(desc))
		if strings.Join(md.Authors, "|") != "Lev Tolstoy" {
			t.Errorf("Authors = %q, want [Lev Tolstoy]", md.Authors)
		}
	})

	t.Run("subelements win over direct text", func(t *testing.T) {
		desc := `<title-info><author>Ignored <nickname>nick</nickname></author><book-title>T</book-title><lang>en</lang></title-info>`
		md, _ := parseBytes(t, wrap(desc))
		if strings.Join(md.Authors, "|") != "nick" {
			t.Errorf("Authors = %q, want [nick]", md.Authors)
		}
	})

	t.Run("no authors stays empty", func(t *testing.T) {
		desc := `<title-info><book-title>T</book-title><lang>en</lang></title-info>`
		md, _ := parseBytes(t, wrap(desc))
		if len(md.Authors) != 0 {
			t.Errorf("Authors = %q, want empty", md.Authors)
		}
	})

	t.Run("annotation from src-title-info", func(t *testing.T) {
		desc := `<title-info><author><nickname>a</nickname></author><book-title>T</book-title><lang>en</lang></title-info>` +
			`<src-title-info><annotation><p>Src note.</p></annotation></src-title-info>`
		md, _ := parseBytes(t, wrap(desc))
		if md.Description != "Src note." {
			t.Errorf("Description = %q, want %q", md.Description, "Src note.")
		}
	})

	t.Run("missing title stays empty", func(t *testing.T) {
		desc := `<title-info><author><nickname>a</nickname></author><lang>en</lang></title-info>`
		md, _ := parseBytes(t, wrap(desc))
		if md.Title != "" {
			t.Errorf("Title = %q, want empty", md.Title)
		}
	})
}

func TestParseMetadataLanguage(t *testing.T) {
	wrap := func(lang string) []byte {
		return []byte(`<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0"><description><title-info><author><nickname>a</nickname></author><book-title>T</book-title><lang>` + lang + `</lang></title-info></description><body><p>x</p></body></FictionBook>`)
	}

	for _, lang := range []string{"hu", "en", "ru", "de"} {
		md, _ := parseBytes(t, wrap(lang))
		if strings.Join(md.Languages, "|") != lang {
			t.Errorf("Languages for %q = %q, want [%s]", lang, md.Languages, lang)
		}
	}
	for _, lang := range []string{"und", "not a language!!!", ""} {
		md, _ := parseBytes(t, wrap(lang))
		if len(md.Languages) != 0 {
			t.Errorf("Languages for %q = %q, want empty", lang, md.Languages)
		}
	}
}

func TestParseMetadataNamespaces(t *testing.T) {
	t.Run("prefixed", func(t *testing.T) {
		data := `<fb:FictionBook xmlns:fb="http://www.gribuser.ru/xml/fictionbook/2.0"><fb:description><fb:title-info><fb:author><fb:nickname>a</fb:nickname></fb:author><fb:book-title>Prefixed</fb:book-title><fb:lang>en</fb:lang></fb:title-info></fb:description><fb:body><fb:p>x</fb:p></fb:body></fb:FictionBook>`
		md, version := parseBytes(t, []byte(data))
		if md.Title != "Prefixed" {
			t.Errorf("Title = %q, want Prefixed", md.Title)
		}
		if version != "2.0" {
			t.Errorf("version = %q, want 2.0", version)
		}
	})

	t.Run("no namespace", func(t *testing.T) {
		data := `<FictionBook><description><title-info><author><nickname>a</nickname></author><book-title>Bare</book-title><lang>en</lang></title-info></description><body><p>x</p></body></FictionBook>`
		md, version := parseBytes(t, []byte(data))
		if md.Title != "Bare" {
			t.Errorf("Title = %q, want Bare", md.Title)
		}
		if version != "" {
			t.Errorf("version = %q, want empty", version)
		}
	})

	t.Run("2.1 namespace", func(t *testing.T) {
		data := `<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.1"><description><title-info><author><nickname>a</nickname></author><book-title>V21</book-title><lang>en</lang></title-info></description><body><p>x</p></body></FictionBook>`
		md, version := parseBytes(t, []byte(data))
		if md.Title != "V21" {
			t.Errorf("Title = %q, want V21", md.Title)
		}
		if version != "2.1" {
			t.Errorf("version = %q, want 2.1", version)
		}
	})
}

func TestParseMetadataEncodings(t *testing.T) {
	t.Run("utf8 BOM", func(t *testing.T) {
		data := append([]byte{0xEF, 0xBB, 0xBF}, []byte(metadataFB2)...)
		md, _ := parseBytes(t, data)
		if md.Title != "Test Book" {
			t.Errorf("Title = %q, want Test Book", md.Title)
		}
	})

	t.Run("windows-1251", func(t *testing.T) {
		raw := `<?xml version="1.0" encoding="windows-1251"?><FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0"><description><title-info><author><first-name>Лев</first-name><last-name>Толстой</last-name></author><book-title>Война</book-title><lang>ru</lang></title-info></description><body><p>x</p></body></FictionBook>`
		encoded, err := charmap.Windows1251.NewEncoder().Bytes([]byte(raw))
		if err != nil {
			t.Fatalf("encode error: %v", err)
		}
		md, _ := parseBytes(t, encoded)
		if md.Title != "Война" {
			t.Errorf("Title = %q, want Война", md.Title)
		}
		if strings.Join(md.Authors, "|") != "Лев Толстой" {
			t.Errorf("Authors = %q, want [Лев Толстой]", md.Authors)
		}
	})

	t.Run("utf16le", func(t *testing.T) {
		raw := `<?xml version="1.0" encoding="UTF-16"?><FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0"><description><title-info><author><nickname>a</nickname></author><book-title>Wide</book-title><lang>en</lang></title-info></description><body><p>x</p></body></FictionBook>`
		md, _ := parseBytes(t, encodeUTF16LE(raw))
		if md.Title != "Wide" {
			t.Errorf("Title = %q, want Wide", md.Title)
		}
	})

	t.Run("stray NUL bytes", func(t *testing.T) {
		data := bytes.ReplaceAll([]byte(metadataFB2), []byte("Book"), []byte("Bo\x00ok"))
		md, _ := parseBytes(t, data)
		if md.Title != "Test Book" {
			t.Errorf("Title = %q, want Test Book", md.Title)
		}
	})
}

func TestParseMetadataRobustness(t *testing.T) {
	t.Run("bare ampersand fixed on retry", func(t *testing.T) {
		data := `<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0"><description><title-info><author><nickname>a</nickname></author><book-title>R &amp; D & Friends</book-title><lang>en</lang></title-info></description><body><p>x</p></body></FictionBook>`
		md, _ := parseBytes(t, []byte(data))
		if md.Title != "R & D & Friends" {
			t.Errorf("Title = %q, want %q", md.Title, "R & D & Friends")
		}
	})

	t.Run("valid entities pass strict", func(t *testing.T) {
		data := `<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0"><description><title-info><author><nickname>a</nickname></author><book-title>Fish &amp; Chips</book-title><lang>en</lang></title-info></description><body><p>x</p></body></FictionBook>`
		md, _ := parseBytes(t, []byte(data))
		if md.Title != "Fish & Chips" {
			t.Errorf("Title = %q, want %q", md.Title, "Fish & Chips")
		}
	})

	t.Run("mismatched tags error", func(t *testing.T) {
		data := `<FictionBook><description><title-info><book-title>X</book-title></title-infx></description></FictionBook>`
		_ = parseBytesErr(t, []byte(data))
	})

	t.Run("truncated document errors", func(t *testing.T) {
		data := `<FictionBook><description><title-info><book-title>Unclosed`
		_ = parseBytesErr(t, []byte(data))
	})

	t.Run("wrong root errors", func(t *testing.T) {
		_ = parseBytesErr(t, []byte(`<library><book>x</book></library>`))
	})

	t.Run("empty input errors", func(t *testing.T) {
		_ = parseBytesErr(t, nil)
	})

	t.Run("trailing garbage after description ignored", func(t *testing.T) {
		data := `<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0"><description><title-info><author><nickname>a</nickname></author><book-title>Early</book-title><lang>en</lang></title-info></description><body><section><p>unclosed`
		md, _ := parseBytes(t, []byte(data))
		if md.Title != "Early" {
			t.Errorf("Title = %q, want Early", md.Title)
		}
	})
}
