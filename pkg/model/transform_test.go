package model

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/pkg/converter"
	"github.com/f0d0r/margaret-ebook-library/pkg/mediatype"
)

func TestResourceSet_OpenReadingOrderAs_Basic(t *testing.T) {
	ctx := context.Background()
	ra := &Resource{Id: "a", MediaType: mediatype.XHTML, ResolvedHref: "a.xhtml", Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("<p>First</p>")), nil
	}}
	rb := &Resource{Id: "b", MediaType: mediatype.HTML, ResolvedHref: "b.html", Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("<p>Second</p>")), nil
	}}
	rcNon := &Resource{Id: "c", MediaType: mediatype.XHTML, ResolvedHref: "c.xhtml", Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("<p>Skip</p>")), nil
	}}
	rs := NewResourceSet([]*Resource{ra, rb, rcNon}, []ReadingOrderItem{
		{Resource: ra, Linear: true},
		{Resource: rb, Linear: true},
		{Resource: rcNon, Linear: false},
	}, nil)

	rc, err := rs.OpenReadingOrderAs(ctx, mediatype.PlainText)
	if err != nil {
		t.Fatalf("OpenReadingOrderAs failed: %v", err)
	}
	b, _ := io.ReadAll(rc)
	_ = rc.Close()
	if got := string(b); got != "FirstSecond" {
		t.Fatalf("got %q, want %q", got, "FirstSecond")
	}
	// ensure non-linear skipped even with direct read
	if strings.Contains(string(b), "Skip") {
		t.Fatalf("should not contain non-linear")
	}
}

func TestResourceSet_OpenReadingOrderAs_Empty(t *testing.T) {
	ctx := context.Background()
	rs := NewResourceSet(nil, nil, nil)
	rc, err := rs.OpenReadingOrderAs(ctx, mediatype.PlainText)
	if err != nil {
		t.Fatalf("empty should not error, got %v", err)
	}
	b, _ := io.ReadAll(rc)
	_ = rc.Close()
	if len(b) != 0 {
		t.Fatalf("empty should give empty, got %q", string(b))
	}

	// nil ResourceSet
	var nilRS *ResourceSet
	_, err = nilRS.OpenReadingOrderAs(ctx, mediatype.PlainText)
	if err == nil {
		t.Fatalf("nil ResourceSet should error")
	}
}

func TestResourceSet_OpenReadingOrderAs_OnlyNonLinear(t *testing.T) {
	ctx := context.Background()
	ra := &Resource{Id: "a", MediaType: mediatype.XHTML, ResolvedHref: "a.xhtml", Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("<p>Skip</p>")), nil
	}}
	rs := NewResourceSet([]*Resource{ra}, []ReadingOrderItem{{Resource: ra, Linear: false}}, nil)
	rc, err := rs.OpenReadingOrderAs(ctx, mediatype.PlainText)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	b, _ := io.ReadAll(rc)
	_ = rc.Close()
	if len(b) != 0 {
		t.Fatalf("only non-linear should be empty, got %q", string(b))
	}
}

func TestResourceSet_OpenReadingOrderAs_LazyAndSmallBuffer(t *testing.T) {
	ctx := context.Background()
	opens := 0
	ra := &Resource{Id: "a", MediaType: mediatype.XHTML, ResolvedHref: "a.xhtml", Open: func() (io.ReadCloser, error) {
		opens++
		return io.NopCloser(strings.NewReader("<p>A</p>")), nil
	}}
	rb := &Resource{Id: "b", MediaType: mediatype.XHTML, ResolvedHref: "b.xhtml", Open: func() (io.ReadCloser, error) {
		opens++
		return io.NopCloser(strings.NewReader("<p>B</p>")), nil
	}}
	rs := NewResourceSet([]*Resource{ra, rb}, []ReadingOrderItem{{Resource: ra, Linear: true}, {Resource: rb, Linear: true}}, nil)
	rc, err := rs.OpenReadingOrderAs(ctx, mediatype.PlainText)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	defer rc.Close()

	// reading with 1-byte buffer stresses multiReadCloser switching
	buf := make([]byte, 1)
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
	if got := out.String(); got != "AB" {
		t.Fatalf("got %q, want %q", got, "AB")
	}
	if opens != 2 {
		t.Fatalf("should have opened 2 resources lazily, got %d", opens)
	}
}

func TestResourceSet_OpenReadingOrderAs_CloseIdempotent(t *testing.T) {
	ctx := context.Background()
	ra := &Resource{Id: "a", MediaType: mediatype.XHTML, ResolvedHref: "a.xhtml", Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("<p>A</p>")), nil
	}}
	rs := NewResourceSet([]*Resource{ra}, []ReadingOrderItem{{Resource: ra, Linear: true}}, nil)
	rc, _ := rs.OpenReadingOrderAs(ctx, mediatype.PlainText)
	// read partially then close twice
	buf := make([]byte, 10)
	_, _ = rc.Read(buf)
	if err := rc.Close(); err != nil {
		t.Fatalf("first close should not error, got %v", err)
	}
	if err := rc.Close(); err != nil {
		t.Fatalf("second close should be idempotent, got %v", err)
	}
}

func TestResourceSet_OpenReadingOrderAs_EmptyTarget(t *testing.T) {
	ra := &Resource{Id: "a", MediaType: mediatype.XHTML, ResolvedHref: "a.xhtml", Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("<p>A</p>")), nil
	}}
	rs := NewResourceSet([]*Resource{ra}, []ReadingOrderItem{{Resource: ra, Linear: true}}, nil)
	_, err := rs.OpenReadingOrderAs(context.Background(), "")
	if err == nil {
		t.Fatalf("empty target should error")
	}
}

func TestResourceSet_OpenReadingOrderAs_NoTransformerPropagates(t *testing.T) {
	ctx := context.Background()
	ra := &Resource{Id: "a", MediaType: mediatype.JPEG, ResolvedHref: "a.jpg", Open: func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader([]byte{1, 2})), nil
	}}
	rs := NewResourceSet([]*Resource{ra}, []ReadingOrderItem{{Resource: ra, Linear: true}}, nil)
	rc, err := rs.OpenReadingOrderAs(ctx, mediatype.PlainText)
	if err != nil {
		t.Fatalf("OpenReadingOrderAs should not error immediately (lazy), got %v", err)
	}
	defer rc.Close()
	buf := make([]byte, 10)
	_, err = rc.Read(buf)
	if err == nil {
		t.Fatalf("Read should propagate ErrNoTransformer")
	}
	if !errors.Is(err, errors.Unwrap(err)) && !strings.Contains(err.Error(), "no transformer") {
		// fallback check
		if !strings.Contains(err.Error(), "no transformer") {
			t.Fatalf("should be no transformer, got %v", err)
		}
	}
}

func TestResourceSet_OpenReadingOrderAs_WithRegistry(t *testing.T) {
	ctx := context.Background()
	reg := converter.NewRegistry(&upperTransformer{from: mediatype.XHTML, to: mediatype.PlainText})
	ra := &Resource{Id: "a", MediaType: mediatype.XHTML, ResolvedHref: "a.xhtml", Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("hello")), nil
	}}
	rs := NewResourceSet([]*Resource{ra}, []ReadingOrderItem{{Resource: ra, Linear: true}}, nil)
	rc, err := rs.OpenReadingOrderAsWithRegistry(ctx, reg, mediatype.PlainText)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	b, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(b) != "HELLO" {
		t.Fatalf("custom registry upper got %q, want %q", string(b), "HELLO")
	}
}

func TestResourceSet_OpenReadingOrderAs_NilRegistryUsesDefault(t *testing.T) {
	ctx := context.Background()
	ra := &Resource{Id: "a", MediaType: mediatype.XHTML, ResolvedHref: "a.xhtml", Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("<p>hi</p>")), nil
	}}
	rs := NewResourceSet([]*Resource{ra}, []ReadingOrderItem{{Resource: ra, Linear: true}}, nil)
	rc, err := rs.OpenReadingOrderAsWithRegistry(ctx, nil, mediatype.PlainText)
	if err != nil {
		t.Fatalf("nil registry should fallback, got %v", err)
	}
	b, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(b) != "hi" {
		t.Fatalf("got %q, want %q", string(b), "hi")
	}
}

func TestMultiReadCloser_ReadAfterClose(t *testing.T) {
	ctx := context.Background()
	ra := &Resource{Id: "a", MediaType: mediatype.XHTML, ResolvedHref: "a.xhtml", Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("<p>A</p>")), nil
	}}
	rs := NewResourceSet([]*Resource{ra}, []ReadingOrderItem{{Resource: ra, Linear: true}}, nil)
	rc, _ := rs.OpenReadingOrderAs(ctx, mediatype.PlainText)
	_ = rc.Close()
	buf := make([]byte, 10)
	_, err := rc.Read(buf)
	if err == nil {
		t.Fatalf("Read after Close should error")
	}
}
