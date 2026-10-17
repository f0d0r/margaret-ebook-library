package fb3

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
	"golang.org/x/text/encoding/charmap"
)

func TestFb3ReadReportsBodyCharset(t *testing.T) {
	cp1251Body, err := charmap.Windows1251.NewEncoder().Bytes([]byte(
		`<?xml version="1.0" encoding="windows-1251"?>` +
			`<fb3-body xmlns="http://www.fictionbook.org/FictionBook3/body" id="6079f35b-49cc-4211-adba-919ccaaff4f2">` +
			`<title><p>Глава</p></title>` +
			`<section><p>Привет, мир!</p></section>` +
			`</fb3-body>`))
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}

	files := []fb3File{
		{"[Content_Types].xml", minimalContentTypes},
		{"_rels/.rels", minimalRootRels},
		{"fb3/description.xml", minimalDescription},
		{"fb3/_rels/description.xml.rels", minimalDescRels},
		{"fb3/body.xml", string(cp1251Body)},
		{"fb3/_rels/body.xml.rels", minimalBodyRels},
		{"fb3/img/cover.png", "PNGDATA"},
	}

	ebook, err := NewFb3Reader(config.DefaultConfig()).Read(buildFb3Blob(t, files))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	byHref := make(map[string]*book.Resource)
	for _, r := range ebook.Resources().All() {
		byHref[r.ResolvedHref] = r
	}

	var body *book.Resource
	for href, r := range byHref {
		if strings.Contains(href, "body") {
			body = r
		}
	}
	if body == nil {
		t.Fatalf("no body resource among %v", byHref)
	}
	if body.Charset != "windows-1251" {
		t.Errorf("body Charset = %q, want windows-1251", body.Charset)
	}

	data, err := body.Data()
	if err != nil {
		t.Fatalf("body Data() error: %v", err)
	}
	if !bytes.Equal(data, cp1251Body) {
		t.Errorf("body stored bytes not byte-identical:\n got %q\nwant %q", data, cp1251Body)
	}
	if bytes.Contains(data, []byte("Привет")) {
		t.Error("body stored bytes were silently transcoded to UTF-8")
	}

	rc, err := body.OpenAs(context.Background(), mediatype.PlainText)
	if err != nil {
		t.Fatalf("body OpenAs(plain text) error: %v", err)
	}
	text, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("body ReadAll error: %v", err)
	}
	if !strings.Contains(string(text), "Привет, мир!") {
		t.Errorf("body decoded text %q missing Привет, мир!", text)
	}

	for _, r := range ebook.Resources().All() {
		if strings.HasPrefix(r.MediaType, "image/") && r.Charset != book.CharsetBinary {
			t.Errorf("image %s Charset = %q, want binary", r.ResolvedHref, r.Charset)
		}
	}
}
