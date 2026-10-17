package xml

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"
	"unicode/utf16"

	"golang.org/x/text/encoding/charmap"
)

func TestCharsetReader(t *testing.T) {
	t.Run("decodes known labels", func(t *testing.T) {
		raw, err := charmap.Windows1251.NewEncoder().Bytes([]byte("Привет"))
		if err != nil {
			t.Fatalf("encode fixture: %v", err)
		}
		r, err := CharsetReader("windows-1251", bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("CharsetReader: %v", err)
		}
		got, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if string(got) != "Привет" {
			t.Errorf("got %q, want Привет", got)
		}
	})

	t.Run("passes utf-8 through unchanged", func(t *testing.T) {
		raw := []byte("café")
		r, err := CharsetReader("UTF-8", bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("CharsetReader: %v", err)
		}
		got, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if !bytes.Equal(got, raw) {
			t.Errorf("got %x, want %x", got, raw)
		}
	})

	t.Run("unknown labels error", func(t *testing.T) {
		if _, err := CharsetReader("no-such-charset", strings.NewReader("")); err == nil {
			t.Fatal("unknown label should error")
		}
	})
}

// utf16Bytes encodes s as UTF-16 in the given byte order, optionally with BOM.
func utf16Bytes(s string, bom bool, bigEndian bool) []byte {
	var out []byte
	if bom {
		if bigEndian {
			out = append(out, 0xFE, 0xFF)
		} else {
			out = append(out, 0xFF, 0xFE)
		}
	}
	for _, u := range utf16.Encode([]rune(s)) {
		if bigEndian {
			out = append(out, byte(u>>8), byte(u))
		} else {
			out = append(out, byte(u), byte(u>>8))
		}
	}
	return out
}

type xmlDoc struct {
	XMLName xml.Name `xml:"doc"`
	Text    string   `xml:"text"`
}

func TestNewDecoder(t *testing.T) {
	const want = `<doc><text>Prívet</text></doc>`
	tests := []struct {
		name string
		raw  []byte
	}{
		{"utf-16be with BOM", utf16Bytes(want, true, true)},
		{"utf-16le with BOM", utf16Bytes(want, true, false)},
		{"utf-16be without BOM", utf16Bytes(want, false, true)},
		{"utf-16le without BOM", utf16Bytes(want, false, false)},
		{"utf-8 passes through", []byte(want)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d xmlDoc
			if err := NewDecoder(bytes.NewReader(tt.raw)).Decode(&d); err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if d.Text != "Prívet" {
				t.Errorf("Text = %q, want Prívet", d.Text)
			}
		})
	}
}

func TestNewDecoderDeclaredEncoding(t *testing.T) {
	// A UTF-8 byte stream declaring windows-1251 must still go through
	// the shared CharsetReader.
	raw, err := charmap.Windows1251.NewEncoder().Bytes([]byte(
		`<?xml version="1.0" encoding="windows-1251"?><doc><text>Привет</text></doc>`))
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	var d xmlDoc
	if err := NewDecoder(bytes.NewReader(raw)).Decode(&d); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if d.Text != "Привет" {
		t.Errorf("Text = %q, want Привет", d.Text)
	}
}
