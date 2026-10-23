package fb2

import (
	"bytes"
	"reflect"
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
<keywords>fantasy, adventure</keywords>
<lang>hu</lang>
</title-info>
<document-info>
<author><nickname>docmaker</nickname></author>
<id>06182534-7d14-4ef9-a738-843ff09973d8</id>
<src-url>http://example.com/source</src-url>
<src-ocr>OCR Person</src-ocr>
</document-info>
<publish-info><isbn>978-0-306-40615-7</isbn></publish-info>
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
	wantSubjects := []string{"sf", "fantasy", "adventure"}
	if !reflect.DeepEqual(md.Subjects, wantSubjects) {
		t.Errorf("Subjects = %#v, want %#v", md.Subjects, wantSubjects)
	}
	if strings.Join(md.Languages, "|") != "hu" {
		t.Errorf("Languages = %q, want [hu]", md.Languages)
	}
	if version != "2.0" {
		t.Errorf("version = %q, want %q", version, "2.0")
	}
	wantIdentifiers := []book.Identifier{
		{Value: "06182534-7d14-4ef9-a738-843ff09973d8"},
		{Scheme: "ISBN", Value: "978-0-306-40615-7"},
		{Scheme: "URL", Value: "http://example.com/source"},
	}
	if !reflect.DeepEqual(md.Identifiers, wantIdentifiers) {
		t.Errorf("Identifiers = %+v, want %+v", md.Identifiers, wantIdentifiers)
	}
	if md.Publisher != "" {
		t.Errorf("Publisher = %q, want %q", md.Publisher, "")
	}
	if md.PublicationDate != "" {
		t.Errorf("PublicationDate = %q, want %q", md.PublicationDate, "")
	}
}

func TestParseMetadataIdentifiers(t *testing.T) {
	wrap := func(desc string) []byte {
		return []byte(`<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0"><description>` + desc + `</description><body><p>x</p></body></FictionBook>`)
	}

	tests := []struct {
		name string
		desc string
		want []book.Identifier
	}{
		{
			name: "own id before sources",
			desc: `<document-info><id>doc-id</id><src-url>https://example.com/book</src-url></document-info>` +
				`<publish-info><isbn>978-0-306-40615-7</isbn></publish-info>`,
			want: []book.Identifier{
				{Value: "doc-id"},
				{Scheme: "ISBN", Value: "978-0-306-40615-7"},
				{Scheme: "URL", Value: "https://example.com/book"},
			},
		},
		{
			name: "multiple isbns preserved",
			desc: `<publish-info><isbn>978-0-306-40615-7</isbn><isbn>978-0-306-40615-7</isbn><isbn>0306406152</isbn></publish-info>`,
			want: []book.Identifier{
				{Scheme: "ISBN", Value: "978-0-306-40615-7"},
				{Scheme: "ISBN", Value: "978-0-306-40615-7"},
				{Scheme: "ISBN", Value: "0306406152"},
			},
		},
		{
			name: "scheme prefixes are stripped",
			desc: `<document-info><id>uuid:06182534-7d14-4ef9-a738-843ff09973d8</id></document-info>` +
				`<publish-info><isbn>urn:isbn:978-0-306-40615-7</isbn><isbn>isbn:0306406152</isbn></publish-info>`,
			want: []book.Identifier{
				{Scheme: "UUID", Value: "06182534-7d14-4ef9-a738-843ff09973d8"},
				{Scheme: "ISBN", Value: "978-0-306-40615-7"},
				{Scheme: "ISBN", Value: "0306406152"},
			},
		},
		{
			name: "non matching prefix stays in value",
			desc: `<publish-info><isbn>calibre:978-0-306-40615-7</isbn></publish-info>`,
			want: []book.Identifier{{Scheme: "ISBN", Value: "calibre:978-0-306-40615-7"}},
		},
		{
			name: "empty values skipped",
			desc: `<document-info><id>  </id><src-url> </src-url></document-info>` +
				`<publish-info><isbn> </isbn></publish-info>`,
			want: nil,
		},
		{
			name: "src ocr is not an identifier",
			desc: `<document-info><src-ocr>OCR Person</src-ocr></document-info>`,
			want: nil,
		},
		{
			name: "no identifier fields",
			desc: `<title-info><book-title>T</book-title></title-info>`,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md, _ := parseBytes(t, wrap(tt.desc))
			if !reflect.DeepEqual(md.Identifiers, tt.want) {
				t.Errorf("Identifiers = %+v, want %+v", md.Identifiers, tt.want)
			}
		})
	}
}

func TestParseMetadataPublisherAndDate(t *testing.T) {
	wrap := func(desc string) []byte {
		return []byte(`<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0"><description>` + desc + `</description><body><p>x</p></body></FictionBook>`)
	}

	tests := []struct {
		name    string
		desc    string
		pub     string
		pubDate string
	}{
		{
			name:    "publisher and year from publish-info",
			desc:    `<publish-info><publisher>Test Publisher</publisher><year>2023</year></publish-info>`,
			pub:     "Test Publisher",
			pubDate: "2023",
		},
		{
			name:    "publisher and date from publish-info",
			desc:    `<publish-info><publisher>Test Publisher</publisher><date>2023-05-15</date></publish-info>`,
			pub:     "Test Publisher",
			pubDate: "2023-05-15",
		},
		{
			name:    "publisher trimmed",
			desc:    `<publish-info><publisher>  Spaced Publisher  </publisher><year>2022</year></publish-info>`,
			pub:     "Spaced Publisher",
			pubDate: "2022",
		},
		{
			name:    "date preferred over year",
			desc:    `<publish-info><date>2021-01-01</date><year>2021</year></publish-info>`,
			pub:     "",
			pubDate: "2021-01-01",
		},
		{
			name:    "empty publish-info",
			desc:    `<publish-info></publish-info>`,
			pub:     "",
			pubDate: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md, _ := parseBytes(t, wrap(tt.desc))
			if md.Publisher != tt.pub {
				t.Errorf("Publisher = %q, want %q", md.Publisher, tt.pub)
			}
			if md.PublicationDate != tt.pubDate {
				t.Errorf("PublicationDate = %q, want %q", md.PublicationDate, tt.pubDate)
			}
		})
	}
}

func TestParseMetadataSeries(t *testing.T) {
	wrap := func(desc string) []byte {
		return []byte(`<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0"><description>` + desc + `</description><body><p>x</p></body></FictionBook>`)
	}

	tests := []struct {
		name      string
		desc      string
		want      string
		wantIndex string
	}{
		{
			name:      "title-info sequence",
			desc:      `<title-info><sequence name="Saga" number="2"/></title-info>`,
			want:      "Saga",
			wantIndex: "2",
		},
		{
			name: "title-info wins over publish-info",
			desc: `<title-info><sequence name="Saga" number="2"/></title-info>` +
				`<publish-info><sequence name="Published" number="9"/></publish-info>`,
			want:      "Saga",
			wantIndex: "2",
		},
		{
			name:      "publish-info fallback",
			desc:      `<title-info></title-info><publish-info><sequence name="Published" number="1"/></publish-info>`,
			want:      "Published",
			wantIndex: "1",
		},
		{
			name: "first non-empty title-info sequence wins",
			desc: `<title-info><sequence name=" "/>` +
				`<sequence name="Second" number="2"/></title-info>`,
			want:      "Second",
			wantIndex: "2",
		},
		{
			name: "first of multiple title-info sequences wins",
			desc: `<title-info><sequence name="First" number="1"/>` +
				`<sequence name="Second" number="2"/></title-info>`,
			want:      "First",
			wantIndex: "1",
		},
		{
			name: "src-title-info is ignored",
			desc: `<title-info></title-info>` +
				`<src-title-info><sequence name="Source" number="3"/></src-title-info>`,
			want:      "",
			wantIndex: "",
		},
		{
			name:      "missing number",
			desc:      `<title-info><sequence name="Saga"/></title-info>`,
			want:      "Saga",
			wantIndex: "",
		},
		{
			name:      "number trimmed verbatim",
			desc:      `<title-info><sequence name="Saga" number=" 02 "/></title-info>`,
			want:      "Saga",
			wantIndex: "02",
		},
		{
			name:      "negative number",
			desc:      `<title-info><sequence name="Saga" number="-1"/></title-info>`,
			want:      "Saga",
			wantIndex: "-1",
		},
		{
			name:      "malformed number kept",
			desc:      `<title-info><sequence name="Saga" number="x"/></title-info>`,
			want:      "Saga",
			wantIndex: "x",
		},
		{
			name: "nested sequence ignored",
			desc: `<title-info><sequence name="Outer" number="1">` +
				`<sequence name="Inner" number="2"/></sequence></title-info>`,
			want:      "Outer",
			wantIndex: "1",
		},
		{
			name: "publish-info first non-empty sequence wins",
			desc: `<publish-info><sequence name=" "/>` +
				`<sequence name="Later" number="4"/></publish-info>`,
			want:      "Later",
			wantIndex: "4",
		},
		{
			name:      "name trimmed",
			desc:      `<title-info><sequence name="  Saga  " number="2"/></title-info>`,
			want:      "Saga",
			wantIndex: "2",
		},
		{
			name:      "number without name ignored",
			desc:      `<title-info><sequence number="2"/></title-info>`,
			want:      "",
			wantIndex: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md, _ := parseBytes(t, wrap(tt.desc))
			if md.Series != tt.want || md.SeriesIndex != tt.wantIndex {
				t.Errorf("Series = (%q, %q), want (%q, %q)",
					md.Series, md.SeriesIndex, tt.want, tt.wantIndex)
			}
		})
	}

	t.Run("prefixed elements", func(t *testing.T) {
		data := `<fb:FictionBook xmlns:fb="http://www.gribuser.ru/xml/fictionbook/2.0"><fb:description>` +
			`<fb:title-info><fb:sequence name="Saga" number="5"/></fb:title-info>` +
			`</fb:description><fb:body><fb:p>x</fb:p></fb:body></fb:FictionBook>`
		md, _ := parseBytes(t, []byte(data))
		if md.Series != "Saga" || md.SeriesIndex != "5" {
			t.Errorf("Series = (%q, %q), want (%q, %q)",
				md.Series, md.SeriesIndex, "Saga", "5")
		}
	})
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

	for _, lang := range []string{"hu", "en", "ru", "de", ""} {
		md, _ := parseBytes(t, wrap(lang))
		if strings.Join(md.Languages, "|") != lang {
			t.Errorf("Languages for %q = %q, want [%s]", lang, md.Languages, lang)
		}
	}
}

func TestFbVersion(t *testing.T) {
	tests := []struct {
		name  string
		space string
		want  string
	}{
		{"2.0", "http://www.gribuser.ru/xml/fictionbook/2.0", "2.0"},
		{"2.1", "http://www.gribuser.ru/xml/fictionbook/2.1", "2.1"},
		{"future 2.2", "http://www.gribuser.ru/xml/fictionbook/2.2", "2.2"},
		{"future 2.10", "http://www.gribuser.ru/xml/fictionbook/2.10", "2.10"},
		{"bare prefix", "http://www.gribuser.ru/xml/fictionbook/", ""},
		{"major only", "http://www.gribuser.ru/xml/fictionbook/2", "2"},
		{"major only other", "http://www.gribuser.ru/xml/fictionbook/20", "20"},
		{"other family declared as-is", "http://www.gribuser.ru/xml/fictionbook/3.0", "3.0"},
		{"silly major declared as-is", "http://www.gribuser.ru/xml/fictionbook/8.9", "8.9"},
		{"unrelated namespace", "http://example.com/book", ""},
		{"empty", "", ""},
		{"different case", "HTTP://www.gribuser.ru/xml/fictionbook/2.0", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fbVersion(tt.space); got != tt.want {
				t.Errorf("fbVersion(%q) = %q, want %q", tt.space, got, tt.want)
			}
		})
	}
}

func TestParseMetadataFutureNamespace(t *testing.T) {
	data := `<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.2"><description><title-info><author><nickname>a</nickname></author><book-title>Future</book-title><lang>en</lang></title-info></description><body><p>x</p></body></FictionBook>`
	md, version := parseBytes(t, []byte(data))
	if md.Title != "Future" {
		t.Errorf("Title = %q, want Future", md.Title)
	}
	if version != "2.2" {
		t.Errorf("version = %q, want 2.2", version)
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
