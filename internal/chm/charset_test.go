package chm

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
	"golang.org/x/text/encoding/charmap"
)

func TestChmEntryCharsets(t *testing.T) {
	cp1251Topic, err := charmap.Windows1251.NewEncoder().Bytes([]byte(
		`<html><head><meta charset="windows-1251"><title>Глава</title></head>` +
			`<body><p>Привет, мир!</p></body></html>`))
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	cp1250Topic, err := charmap.Windows1250.NewEncoder().Bytes([]byte(
		`<html><body><p>árvíztűrő</p></body></html>`))
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}

	b := readFixture(t, fixture{files: map[string][]byte{
		"/#SYSTEM": buildSystem(3, []sysEntry{
			{sysTitle, []byte("Charset Book\x00")},
			{sysDefaultTopic, []byte("/a.html\x00")},
			// Hungarian: LCID default is windows-1250.
			{sysInfo, sysInfoEntry(0x040E)},
		}),
		"/a.html":  cp1251Topic,
		"/b.html":  cp1250Topic,
		"/pic.png": []byte("\x89PNG\r\n\x1a\n"),
	}})

	byHref := make(map[string]*book.Resource)
	for _, r := range b.Resources().All() {
		byHref[r.ResolvedHref] = r
	}

	t.Run("meta declaration wins over LCID", func(t *testing.T) {
		res := byHref["a.html"]
		if res == nil {
			t.Fatal("a.html resource missing")
		}
		if res.Charset != "windows-1251" {
			t.Errorf("Charset = %q, want windows-1251", res.Charset)
		}
		data, err := res.Data()
		if err != nil {
			t.Fatalf("Data() error: %v", err)
		}
		if !bytes.Equal(data, cp1251Topic) {
			t.Errorf("stored bytes not byte-identical:\n got %q\nwant %q", data, cp1251Topic)
		}
		if bytes.Contains(data, []byte("Привет")) {
			t.Error("stored bytes were silently transcoded to UTF-8")
		}
		rc, err := res.OpenAs(context.Background(), mediatype.PlainText)
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
	})

	t.Run("LCID fallback without declaration", func(t *testing.T) {
		res := byHref["b.html"]
		if res == nil {
			t.Fatal("b.html resource missing")
		}
		if res.Charset != "windows-1250" {
			t.Errorf("Charset = %q, want windows-1250", res.Charset)
		}
		data, err := res.Data()
		if err != nil {
			t.Fatalf("Data() error: %v", err)
		}
		if !bytes.Equal(data, cp1250Topic) {
			t.Errorf("stored bytes not byte-identical:\n got %q\nwant %q", data, cp1250Topic)
		}
		rc, err := res.OpenAs(context.Background(), mediatype.PlainText)
		if err != nil {
			t.Fatalf("OpenAs(plain text) error: %v", err)
		}
		text, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("ReadAll error: %v", err)
		}
		if !strings.Contains(string(text), "árvíztűrő") {
			t.Errorf("decoded text %q missing árvíztűrő", text)
		}
	})

	t.Run("binary stays binary", func(t *testing.T) {
		res := byHref["pic.png"]
		if res == nil {
			t.Fatal("pic.png resource missing")
		}
		if res.Charset != book.CharsetBinary {
			t.Errorf("Charset = %q, want binary", res.Charset)
		}
	})
}
