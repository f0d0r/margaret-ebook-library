package converter

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

func fb3ToText(t *testing.T, in string) string {
	t.Helper()
	tr := &fb3BodyToTextTransformer{from: mediatype.FB3Body, to: mediatype.PlainText}
	rc, err := tr.Transform(context.Background(), strings.NewReader(in))
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

func TestFb3BodyToTextEdgeRegistered(t *testing.T) {
	if _, ok := DefaultRegistry.Find(mediatype.FB3Body, mediatype.PlainText); !ok {
		t.Error("FB3Body -> PlainText edge missing from DefaultRegistry")
	}
}

func TestFb3BodyToText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"paragraphs", `<section><p>Hello <strong>world</strong>.</p><p>Second.</p></section>`, "Hello world.\nSecond."},
		{"nested sections", `<section><p>a</p><section><p>b</p></section></section>`, "a\nb"},
		{"title and subtitle", `<section><title><p>T</p></title><subtitle>S</subtitle><p>x</p></section>`, "T\n\nS\nx"},
		{"poem lines", `<poem><stanza><v>one</v><v>two</v></stanza></poem>`, "one\ntwo"},
		{"list items", `<ul><li>one</li><li>two</li></ul>`, "one\ntwo"},
		{"table cells", `<table><tr><th>h</th><td>c</td></tr></table>`, "h\nc"},
		{"br break", `<p>a</p><br/><p>b</p>`, "a\n\nb"},
		{"entities", `Fish &amp; Chips &nbsp; X&#33; Y&#x41; &bogus; end`, "Fish & Chips \u00a0 X! YA &bogus; end"},
		{"unterminated entity literal", `a & b`, "a & b"},
		{"cdata literal", `<p><![CDATA[a <b> &amp;]]></p>`, "a <b> &amp;"},
		{"comments and PIs skipped", `<p>a<!-- <p>fake -->b<?pi?>c</p>`, "abc"},
		{"prefixed tags", `<fb3:p>hi</fb3:p>`, "hi"},
		{"img transparent", `<p><img src="x"/>caption</p>`, "caption"},
		{"broken nesting tolerated", `<p>a<div>b</p>c`, "ab\nc"},
		{"unknown end ignored", `text</weird>`, "text"},
		{"formatting whitespace dropped", "<p>a</p>\n   \n<p>b</p>", "a\nb"},
		{"inner spacing kept", `<p>  spaced   text  </p>`, "spaced   text  "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fb3ToText(t, tt.in); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFb3BodyToTextNilReader(t *testing.T) {
	tr := &fb3BodyToTextTransformer{from: mediatype.FB3Body, to: mediatype.PlainText}
	rc, err := tr.Transform(context.Background(), nil)
	if err != nil {
		t.Fatalf("Transform(nil) error: %v", err)
	}
	defer func() { _ = rc.Close() }()
	out, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("got %q, want empty", out)
	}
}
