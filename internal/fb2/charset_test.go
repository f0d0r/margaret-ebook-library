package fb2

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
	"golang.org/x/text/encoding/charmap"
)

// utf16Bytes encodes s as UTF-16LE, optionally prefixed with a BOM.
func utf16Bytes(s string, bom bool) []byte {
	var out []byte
	if bom {
		out = append(out, 0xFF, 0xFE)
	}
	for _, u := range utf16.Encode([]rune(s)) {
		out = append(out, byte(u), byte(u>>8))
	}
	return out
}

// utf16BEBytes encodes s as UTF-16BE without a BOM.
func utf16BEBytes(s string) []byte {
	var out []byte
	for _, u := range utf16.Encode([]rune(s)) {
		out = append(out, byte(u>>8), byte(u))
	}
	return out
}

func TestFb2ReadPreservesCP1251(t *testing.T) {
	src := `<?xml version="1.0" encoding="windows-1251"?>` +
		`<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">` +
		`<description><title-info><book-title>Война и мир</book-title><lang>ru</lang></title-info></description>` +
		`<body><section><p>Привет, мир!</p></section></body></FictionBook>`
	raw, err := charmap.Windows1251.NewEncoder().Bytes([]byte(src))
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}

	ebook, err := NewFb2Reader(config.DefaultConfig()).Read(book.NewBytesBlob(raw))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if got := ebook.Metadata().Title; got != "Война и мир" {
		t.Errorf("Title = %q, want Война и мир", got)
	}

	ro := ebook.Resources().ReadingOrder()
	if len(ro) == 0 {
		t.Fatal("no reading-order resources")
	}
	res := ro[0].Resource
	if res.Charset != "windows-1251" {
		t.Errorf("Charset = %q, want windows-1251", res.Charset)
	}

	// Byte identity: the stored body slice is the exact cp1251 span
	// between the <body> tags.
	start := bytes.Index(raw, []byte("<body>")) + len("<body>")
	end := bytes.Index(raw, []byte("</body>"))
	want := raw[start:end]
	got, err := res.Data()
	if err != nil {
		t.Fatalf("Data() error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("stored bytes not byte-identical:\n got %x\nwant %x", got, want)
	}
	if bytes.Contains(got, []byte("Привет")) {
		t.Error("stored bytes were silently transcoded to UTF-8")
	}

	text, err := res.DataAs(context.Background(), mediatype.PlainText)
	if err != nil {
		t.Fatalf("DataAs(plain text) error: %v", err)
	}
	if !strings.Contains(string(text), "Привет, мир!") {
		t.Errorf("decoded text %q missing Привет, мир!", string(text))
	}
}

func TestFb2ReadPreservesUTF16LE(t *testing.T) {
	src := `<?xml version="1.0" encoding="UTF-16"?>` +
		`<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">` +
		`<description><title-info><book-title>Wide Book</book-title><lang>ru</lang></title-info></description>` +
		`<body><section><p>Привет, мир!</p></section></body></FictionBook>`
	raw := utf16Bytes(src, true)

	ebook, err := NewFb2Reader(config.DefaultConfig()).Read(book.NewBytesBlob(raw))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if got := ebook.Metadata().Title; got != "Wide Book" {
		t.Errorf("Title = %q, want Wide Book", got)
	}

	res := ebook.Resources().ReadingOrder()[0].Resource
	if res.Charset != "utf-16le" {
		t.Errorf("Charset = %q, want utf-16le", res.Charset)
	}

	openTag := utf16Bytes("<body>", false)
	closeTag := utf16Bytes("</body>", false)
	start := bytes.Index(raw, openTag) + len(openTag)
	end := bytes.Index(raw, closeTag)
	want := raw[start:end]
	got, err := res.Data()
	if err != nil {
		t.Fatalf("Data() error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("stored bytes not byte-identical:\n got %x\nwant %x", got, want)
	}
	if !bytes.Equal(got, utf16Bytes(`<section><p>Привет, мир!</p></section>`, false)) {
		t.Errorf("stored body = %x, want the raw UTF-16LE span", got)
	}

	text, err := res.DataAs(context.Background(), mediatype.PlainText)
	if err != nil {
		t.Fatalf("DataAs(plain text) error: %v", err)
	}
	if !strings.Contains(string(text), "Привет, мир!") {
		t.Errorf("decoded text %q missing Привет, мир!", string(text))
	}
}

func TestFb2ReadBOMLessUTF16(t *testing.T) {
	tests := []struct {
		name        string
		decl        string
		wantCharset string
		encode      func(string) []byte
	}{
		{"le declared utf-16le", "utf-16le", "utf-16le", func(s string) []byte { return utf16Bytes(s, false) }},
		{"le declared utf-16", "UTF-16", "utf-16le", func(s string) []byte { return utf16Bytes(s, false) }},
		{"be declared utf-16be", "utf-16be", "utf-16be", utf16BEBytes},
		{"be declared utf-16", "UTF-16", "utf-16be", utf16BEBytes},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := `<?xml version="1.0" encoding="` + tt.decl + `"?>` +
				`<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">` +
				`<description><title-info><book-title>Wide Book</book-title><lang>ru</lang></title-info></description>` +
				`<body><section><p>Привет, мир!</p></section></body></FictionBook>`
			raw := tt.encode(src)

			ebook, err := NewFb2Reader(config.DefaultConfig()).Read(book.NewBytesBlob(raw))
			if err != nil {
				t.Fatalf("Read() error: %v", err)
			}
			if got := ebook.Metadata().Title; got != "Wide Book" {
				t.Errorf("Title = %q, want Wide Book", got)
			}

			res := ebook.Resources().ReadingOrder()[0].Resource
			if res.Charset != tt.wantCharset {
				t.Errorf("Charset = %q, want %q", res.Charset, tt.wantCharset)
			}

			// Byte identity proves the BOM-less range mapping is exact.
			openTag := tt.encode("<body>")
			closeTag := tt.encode("</body>")
			start := bytes.Index(raw, openTag) + len(openTag)
			end := bytes.Index(raw, closeTag)
			want := raw[start:end]
			got, err := res.Data()
			if err != nil {
				t.Fatalf("Data() error: %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("stored bytes not byte-identical:\n got %x\nwant %x", got, want)
			}

			text, err := res.DataAs(context.Background(), mediatype.PlainText)
			if err != nil {
				t.Fatalf("DataAs(plain text) error: %v", err)
			}
			if !strings.Contains(string(text), "Привет, мир!") {
				t.Errorf("decoded text %q missing Привет, мир!", string(text))
			}
		})
	}
}

func TestFb2ReadUTF8DeclReportsUTF8(t *testing.T) {
	ebook, err := NewFb2Reader(config.DefaultConfig()).Read(book.NewBytesBlob([]byte(richFb2())))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	for _, r := range ebook.Resources().All() {
		if r.MediaType == mediatype.JPEG || r.MediaType == mediatype.PNG {
			if r.Charset != book.CharsetBinary {
				t.Errorf("binary resource %s Charset = %q, want binary", r.Href, r.Charset)
			}
			continue
		}
		if r.MediaType == mediatype.FB2Body && r.Charset != book.CharsetUTF8 {
			t.Errorf("body resource %s Charset = %q, want utf-8", r.Href, r.Charset)
		}
	}
}
