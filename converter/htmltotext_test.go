package converter

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

func readAll(t *testing.T, rc io.ReadCloser) string {
	t.Helper()
	defer rc.Close()
	b, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	return string(b)
}

func transformString(t *testing.T, from string, input string) string {
	t.Helper()
	tr, ok := DefaultRegistry.Find(from, mediatype.PlainText)
	if !ok {
		t.Fatalf("no transformer for %s -> %s", from, mediatype.PlainText)
	}
	rc, err := tr.Transform(context.Background(), strings.NewReader(input))
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}
	return readAll(t, rc)
}

func TestHtmlToTextTransformer_FindAndTransform(t *testing.T) {
	tests := []struct {
		name  string
		from  string
		input string
		want  string
	}{
		{"empty", mediatype.XHTML, "", ""},
		{"plain text", mediatype.XHTML, "Hello, World!", "Hello, World!"},
		{"simple tag", mediatype.XHTML, "<p>Hello</p>", "Hello"},
		{"nested", mediatype.XHTML, "<div><p><span>Nested</span></p></div>", "Nested"},
		{"multiple", mediatype.XHTML, "<p>First</p><p>Second</p>", "FirstSecond"},
		{"whitespace", mediatype.XHTML, "<p>First</p> <p>Second</p>", "First Second"},
		{"entities", mediatype.XHTML, "<p>&lt;tag&gt;</p>", "<tag>"},
		{"unicode", mediatype.XHTML, "<p>Hello 世界 🌍</p>", "Hello 世界 🌍"},
		{"comment ignored", mediatype.XHTML, "<p>Before</p><!-- comment --><p>After</p>", "BeforeAfter"},
		{"doctype", mediatype.XHTML, "<!DOCTYPE html><p>Content</p>", "Content"},
		{"html alias", mediatype.HTML, "<p>HTML</p>", "HTML"},
		{"mobi alias", mediatype.MobiHTML, "<div>Mobi</div>", "Mobi"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := transformString(t, tt.from, tt.input)
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHtmlToTextTransformer_Transform_NilReader(t *testing.T) {
	tr, _ := DefaultRegistry.Find(mediatype.XHTML, mediatype.PlainText)
	rc, err := tr.Transform(context.Background(), nil)
	if err != nil {
		t.Fatalf("nil reader should not error, got %v", err)
	}
	if got := readAll(t, rc); got != "" {
		t.Fatalf("nil reader should give empty, got %q", got)
	}
}

func TestHtmlToTextTransformer_Streaming_PartialBuffers(t *testing.T) {
	tr, _ := DefaultRegistry.Find(mediatype.HTML, mediatype.PlainText)
	rc, err := tr.Transform(context.Background(), strings.NewReader("<p>abcdefghij</p>"))
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}
	defer rc.Close()

	// Read with 3-byte buffer repeatedly — tests buf remainder logic
	buf := make([]byte, 3)
	var out bytes.Buffer
	for {
		n, e := rc.Read(buf)
		if n > 0 {
			out.Write(buf[:n])
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatalf("Read failed: %v", e)
		}
	}
	if got := out.String(); got != "abcdefghij" {
		t.Fatalf("partial buffer read got %q, want %q", got, "abcdefghij")
	}
}

func TestHtmlToTextTransformer_Streaming_LargeToken(t *testing.T) {
	// Token longer than read buffer — remainder must be preserved
	long := strings.Repeat("x", 10000)
	input := "<p>" + long + "</p>"
	tr, _ := DefaultRegistry.Find(mediatype.XHTML, mediatype.PlainText)
	rc, _ := tr.Transform(context.Background(), strings.NewReader(input))
	defer rc.Close()

	buf := make([]byte, 512)
	var out bytes.Buffer
	for {
		n, err := rc.Read(buf)
		if n > 0 {
			out.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}
	}
	if got := out.String(); got != long {
		t.Fatalf("large token len %d, want %d", len(got), len(long))
	}
}

func TestHtmlToTextTransformer_Streaming_Unicode(t *testing.T) {
	input := "<p>Hello 世界 🌍</p>"
	tr, _ := DefaultRegistry.Find(mediatype.XHTML, mediatype.PlainText)
	rc, _ := tr.Transform(context.Background(), strings.NewReader(input))
	defer rc.Close()

	// 1-byte buffer to stress remainder handling with multi-byte runes
	buf := make([]byte, 1)
	var out bytes.Buffer
	for {
		n, err := rc.Read(buf)
		if n > 0 {
			out.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}
	}
	if got := out.String(); got != "Hello 世界 🌍" {
		t.Fatalf("unicode streaming got %q, want %q", got, "Hello 世界 🌍")
	}
}

func TestHtmlToTextTransformer_Streaming_MultipleTokens(t *testing.T) {
	// Multiple text tokens across tags
	input := "<p>First</p><p>Second</p><p>Third</p>"
	tr, _ := DefaultRegistry.Find(mediatype.XHTML, mediatype.PlainText)
	rc, _ := tr.Transform(context.Background(), strings.NewReader(input))
	defer rc.Close()

	// Buffer smaller than first token but larger than second — fill loop
	buf := make([]byte, 4)
	var out bytes.Buffer
	for {
		n, err := rc.Read(buf)
		if n > 0 {
			out.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}
	}
	if got := out.String(); got != "FirstSecondThird" {
		t.Fatalf("multiple tokens got %q, want %q", got, "FirstSecondThird")
	}
}

func TestHtmlToTextTransformer_Close_Propagates(t *testing.T) {
	closed := false
	inner := &closeTracker{Reader: strings.NewReader("<p>hi</p>"), closed: &closed}
	tr, _ := DefaultRegistry.Find(mediatype.XHTML, mediatype.PlainText)
	rc, err := tr.Transform(context.Background(), inner)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}
	// reading should work
	if got := readAll(t, rc); got != "hi" {
		t.Fatalf("got %q, want %q", got, "hi")
	}
	// readAll closes rc, which should propagate to inner
	if !closed {
		t.Fatalf("Close should propagate to underlying closer")
	}
}

func TestHtmlToTextTransformer_Read_AfterEOF(t *testing.T) {
	tr, _ := DefaultRegistry.Find(mediatype.XHTML, mediatype.PlainText)
	rc, _ := tr.Transform(context.Background(), strings.NewReader("<p>hi</p>"))
	defer rc.Close()

	b, _ := io.ReadAll(rc)
	if string(b) != "hi" {
		t.Fatalf("got %q, want hi", string(b))
	}
	buf := make([]byte, 10)
	n, err := rc.Read(buf)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("after EOF should return io.EOF, got %v", err)
	}
	if n != 0 {
		t.Fatalf("after EOF n should be 0, got %d", n)
	}
}

func TestHtmlToTextTransformer_ContextCancel_BeforeRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tr, _ := DefaultRegistry.Find(mediatype.XHTML, mediatype.PlainText)
	rc, _ := tr.Transform(ctx, strings.NewReader("<p>hello</p>"))
	defer rc.Close()

	buf := make([]byte, 10)
	_, err := rc.Read(buf)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("should be context.Canceled, got %v", err)
	}
}

func TestHtmlToTextTransformer_ImplementsTransformer(t *testing.T) {
	var _ Transformer = (*htmlToTextTransformer)(nil)
	tr := &htmlToTextTransformer{from: mediatype.XHTML, to: mediatype.PlainText}
	if tr.From() != mediatype.XHTML || tr.To() != mediatype.PlainText {
		t.Fatalf("From/To mismatch")
	}
}

func TestHtmlToTextTransformer_ErrorsDontReturnZeroNilEOF(t *testing.T) {
	// Ensure we don't return (0, nil) at EOF — would cause infinite loop in io.Copy
	tr, _ := DefaultRegistry.Find(mediatype.XHTML, mediatype.PlainText)
	rc, _ := tr.Transform(context.Background(), strings.NewReader(""))
	defer rc.Close()
	buf := make([]byte, 10)
	n, err := rc.Read(buf)
	if n != 0 || !errors.Is(err, io.EOF) {
		t.Fatalf("empty should return 0, EOF, got %d, %v", n, err)
	}
}

// closeTracker for htmltotext tests
type closeTracker struct {
	io.Reader
	closed *bool
}

func (c *closeTracker) Close() error { *c.closed = true; return nil }
