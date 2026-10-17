package ereader

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/internal/config"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
	"golang.org/x/text/encoding/charmap"
)

func TestEreaderReadPreservesCP1252(t *testing.T) {
	src := `\Café\BBy é\B and café line`
	pml, err := charmap.Windows1252.NewEncoder().Bytes([]byte(src))
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}

	// The 132/dropbook zlib path preserves the stored bytes verbatim
	// (the 202 path always runs PalmDOC decompression over the records).
	b := buildPDB(t, "Doc", "PNRdPPrs", [][]byte{
		header132(header132Fields{compression: 10, nonText: 2}),
		zlibCompress(t, string(pml)),
	})
	bk, err := NewEreaderReader(config.DefaultConfig()).Read(b)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	res := bk.Resources().All()[0]
	if res.Charset != "windows-1252" {
		t.Errorf("Charset = %q, want windows-1252", res.Charset)
	}
	data, err := res.Data()
	if err != nil {
		t.Fatalf("Data() error: %v", err)
	}
	if !bytes.Equal(data, pml) {
		t.Errorf("stored bytes not byte-identical:\n got %q\nwant %q", data, pml)
	}
	if bytes.Contains(data, []byte("café")) {
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
	if !strings.Contains(string(text), "café line") {
		t.Errorf("decoded text %q missing café line", text)
	}
}
