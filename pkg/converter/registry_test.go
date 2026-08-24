package converter

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/pkg/mediatype"
)

// dummyTransformer is a simple test transformer for registry tests.
type dummyTransformer struct {
	from string
	to   string
	fn   func(io.Reader) (io.Reader, error)
}

func (d *dummyTransformer) From() string { return d.from }
func (d *dummyTransformer) To() string   { return d.to }
func (d *dummyTransformer) Transform(_ context.Context, r io.Reader) (io.ReadCloser, error) {
	if d.fn != nil {
		nr, err := d.fn(r)
		if err != nil {
			return nil, err
		}
		if c, ok := r.(io.Closer); ok {
			// wrap to propagate close
			return &readCloserWrapper{Reader: nr, Closer: c}, nil
		}
		return io.NopCloser(nr), nil
	}
	b, _ := io.ReadAll(r)
	if c, ok := r.(io.Closer); ok {
		_ = c.Close()
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

type readCloserWrapper struct {
	io.Reader
	io.Closer
}

func TestRegistry_RegisterAndFind(t *testing.T) {
	reg := NewRegistry()
	tr := &dummyTransformer{from: mediatype.XHTML, to: mediatype.PlainText}
	reg.Register(tr)

	found, ok := reg.Find(mediatype.XHTML, mediatype.PlainText)
	if !ok || found != tr {
		t.Fatalf("Find should return registered transformer")
	}

	// Normalized lookup with charset and case
	found, ok = reg.Find("APPLICATION/XHTML+XML; charset=utf-8", "TEXT/PLAIN ; charset=utf-8")
	if !ok || found != tr {
		t.Fatalf("Find should normalize MIME types")
	}

	if _, ok := reg.Find(mediatype.HTML, mediatype.PlainText); ok {
		t.Fatalf("Find should not return unregistered edge")
	}
}

func TestRegistry_Register_NilAndEmpty(t *testing.T) {
	reg := NewRegistry()
	reg.Register(nil)
	if _, ok := reg.Find(mediatype.XHTML, mediatype.PlainText); ok {
		t.Fatalf("nil transformer should not be registered")
	}
	reg.Register(&dummyTransformer{from: "", to: mediatype.PlainText})
	reg.Register(&dummyTransformer{from: mediatype.XHTML, to: ""})
	if _, ok := reg.Find(mediatype.XHTML, mediatype.PlainText); ok {
		t.Fatalf("empty From/To should not be registered")
	}
}

func TestRegistry_Register_Overwrite(t *testing.T) {
	reg := NewRegistry()
	tr1 := &dummyTransformer{from: mediatype.XHTML, to: mediatype.PlainText}
	tr2 := &dummyTransformer{from: mediatype.XHTML, to: mediatype.PlainText}
	reg.Register(tr1)
	reg.Register(tr2)
	found, ok := reg.Find(mediatype.XHTML, mediatype.PlainText)
	if !ok || found != tr2 {
		t.Fatalf("second Register should overwrite first")
	}
}

func TestRegistry_FindPath_Identity(t *testing.T) {
	reg := NewRegistry(&dummyTransformer{from: mediatype.XHTML, to: mediatype.PlainText})
	path, ok := reg.FindPath(mediatype.XHTML, mediatype.XHTML)
	if !ok {
		t.Fatalf("FindPath identity should return ok=true")
	}
	if len(path) != 0 {
		t.Fatalf("identity path should be empty, got %d", len(path))
	}
	// normalized identity
	path, ok = reg.FindPath("TEXT/PLAIN; charset=utf-8", "text/plain")
	if !ok || len(path) != 0 {
		t.Fatalf("normalized identity should be empty")
	}
}

func TestRegistry_FindPath_Direct(t *testing.T) {
	reg := NewRegistry(&dummyTransformer{from: mediatype.XHTML, to: mediatype.PlainText})
	path, ok := reg.FindPath(mediatype.XHTML, mediatype.PlainText)
	if !ok || len(path) != 1 {
		t.Fatalf("direct path should have 1 transformer, ok=%v len=%d", ok, len(path))
	}
}

func TestRegistry_FindPath_MultiHop(t *testing.T) {
	reg := NewRegistry(
		&dummyTransformer{from: mediatype.XHTML, to: mediatype.HTML},
		&dummyTransformer{from: mediatype.HTML, to: mediatype.PlainText},
	)
	path, ok := reg.FindPath(mediatype.XHTML, mediatype.PlainText)
	if !ok || len(path) != 2 {
		t.Fatalf("multi-hop should be 2, ok=%v len=%d", ok, len(path))
	}
	if path[0].From() != mediatype.XHTML || path[1].To() != mediatype.PlainText {
		t.Fatalf("unexpected path order")
	}
}

func TestRegistry_FindPath_ShortestPath(t *testing.T) {
	// diamond: a->b, a->c, b->d, c->d, plus a->d direct (should pick direct)
	a, b, c, d := "type/a", "type/b", "type/c", "type/d"
	reg := NewRegistry(
		&dummyTransformer{from: a, to: b},
		&dummyTransformer{from: a, to: c},
		&dummyTransformer{from: b, to: d},
		&dummyTransformer{from: c, to: d},
	)
	path, ok := reg.FindPath(a, d)
	if !ok || len(path) != 2 {
		t.Fatalf("diamond without direct should be 2, got %d", len(path))
	}
	// add direct edge
	reg.Register(&dummyTransformer{from: a, to: d})
	path, ok = reg.FindPath(a, d)
	if !ok || len(path) != 1 {
		t.Fatalf("direct should be shortest (1), got %d", len(path))
	}
	if path[0].From() != a || path[0].To() != d {
		t.Fatalf("direct edge mismatch")
	}
}

func TestRegistry_FindPath_Cycle(t *testing.T) {
	reg := NewRegistry(
		&dummyTransformer{from: "type/a", to: "type/b"},
		&dummyTransformer{from: "type/b", to: "type/a"},
		&dummyTransformer{from: "type/b", to: "type/c"},
	)
	path, ok := reg.FindPath("type/a", "type/c")
	if !ok || len(path) != 2 {
		t.Fatalf("cycle graph should still find a->b->c, ok=%v len=%d", ok, len(path))
	}
	// no infinite loop
	if _, ok := reg.FindPath("type/a", "type/unknown"); ok {
		t.Fatalf("unknown target should not be found")
	}
}

func TestRegistry_FindPath_EmptyInput(t *testing.T) {
	reg := NewRegistry(&dummyTransformer{from: mediatype.XHTML, to: mediatype.PlainText})
	if _, ok := reg.FindPath("", mediatype.PlainText); ok {
		t.Fatalf("empty from should not find path")
	}
	if _, ok := reg.FindPath(mediatype.XHTML, ""); ok {
		t.Fatalf("empty to should not find path")
	}
}

func TestRegistry_FindPath_Unknown(t *testing.T) {
	reg := NewRegistry(&dummyTransformer{from: mediatype.XHTML, to: mediatype.PlainText})
	if _, ok := reg.FindPath(mediatype.JPEG, mediatype.PlainText); ok {
		t.Fatalf("unknown from should not find path")
	}
}

func TestRegistry_NewRegistry(t *testing.T) {
	tr := &dummyTransformer{from: mediatype.XHTML, to: mediatype.PlainText}
	reg := NewRegistry(tr)
	if _, ok := reg.Find(mediatype.XHTML, mediatype.PlainText); !ok {
		t.Fatalf("NewRegistry should pre-populate")
	}
}

func TestRegisterAliases(t *testing.T) {
	reg := NewRegistry()
	called := 0
	factory := func(from, to string) Transformer {
		called++
		return &dummyTransformer{from: from, to: to}
	}
	RegisterAliases(reg, factory, mediatype.PlainText, mediatype.XHTML, mediatype.HTML, mediatype.MobiHTML)
	if called != 3 {
		t.Fatalf("factory should be called 3 times, got %d", called)
	}
	for _, from := range []string{mediatype.XHTML, mediatype.HTML, mediatype.MobiHTML} {
		if _, ok := reg.Find(from, mediatype.PlainText); !ok {
			t.Fatalf("alias %s -> %s not registered", from, mediatype.PlainText)
		}
	}
}

func TestRegisterAliases_Nil(t *testing.T) {
	// should not panic
	RegisterAliases(nil, nil, mediatype.PlainText, mediatype.XHTML)
	RegisterAliases(NewRegistry(), nil, mediatype.PlainText, mediatype.XHTML)
	factory := func(from, to string) Transformer { return &dummyTransformer{from: from, to: to} }
	RegisterAliases(nil, factory, mediatype.PlainText, mediatype.XHTML)
}

func TestDefaultRegistry(t *testing.T) {
	for _, from := range []string{mediatype.XHTML, mediatype.HTML, mediatype.MobiHTML} {
		if _, ok := DefaultRegistry.Find(from, mediatype.PlainText); !ok {
			t.Fatalf("DefaultRegistry should have %s -> %s", from, mediatype.PlainText)
		}
	}
	if _, ok := DefaultRegistry.Find(mediatype.PlainText, mediatype.XHTML); ok {
		t.Fatalf("DefaultRegistry should not have reverse edge")
	}
	// charset normalized lookup
	if _, ok := DefaultRegistry.Find("application/xhtml+xml; charset=utf-8", "text/plain"); !ok {
		t.Fatalf("DefaultRegistry should normalize MIME")
	}
}

func TestRegistry_FindPath_NormalizedMultiHop(t *testing.T) {
	reg := NewRegistry(
		&dummyTransformer{from: mediatype.XHTML, to: mediatype.HTML},
		&dummyTransformer{from: mediatype.HTML, to: mediatype.PlainText},
	)
	path, ok := reg.FindPath("APPLICATION/XHTML+XML; charset=utf-8", "TEXT/PLAIN")
	if !ok || len(path) != 2 {
		t.Fatalf("normalized multi-hop should be 2, ok=%v len=%d", ok, len(path))
	}
}
