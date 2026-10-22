package fb3

import (
	"reflect"
	"strings"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
)

const metadataFB3 = `<?xml version="1.0" encoding="UTF-8"?>` +
	`<fb3-description xmlns="http://www.fictionbook.org/FictionBook3/description" id="6079f35b-49cc-4211-adba-919ccaaff4f2" version="1.0">` +
	`<periodical id="18ea4ccf-daa9-102b-9003-10751c2f945c"><title><main>Source Mag</main></title><issn>1234-5678</issn><issue number="5" year="1998">Number 5</issue></periodical>` +
	`<title><main>Test Book</main><sub>ignored</sub><alt>ignored alt</alt></title>` +
	`<sequence><title><main>Series</main></title></sequence>` +
	`<fb3-relations>` +
	`<subject link="author"><title><main>John Ronald Doe</main></title><first-name>John</first-name><middle-name>Ronald</middle-name><last-name>Doe</last-name></subject>` +
	`<subject link="author"><title><main>Acme Press</main></title></subject>` +
	`<subject link="author"></subject>` +
	`<subject link="editor"><first-name>Ed</first-name><last-name>Itor</last-name></subject>` +
	`<object link="alt_media"><title><main>Other</main></title></object>` +
	`</fb3-relations>` +
	`<fb3-classification><subject>test</subject></fb3-classification>` +
	`<lang>hu</lang>` +
	`<written><lang>ru</lang></written>` +
	`<document-info created="2018-04-16T00:00:00" updated="2018-04-16T00:00:00" doi="10.1000/xyz" src-url="http://example.com/source"><isbn>978-0-306-40615-7</isbn></document-info>` +
	`<paper-publish-info title="Print Edition"><isbn>978-3-16-148410-0</isbn></paper-publish-info>` +
	`<annotation><p>First <strong>para</strong> and <a href="http://example.com">link</a>.</p><br/><p>Second para.</p></annotation>` +
	`</fb3-description>`

const wantFB3Annotation = "First para and link.\n\nSecond para."

func parseFB3(t *testing.T, data []byte) book.Metadata {
	t.Helper()
	pm, err := parseDescription(data)
	if err != nil {
		t.Fatalf("parseDescription() error: %v", err)
	}
	return pm.metadata
}

func parseFB3Err(t *testing.T, data []byte) error {
	t.Helper()
	_, err := parseDescription(data)
	if err == nil {
		t.Fatal("parseDescription() expected error, got nil")
	}
	return err
}

func TestParseDescriptionFull(t *testing.T) {
	md := parseFB3(t, []byte(metadataFB3))

	if md.Title != "Test Book" {
		t.Errorf("Title = %q, want %q", md.Title, "Test Book")
	}
	wantAuthors := []string{"John Ronald Doe", "Acme Press"}
	if strings.Join(md.Authors, "|") != strings.Join(wantAuthors, "|") {
		t.Errorf("Authors = %q, want %q", md.Authors, wantAuthors)
	}
	if md.Description != wantFB3Annotation {
		t.Errorf("Description = %q, want %q", md.Description, wantFB3Annotation)
	}
	if len(md.Languages) != 1 || md.Languages[0] != "hu" {
		t.Errorf("Languages = %q, want [hu]", md.Languages)
	}
	wantIdentifiers := []book.Identifier{
		{Scheme: "UUID", Value: "6079f35b-49cc-4211-adba-919ccaaff4f2"},
		{Scheme: "ISBN", Value: "978-0-306-40615-7"},
		{Scheme: "DOI", Value: "10.1000/xyz"},
		{Scheme: "ISBN", Value: "978-3-16-148410-0"},
		{Scheme: "ISSN", Value: "1234-5678"},
		{Scheme: "URL", Value: "http://example.com/source"},
	}
	if !reflect.DeepEqual(md.Identifiers, wantIdentifiers) {
		t.Errorf("Identifiers = %+v, want %+v", md.Identifiers, wantIdentifiers)
	}
}

func TestParseDescriptionIdentifiers(t *testing.T) {
	wrap := func(attrs, body string) []byte {
		return []byte(`<fb3-description ` + attrs + `>` + body + `</fb3-description>`)
	}

	tests := []struct {
		name string
		data []byte
		want []book.Identifier
	}{
		{
			name: "own identifiers before sources",
			data: wrap(`id="book-id"`, `<document-info created="2024-01-01T00:00:00" updated="2024-01-01T00:00:00"><isbn>978-0-306-40615-7</isbn></document-info>`+
				`<paper-publish-info title="Print"><isbn>978-3-16-148410-0</isbn></paper-publish-info>`+
				`<document-info created="2024-01-01T00:00:00" updated="2024-01-01T00:00:00" src-url="https://example.com/book"/>`),
			want: []book.Identifier{
				{Scheme: "UUID", Value: "book-id"},
				{Scheme: "ISBN", Value: "978-0-306-40615-7"},
				{Scheme: "ISBN", Value: "978-3-16-148410-0"},
				{Scheme: "URL", Value: "https://example.com/book"},
			},
		},
		{
			name: "multiple isbns and paper publications preserved",
			data: wrap(`id="book-id"`, `<document-info created="2024-01-01T00:00:00" updated="2024-01-01T00:00:00"><isbn>978-0-306-40615-7</isbn><isbn>0306406152</isbn></document-info>`+
				`<paper-publish-info title="Print 1"><isbn>978-3-16-148410-0</isbn></paper-publish-info>`+
				`<paper-publish-info title="Print 2"><isbn>978-0-306-40615-7</isbn></paper-publish-info>`),
			want: []book.Identifier{
				{Scheme: "UUID", Value: "book-id"},
				{Scheme: "ISBN", Value: "978-0-306-40615-7"},
				{Scheme: "ISBN", Value: "0306406152"},
				{Scheme: "ISBN", Value: "978-3-16-148410-0"},
				{Scheme: "ISBN", Value: "978-0-306-40615-7"},
			},
		},
		{
			name: "scheme prefixes are stripped",
			data: wrap(`id="uuid:06182534-7d14-4ef9-a738-843ff09973d8"`, `<periodical><issn>issn:1234-5678</issn></periodical>`+
				`<document-info created="2024-01-01T00:00:00" updated="2024-01-01T00:00:00" doi="doi:10.1000/xyz" src-url="https://example.com/book"><isbn>urn:isbn:978-0-306-40615-7</isbn></document-info>`+
				`<paper-publish-info title="Print"><isbn>isbn:978-3-16-148410-0</isbn></paper-publish-info>`),
			want: []book.Identifier{
				{Scheme: "UUID", Value: "06182534-7d14-4ef9-a738-843ff09973d8"},
				{Scheme: "ISBN", Value: "978-0-306-40615-7"},
				{Scheme: "DOI", Value: "10.1000/xyz"},
				{Scheme: "ISBN", Value: "978-3-16-148410-0"},
				{Scheme: "ISSN", Value: "1234-5678"},
				{Scheme: "URL", Value: "https://example.com/book"},
			},
		},
		{
			name: "non matching prefix stays in value",
			data: wrap(`id="book-id"`, `<document-info created="2024-01-01T00:00:00" updated="2024-01-01T00:00:00"><isbn>calibre:978-0-306-40615-7</isbn></document-info>`),
			want: []book.Identifier{
				{Scheme: "UUID", Value: "book-id"},
				{Scheme: "ISBN", Value: "calibre:978-0-306-40615-7"},
			},
		},
		{
			name: "empty values skipped",
			data: wrap(`id=" "`, `<document-info created="2024-01-01T00:00:00" updated="2024-01-01T00:00:00" doi=" " src-url=" "><isbn> </isbn></document-info>`+
				`<paper-publish-info title="Print"><isbn> </isbn></paper-publish-info>`+
				`<periodical><issn> </issn></periodical>`),
			want: nil,
		},
		{
			name: "no identifier fields",
			data: wrap(``, `<title><main>T</main></title><lang>en</lang>`),
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := parseFB3(t, tt.data)
			if !reflect.DeepEqual(md.Identifiers, tt.want) {
				t.Errorf("Identifiers = %+v, want %+v", md.Identifiers, tt.want)
			}
		})
	}
}

func TestParseDescriptionLang(t *testing.T) {
	wrap := func(lang string) []byte {
		return []byte(`<fb3-description><title><main>T</main></title>` +
			`<fb3-relations><subject link="author"><last-name>A</last-name></subject></fb3-relations>` +
			`<fb3-classification><subject>s</subject></fb3-classification>` +
			`<lang>` + lang + `</lang></fb3-description>`)
	}

	for _, lang := range []string{"hu", "en", "ru"} {
		md := parseFB3(t, wrap(lang))
		if strings.Join(md.Languages, "|") != lang {
			t.Errorf("Languages for %q = %q, want [%s]", lang, md.Languages, lang)
		}
	}
	for _, lang := range []string{"und", "not a language!!!", ""} {
		md := parseFB3(t, wrap(lang))
		if len(md.Languages) != 0 {
			t.Errorf("Languages for %q = %q, want empty", lang, md.Languages)
		}
	}
}

func TestParseDescriptionEdge(t *testing.T) {
	tests := []struct {
		name  string
		data  string
		check func(t *testing.T, md book.Metadata)
	}{
		{"no relations means no authors", `<fb3-description><title><main>T</main></title><lang>en</lang></fb3-description>`,
			func(t *testing.T, md book.Metadata) {
				if len(md.Authors) != 0 {
					t.Errorf("Authors = %q, want empty", md.Authors)
				}
			}},
		{"missing title stays empty", `<fb3-description><fb3-relations><subject link="author"><last-name>A</last-name></subject></fb3-relations><lang>en</lang></fb3-description>`,
			func(t *testing.T, md book.Metadata) {
				if md.Title != "" {
					t.Errorf("Title = %q, want empty", md.Title)
				}
			}},
		{"missing annotation stays empty", `<fb3-description><title><main>T</main></title><lang>en</lang></fb3-description>`,
			func(t *testing.T, md book.Metadata) {
				if md.Description != "" {
					t.Errorf("Description = %q, want empty", md.Description)
				}
			}},
		{"bare ampersand fixed", `<fb3-description><title><main>A & B</main></title><lang>en</lang></fb3-description>`,
			func(t *testing.T, md book.Metadata) {
				if md.Title != "A & B" {
					t.Errorf("Title = %q, want %q", md.Title, "A & B")
				}
			}},
		{"utf8 BOM stripped", "\xEF\xBB\xBF" + `<fb3-description><title><main>BOM</main></title><lang>en</lang></fb3-description>`,
			func(t *testing.T, md book.Metadata) {
				if md.Title != "BOM" {
					t.Errorf("Title = %q, want BOM", md.Title)
				}
			}},
		{"prefixed namespace", `<fb3d:fb3-description xmlns:fb3d="http://www.fictionbook.org/FictionBook3/description"><fb3d:title><fb3d:main>P</fb3d:main></fb3d:title><fb3d:lang>en</fb3d:lang></fb3d:fb3-description>`,
			func(t *testing.T, md book.Metadata) {
				if md.Title != "P" {
					t.Errorf("Title = %q, want P", md.Title)
				}
			}},
		{"comments ignored", `<fb3-description><!-- lead --><title><main>C</main></title><!-- trail --><lang>en</lang></fb3-description>`,
			func(t *testing.T, md book.Metadata) {
				if md.Title != "C" {
					t.Errorf("Title = %q, want C", md.Title)
				}
			}},
		{"co_author counts as author", `<fb3-description><title><main>T</main></title>` +
			`<fb3-relations>` +
			`<subject link="author"><last-name>A</last-name></subject>` +
			`<subject link="co_author"><first-name>Co</first-name><last-name>B</last-name></subject>` +
			`<subject link="editor"><last-name>E</last-name></subject>` +
			`</fb3-relations><lang>en</lang></fb3-description>`,
			func(t *testing.T, md book.Metadata) {
				if strings.Join(md.Authors, "|") != "A|Co B" {
					t.Errorf("Authors = %q, want [A Co B]", md.Authors)
				}
			}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, parseFB3(t, []byte(tt.data)))
		})
	}
}

func TestParseDescriptionErrors(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{"empty", ``},
		{"non-fb3 root", `<library><book>x</book></library>`},
		{"truncated", `<fb3-description><title><main>T`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = parseFB3Err(t, []byte(tt.data))
		})
	}
}

func TestParseDescriptionPublisherAndDate(t *testing.T) {
	wrap := func(inner string) []byte {
		return []byte(`<fb3-description xmlns="http://www.fictionbook.org/FictionBook3/description" id="test-id">` +
			`<title><main>Test Book</main></title>` +
			`<fb3-relations><subject link="author"><last-name>Author</last-name></subject></fb3-relations>` +
			`<lang>en</lang>` +
			inner +
			`</fb3-description>`)
	}

	tests := []struct {
		name  string
		inner string
		pub   string
		date  string
	}{
		{
			name:  "paper-publish-info publisher attribute",
			inner: `<paper-publish-info title="Print Edition" publisher="Acme Press"><isbn>978-3-16-148410-0</isbn></paper-publish-info>`,
			pub:   "Acme Press",
			date:  "",
		},
		{
			name:  "paper-publish-info year attribute",
			inner: `<paper-publish-info title="Print Edition" year="2020"><isbn>978-3-16-148410-0</isbn></paper-publish-info>`,
			pub:   "",
			date:  "2020",
		},
		{
			name:  "paper-publish-info both publisher and year",
			inner: `<paper-publish-info title="Print Edition" publisher="Acme Press" year="2020"><isbn>978-3-16-148410-0</isbn></paper-publish-info>`,
			pub:   "Acme Press",
			date:  "2020",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := parseFB3(t, wrap(tt.inner))
			if md.Publisher != tt.pub {
				t.Errorf("Publisher = %q, want %q", md.Publisher, tt.pub)
			}
			if md.PublicationDate != tt.date {
				t.Errorf("PublicationDate = %q, want %q", md.PublicationDate, tt.date)
			}
		})
	}
}
