package converter

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"testing/iotest"

	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

func lrfToText(t *testing.T, r io.Reader) string {
	t.Helper()
	tr := &lrfToTextTransformer{from: mediatype.LRFText, to: mediatype.PlainText}
	rc, err := tr.Transform(context.Background(), r)
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

func TestLrfToText(t *testing.T) {
	plot := "\uFFFC"
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain", "Hello", "Hello"},
		{"unicode", "café — naïve", "café — naïve"},
		{"paragraphs kept", "a\n\nb", "a\n\nb"},
		{"plot dropped", "before" + plot + "after", "beforeafter"},
		{"plot at start", plot + "text", "text"},
		{"plot at end", "text" + plot, "text"},
		{"multiple plots", plot + "a" + plot + "b" + plot, "ab"},
		{"similar runes kept", "\uF000\uFFFD", "\uF000\uFFFD"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lrfToText(t, bytes.NewReader([]byte(tt.input))); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLrfToTextPartialPrefix(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  []byte
	}{
		{"lone ef", []byte{0xEF}, []byte{0xEF}},
		{"ef bf", []byte{0xEF, 0xBF}, []byte{0xEF, 0xBF}},
		{"ef then ascii", []byte{0xEF, 'x'}, []byte{0xEF, 'x'}},
		{"ef bf then ascii", []byte{0xEF, 0xBF, 'x'}, []byte{0xEF, 0xBF, 'x'}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := []byte(lrfToText(t, bytes.NewReader(tt.input)))
			if !bytes.Equal(got, tt.want) {
				t.Errorf("got % X, want % X", got, tt.want)
			}
		})
	}
}

func TestLrfToTextOneByteReads(t *testing.T) {
	input := "a\uFFFCC\uFFFCé"
	want := "aCé"
	if got := lrfToText(t, iotest.OneByteReader(bytes.NewReader([]byte(input)))); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLrfToTextNilReader(t *testing.T) {
	tr := &lrfToTextTransformer{from: mediatype.LRFText, to: mediatype.PlainText}
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

func TestLrfToTextContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tr := &lrfToTextTransformer{from: mediatype.LRFText, to: mediatype.PlainText}
	rc, err := tr.Transform(ctx, bytes.NewReader([]byte("text")))
	if err != nil {
		t.Fatalf("Transform() error: %v", err)
	}
	defer func() { _ = rc.Close() }()
	buf := make([]byte, 4)
	if _, err := rc.Read(buf); !errors.Is(err, context.Canceled) {
		t.Fatalf("Read() error = %v, want context.Canceled", err)
	}
}

func TestLrfToTextRegistered(t *testing.T) {
	if _, ok := DefaultRegistry.Find(mediatype.LRFText, mediatype.PlainText); !ok {
		t.Error("no LRFText -> PlainText transformer registered")
	}
}
