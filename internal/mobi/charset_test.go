package mobi

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

func cp1252Bytes(t *testing.T, s string) []byte {
	t.Helper()
	b, err := charmap.Windows1252.NewEncoder().Bytes([]byte(s))
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	return b
}

func TestPalmDocReadPreservesCP1252(t *testing.T) {
	src := strings.Replace(palmDocBookHTMLPayload, "Test Palm Book", "Café Book", 1)
	src = strings.Replace(src, "Hello PalmDOC", "Unïcode café", 1)
	payload := cp1252Bytes(t, src)

	raw := buildPalmDocBookFile("Cafe", payload, palmDocBookOpt{compression: 2})
	b, err := NewMobiReader(config.DefaultConfig()).Read(book.NewBytesBlob(raw))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if got := b.Metadata().Title; got != "Café Book" {
		t.Errorf("Title = %q, want Café Book", got)
	}

	ro := b.Resources().ReadingOrder()
	if len(ro) == 0 {
		t.Fatal("no reading-order resources")
	}
	res := ro[0].Resource
	if res.Charset != "windows-1252" {
		t.Errorf("Charset = %q, want windows-1252", res.Charset)
	}
	data, err := res.Data()
	if err != nil {
		t.Fatalf("Data() error: %v", err)
	}
	if !bytes.Equal(data, payload) {
		t.Errorf("stored bytes not byte-identical:\n got %q\nwant %q", data, payload)
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
	if !strings.Contains(string(text), "Unïcode café") {
		t.Errorf("decoded text %q missing Unïcode café", text)
	}
}

// buildMobi6TestBlob assembles a minimal MOBI6 book with one text record.
func buildMobi6TestBlob(t *testing.T, text []byte, enc TextEncodingType) []byte {
	t.Helper()
	opts := defaultMobiHeaderOpts()
	opts.textEncoding = enc
	opts.textLength = uint32(len(text))
	opts.textRecordCount = 1
	return makePdbBlob(t, "TestMobi6", []recordSpec{
		makeMobiHeaderRecord(opts),
		{data: text},
	})
}

func TestMobi6CharsetFollowsTextEncoding(t *testing.T) {
	ctx := context.Background()
	src := `<html><head><title>T</title></head><body><p>Unïcode café</p></body></html>`

	t.Run("cp1252", func(t *testing.T) {
		payload := cp1252Bytes(t, src)
		b, err := NewMobiReader(config.DefaultConfig()).Read(book.NewBytesBlob(buildMobi6TestBlob(t, payload, CP1252)))
		if err != nil {
			t.Fatalf("Read() error: %v", err)
		}
		res := b.Resources().ReadingOrder()[0].Resource
		if res.Charset != "windows-1252" {
			t.Errorf("Charset = %q, want windows-1252", res.Charset)
		}
		data, err := res.Data()
		if err != nil {
			t.Fatalf("Data() error: %v", err)
		}
		if !bytes.Equal(data, payload) {
			t.Errorf("stored bytes not byte-identical:\n got %q\nwant %q", data, payload)
		}
		rc, err := res.OpenAs(ctx, mediatype.PlainText)
		if err != nil {
			t.Fatalf("OpenAs(plain text) error: %v", err)
		}
		text, _ := io.ReadAll(rc)
		_ = rc.Close()
		if !strings.Contains(string(text), "Unïcode café") {
			t.Errorf("decoded text %q missing Unïcode café", text)
		}
	})

	t.Run("utf8", func(t *testing.T) {
		payload := []byte(src)
		b, err := NewMobiReader(config.DefaultConfig()).Read(book.NewBytesBlob(buildMobi6TestBlob(t, payload, UTF8)))
		if err != nil {
			t.Fatalf("Read() error: %v", err)
		}
		res := b.Resources().ReadingOrder()[0].Resource
		if res.Charset != book.CharsetUTF8 {
			t.Errorf("Charset = %q, want utf-8", res.Charset)
		}
		data, err := res.Data()
		if err != nil {
			t.Fatalf("Data() error: %v", err)
		}
		if !bytes.Equal(data, payload) {
			t.Errorf("stored bytes not byte-identical:\n got %q\nwant %q", data, payload)
		}
	})
}

func TestPalmDocCharsetIsCP1252(t *testing.T) {
	raw := buildPalmDocBookFile("Plain", []byte(palmDocBookPlainPayload), palmDocBookOpt{})
	b, err := NewMobiReader(config.DefaultConfig()).Read(book.NewBytesBlob(raw))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	res := b.Resources().ReadingOrder()[0].Resource
	if res.Charset != "windows-1252" {
		t.Errorf("plain PalmDOC Charset = %q, want windows-1252", res.Charset)
	}
}
