package fb3

import (
	"strings"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
)

const metadataFB3 = `<?xml version="1.0" encoding="UTF-8"?>` +
	`<fb3-description xmlns="http://www.fictionbook.org/FictionBook3/description" version="1.0">` +
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
	`<document-info created="2018-04-16T00:00:00"/>` +
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
			parseFB3Err(t, []byte(tt.data))
		})
	}
}
