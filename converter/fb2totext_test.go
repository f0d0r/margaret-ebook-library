package converter

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

func fb2ToText(t *testing.T, in string) string {
	t.Helper()
	tr := &fb2BodyToTextTransformer{from: mediatype.FB2Body, to: mediatype.PlainText}
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

func TestFb2BodyToTextEdgeRegistered(t *testing.T) {
	if _, ok := DefaultRegistry.Find(mediatype.FB2Body, mediatype.PlainText); !ok {
		t.Error("FB2Body -> PlainText edge missing from DefaultRegistry")
	}
}

func TestFb2BodyToText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"paragraphs", `<section><p>Hello <strong>world</strong>.</p><p>Second.</p></section>`, "Hello world.\nSecond."},
		{"nested sections", `<section><p>a</p><section><p>b</p></section></section>`, "a\nb"},
		{"poem lines", `<poem><stanza><v>one</v><v>two</v></stanza></poem>`, "one\ntwo"},
		{"entities", `Fish &amp; Chips &nbsp; X&#33; Y&#x41; &bogus; end`, "Fish & Chips \u00a0 X! YA &bogus; end"},
		{"unterminated entity literal", `a & b`, "a & b"},
		{"cdata literal", `<p><![CDATA[a <b> &amp;]]></p>`, "a <b> &amp;"},
		{"comments and PIs skipped", `<p>a<!-- <p>fake -->b<?pi?>c</p>`, "abc"},
		{"prefixed tags", `<fb:p>hi</fb:p>`, "hi"},
		{"binary skipped", `<binary>ZZZ</binary><p>ok</p>`, "ok"},
		{"stylesheet skipped", `<stylesheet>p{}</stylesheet><p>ok</p>`, "ok"},
		{"empty-line blank", `<p>a</p><empty-line/><p>b</p>`, "a\n\nb"},
		{"broken nesting tolerated", `<p>a<div>b</p>c`, "ab\nc"},
		{"unknown end ignored", `text</weird>`, "text"},
		{"unclosed binary to EOF", `<p>a</p><binary>ZZZ`, "a"},
		{"formatting whitespace dropped", "<p>a</p>\n   \n<p>b</p>", "a\nb"},
		{"inner spacing kept", `<p>  spaced   text  </p>`, "spaced   text  "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fb2ToText(t, tt.in); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFb2BodyToTextNilReader(t *testing.T) {
	tr := &fb2BodyToTextTransformer{from: mediatype.FB2Body, to: mediatype.PlainText}
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

func TestFb2BodyToTextSmallBuffer(t *testing.T) {
	tr := &fb2BodyToTextTransformer{from: mediatype.FB2Body, to: mediatype.PlainText}
	rc, err := tr.Transform(context.Background(), strings.NewReader(`<p>Hello world, this is a longer paragraph.</p><p>Second.</p>`))
	if err != nil {
		t.Fatalf("Transform() error: %v", err)
	}
	defer func() { _ = rc.Close() }()
	var sb strings.Builder
	buf := make([]byte, 3)
	for {
		n, err := rc.Read(buf)
		sb.Write(buf[:n])
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read() error: %v", err)
		}
	}
	want := "Hello world, this is a longer paragraph.\nSecond."
	if sb.String() != want {
		t.Errorf("got %q, want %q", sb.String(), want)
	}
}
