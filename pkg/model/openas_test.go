package model

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/pkg/converter"
	"github.com/f0d0r/margaret-ebook-library/pkg/errs"
	"github.com/f0d0r/margaret-ebook-library/pkg/mediatype"
)

func mustReadAll(t *testing.T, rc io.ReadCloser) string {
	t.Helper()
	defer rc.Close()
	b, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	return string(b)
}

// upperTransformer is a test helper that upper-cases bytes for BFS chain tests.
type upperTransformer struct{ from, to string }

func (u *upperTransformer) From() string { return u.from }
func (u *upperTransformer) To() string   { return u.to }
func (u *upperTransformer) Transform(_ context.Context, r io.Reader) (io.ReadCloser, error) {
	b, _ := io.ReadAll(r)
	if c, ok := r.(io.Closer); ok {
		_ = c.Close()
	}
	return io.NopCloser(bytes.NewReader(bytes.ToUpper(b))), nil
}

type closeTrackerOpenAs struct {
	io.Reader
	closed *bool
}

func (c *closeTrackerOpenAs) Close() error { *c.closed = true; return nil }

func TestResource_OpenAs_Identity(t *testing.T) {
	ctx := context.Background()
	r := &Resource{MediaType: mediatype.PlainText, Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("hello")), nil
	}}
	rc, err := r.OpenAs(ctx, mediatype.PlainText)
	if err != nil {
		t.Fatalf("identity should not error, got %v", err)
	}
	if got := mustReadAll(t, rc); got != "hello" {
		t.Fatalf("got %q, want %q", got, "hello")
	}
}

func TestResource_OpenAs_Identity_Normalized(t *testing.T) {
	ctx := context.Background()
	r := &Resource{MediaType: "TEXT/PLAIN; charset=utf-8", Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("hi")), nil
	}}
	rc, err := r.OpenAs(ctx, "text/plain")
	if err != nil {
		t.Fatalf("normalized identity should not error, got %v", err)
	}
	if got := mustReadAll(t, rc); got != "hi" {
		t.Fatalf("got %q, want %q", got, "hi")
	}
}

func TestResource_OpenAs_XHTMLToPlain(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		from string
		html string
		want string
	}{
		{"xhtml", mediatype.XHTML, "<p>Hello</p>", "Hello"},
		{"html", mediatype.HTML, "<p>HTML</p>", "HTML"},
		{"mobi", mediatype.MobiHTML, "<div>Mobi</div>", "Mobi"},
		{"charset", "application/xhtml+xml; charset=utf-8", "<p>hi</p>", "hi"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Resource{MediaType: tt.from, Open: func() (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader(tt.html)), nil
			}}
			rc, err := r.OpenAs(ctx, mediatype.PlainText)
			if err != nil {
				t.Fatalf("OpenAs failed: %v", err)
			}
			if got := mustReadAll(t, rc); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResource_OpenAs_TargetCharsetNormalized(t *testing.T) {
	ctx := context.Background()
	r := &Resource{MediaType: mediatype.XHTML, Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("<p>hi</p>")), nil
	}}
	rc, err := r.OpenAs(ctx, "text/plain; charset=utf-8")
	if err != nil {
		t.Fatalf("charset target should normalize, got %v", err)
	}
	if got := mustReadAll(t, rc); got != "hi" {
		t.Fatalf("got %q, want %q", got, "hi")
	}
}

func TestResource_OpenAs_NoTransformer(t *testing.T) {
	ctx := context.Background()
	r := &Resource{MediaType: mediatype.JPEG, Open: func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader([]byte{1, 2})), nil
	}}
	_, err := r.OpenAs(ctx, mediatype.PlainText)
	if !errors.Is(err, errs.ErrNoTransformer) {
		t.Fatalf("should be ErrNoTransformer, got %v", err)
	}
	if err.Error() == "" || !strings.Contains(err.Error(), "image/jpeg") {
		t.Fatalf("error should mention MIME, got %v", err)
	}
}

func TestResource_OpenAs_UnsupportedMediaType(t *testing.T) {
	ctx := context.Background()
	r := &Resource{MediaType: "", Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("x")), nil
	}}
	_, err := r.OpenAs(ctx, mediatype.PlainText)
	if !errors.Is(err, errs.ErrUnsupportedMediaType) {
		t.Fatalf("empty from should be ErrUnsupportedMediaType, got %v", err)
	}
	r2 := &Resource{MediaType: mediatype.XHTML, Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("x")), nil
	}}
	_, err = r2.OpenAs(ctx, "")
	if !errors.Is(err, errs.ErrUnsupportedMediaType) {
		t.Fatalf("empty target should be ErrUnsupportedMediaType, got %v", err)
	}
}

func TestResource_OpenAs_NilResource(t *testing.T) {
	var r *Resource
	_, err := r.OpenAs(context.Background(), mediatype.PlainText)
	if err == nil || !strings.Contains(err.Error(), "resource is nil") {
		t.Fatalf("nil resource should error, got %v", err)
	}
}

func TestResource_OpenAs_NilOpen(t *testing.T) {
	r := &Resource{MediaType: mediatype.XHTML, Open: nil}
	_, err := r.OpenAs(context.Background(), mediatype.PlainText)
	if err == nil || !strings.Contains(err.Error(), "not openable") {
		t.Fatalf("nil Open should error, got %v", err)
	}
}

func TestResource_OpenAs_OpenErrorPropagates(t *testing.T) {
	r := &Resource{MediaType: mediatype.PlainText, Open: func() (io.ReadCloser, error) {
		return nil, errors.New("open fail")
	}}
	_, err := r.OpenAs(context.Background(), mediatype.PlainText)
	if err == nil || !strings.Contains(err.Error(), "open fail") {
		t.Fatalf("should propagate Open error, got %v", err)
	}
	// also for transformed path
	r2 := &Resource{MediaType: mediatype.XHTML, Open: func() (io.ReadCloser, error) {
		return nil, errors.New("open fail2")
	}}
	_, err = r2.OpenAs(context.Background(), mediatype.PlainText)
	if err == nil || !strings.Contains(err.Error(), "open fail2") {
		t.Fatalf("transformed Open error should propagate, got %v", err)
	}
}

func TestResource_OpenAs_ClosePropagation(t *testing.T) {
	closed := false
	r := &Resource{MediaType: mediatype.XHTML, Open: func() (io.ReadCloser, error) {
		return &closeTrackerOpenAs{Reader: strings.NewReader("<p>hi</p>"), closed: &closed}, nil
	}}
	rc, err := r.OpenAs(context.Background(), mediatype.PlainText)
	if err != nil {
		t.Fatalf("OpenAs failed: %v", err)
	}
	// reading should work
	if got := mustReadAll(t, rc); got != "hi" {
		t.Fatalf("got %q, want %q", got, "hi")
	}
	if !closed {
		t.Fatalf("Close should propagate to underlying")
	}
}

func TestResource_OpenAs_ChainedTransform(t *testing.T) {
	ctx := context.Background()
	reg := converter.NewRegistry(
		&upperTransformer{from: mediatype.XHTML, to: mediatype.HTML},
		&upperTransformer{from: mediatype.HTML, to: mediatype.PlainText},
	)
	r := &Resource{MediaType: mediatype.XHTML, Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("hello")), nil
	}}
	rc, err := r.OpenAsWithRegistry(ctx, reg, mediatype.PlainText)
	if err != nil {
		t.Fatalf("chained OpenAs failed: %v", err)
	}
	if got := mustReadAll(t, rc); got != "HELLO" {
		t.Fatalf("chained got %q, want %q", got, "HELLO")
	}
}

func TestResource_OpenAsWithRegistry_Isolation(t *testing.T) {
	ctx := context.Background()
	custom := converter.NewRegistry()
	r := &Resource{MediaType: mediatype.XHTML, Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("<p>x</p>")), nil
	}}
	_, err := r.OpenAsWithRegistry(ctx, custom, mediatype.PlainText)
	if !errors.Is(err, errs.ErrNoTransformer) {
		t.Fatalf("isolated registry should give ErrNoTransformer, got %v", err)
	}
	// DefaultRegistry still works
	rc, err := r.OpenAs(ctx, mediatype.PlainText)
	if err != nil {
		t.Fatalf("default should still work, got %v", err)
	}
	if got := mustReadAll(t, rc); got != "x" {
		t.Fatalf("got %q, want %q", got, "x")
	}
}

func TestResource_OpenAs_NilRegistryUsesDefault(t *testing.T) {
	ctx := context.Background()
	r := &Resource{MediaType: mediatype.XHTML, Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("<p>hi</p>")), nil
	}}
	rc, err := r.OpenAsWithRegistry(ctx, nil, mediatype.PlainText)
	if err != nil {
		t.Fatalf("nil registry should fallback to DefaultRegistry, got %v", err)
	}
	if got := mustReadAll(t, rc); got != "hi" {
		t.Fatalf("got %q, want %q", got, "hi")
	}
}

func TestResource_DataAs(t *testing.T) {
	ctx := context.Background()
	r := &Resource{MediaType: mediatype.XHTML, Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("<p>DataAs</p>")), nil
	}}
	b, err := r.DataAs(ctx, mediatype.PlainText)
	if err != nil {
		t.Fatalf("DataAs failed: %v", err)
	}
	if string(b) != "DataAs" {
		t.Fatalf("got %q, want %q", string(b), "DataAs")
	}
}

func TestResource_DataAsWithRegistry(t *testing.T) {
	ctx := context.Background()
	reg := converter.NewRegistry(&upperTransformer{from: mediatype.XHTML, to: mediatype.PlainText})
	r := &Resource{MediaType: mediatype.XHTML, Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("hello")), nil
	}}
	b, err := r.DataAsWithRegistry(ctx, reg, mediatype.PlainText)
	if err != nil {
		t.Fatalf("DataAsWithRegistry failed: %v", err)
	}
	if string(b) != "HELLO" {
		t.Fatalf("got %q, want %q", string(b), "HELLO")
	}
}

func TestResource_OpenAs_StreamingSmallBuffer(t *testing.T) {
	ctx := context.Background()
	r := &Resource{MediaType: mediatype.HTML, Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("<p>abcdefghij</p>")), nil
	}}
	rc, err := r.OpenAs(ctx, mediatype.PlainText)
	if err != nil {
		t.Fatalf("OpenAs failed: %v", err)
	}
	defer rc.Close()
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
		t.Fatalf("got %q, want %q", got, "abcdefghij")
	}
}

func TestResource_OpenAs_TransformStepError(t *testing.T) {
	ctx := context.Background()
	failTransformer := &failingTransformer{from: mediatype.XHTML, to: mediatype.PlainText}
	reg := converter.NewRegistry(failTransformer)
	r := &Resource{MediaType: mediatype.XHTML, Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("<p>hi</p>")), nil
	}}
	_, err := r.OpenAsWithRegistry(ctx, reg, mediatype.PlainText)
	if err == nil || !strings.Contains(err.Error(), "transform step") {
		t.Fatalf("should wrap transform step error, got %v", err)
	}
}

type failingTransformer struct{ from, to string }

func (f *failingTransformer) From() string { return f.from }
func (f *failingTransformer) To() string   { return f.to }
func (f *failingTransformer) Transform(_ context.Context, _ io.Reader) (io.ReadCloser, error) {
	return nil, errors.New("transform fail")
}
