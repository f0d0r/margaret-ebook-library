package converter

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

func pmlToText(t *testing.T, input []byte) string {
	t.Helper()
	tr := &pmlToTextTransformer{from: mediatype.EreaderPML, to: mediatype.PlainText}
	rc, err := tr.Transform(context.Background(), bytes.NewReader(input))
	if err != nil {
		t.Fatalf("Transform() error: %v", err)
	}
	defer func() { _ = rc.Close() }()
	out, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	return string(out)
}

func TestPmlToText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain", "Hello", "Hello"},
		{"inline formatting", "\\bBold\\b and \\iit\\i", "Bold and it"},
		{"page break", "a\\pb", "a\n\nb"},
		{"newlines kept", "a\nb", "a\nb"},
		{"newlines collapsed", "a\n\n\nb", "a\n\nb"},
		{"comment", "a\\vnote\\vb", "ab"},
		{"char code", "\\a169", "\u00a9"},
		{"unicode lower", "\\U00e9", "\u00e9"},
		{"unicode upper", "\\U00E9", "\u00e9"},
		{"bad code literal", "\\axyz", "\\axyz"},
		{"image and link dropped", "\\m=\"i.png\"x\\q=\"#a\"y\\Q=\"z\"", "xy"},
		{"chapter", "\\C0=\"Ch\"\nbody", "Ch\n\nbody"},
		{"unknown tag dropped", "a\\zb", "ab"},
		{"trailing backslash", "ab\\", "ab\\"},
		{"crlf", "a\r\nb", "a\nb"},
		{"lone cr", "a\rb", "a\nb"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pmlToText(t, []byte(tt.input)); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPmlToTextCP1252(t *testing.T) {
	// The transformer itself expects UTF-8: the book layer decodes the
	// resource's original windows-1252 bytes centrally before the chain
	// runs. UTF-8 "é" must pass through byte-identically here; the
	// end-to-end windows-1252 -> UTF-8 path is covered in book tests.
	if got := pmlToText(t, []byte("é")); got != "é" {
		t.Errorf("got %q, want é", got)
	}
}

func TestPmlToTextRegistered(t *testing.T) {
	if _, ok := DefaultRegistry.Find(mediatype.EreaderPML, mediatype.PlainText); !ok {
		t.Error("no EreaderPML -> PlainText transformer registered")
	}
}
