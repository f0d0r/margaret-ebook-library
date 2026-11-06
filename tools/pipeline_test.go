package tools

import (
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

type recordingProcessor struct {
	events   []string
	head     string
	limit    int
	fallback bool
}

func (p *recordingProcessor) Process([]byte) error {
	p.events = append(p.events, "process")
	return nil
}

func (p *recordingProcessor) HeadLimit() int { return p.limit }

func (p *recordingProcessor) ProcessHead(head []byte) error {
	p.head = string(head)
	p.events = append(p.events, "head")
	return nil
}

func (p *recordingProcessor) NeedsFallback() bool { return p.fallback }

func (p *recordingProcessor) ProcessFallback(context.Context, *book.ResourceSet, map[*book.Resource]struct{}) error {
	p.events = append(p.events, "fallback")
	return nil
}

func (p *recordingProcessor) Finish(context.Context) error {
	p.events = append(p.events, "finish")
	return nil
}

func TestScanReadingOrderFeedsProcessors(t *testing.T) {
	rs := newTestResources(t, []string{"<p>First</p>", "<p>Second</p>"})
	head := NewTextHead(64)
	rec := &recordingProcessor{limit: 16, fallback: true}

	if err := ScanReadingOrder(context.Background(), rs, head, rec); err != nil {
		t.Fatal(err)
	}
	if want := "First\nSecond"; head.Text() != want {
		t.Errorf("head = %q, want %q", head.Text(), want)
	}
	if want := "First\nSecond"; string(rec.head) != want {
		t.Errorf("processor head = %q, want %q", rec.head, want)
	}
	if len(rec.events) < 4 {
		t.Fatalf("expected process, head, fallback and finish events, got %v", rec.events)
	}
	if rec.events[0] != "process" {
		t.Errorf("first event = %q, want process", rec.events[0])
	}
	headIdx := slices.Index(rec.events, "head")
	fallbackIdx := slices.Index(rec.events, "fallback")
	finishIdx := slices.Index(rec.events, "finish")
	if headIdx < 0 || fallbackIdx < 0 || finishIdx < 0 {
		t.Fatalf("missing lifecycle events: %v", rec.events)
	}
	if headIdx > fallbackIdx || fallbackIdx > finishIdx {
		t.Errorf("event order = %v, want head, fallback, finish", rec.events)
	}
}

func TestScanReadingOrderHeadLimit(t *testing.T) {
	rs := newTestResources(t, []string{"abcdefghij"})
	head := NewTextHead(4)
	if err := ScanReadingOrder(context.Background(), rs, head); err != nil {
		t.Fatal(err)
	}
	if want := "abcd"; head.Text() != want {
		t.Errorf("head = %q, want %q", head.Text(), want)
	}
}

func TestScanReadingOrderPerProcessorHeadLimit(t *testing.T) {
	rs := newTestResources(t, []string{"abcdefghij"})
	short := &recordingProcessor{limit: 4}
	long := &recordingProcessor{limit: 8}
	zero := &recordingProcessor{limit: 0}

	if err := ScanReadingOrder(context.Background(), rs, short, long, zero); err != nil {
		t.Fatal(err)
	}
	if want := "abcd"; short.head != want {
		t.Errorf("short head = %q, want %q", short.head, want)
	}
	if want := "abcdefgh"; long.head != want {
		t.Errorf("long head = %q, want %q", long.head, want)
	}
	if zero.head != "" || slices.Contains(zero.events, "head") {
		t.Errorf("zero-limit processor received head %q (events %v)", zero.head, zero.events)
	}
}

func TestScanReadingOrderHeadOnlyStopsReading(t *testing.T) {
	const size = 1 << 20
	content := strings.Repeat("a", size)
	var read int
	r := &book.Resource{
		Id:           "c1",
		MediaType:    mediatype.PlainText,
		ResolvedHref: "c1",
		Open: func() (io.ReadCloser, error) {
			return &countingReadCloser{r: strings.NewReader(content), n: &read}, nil
		},
	}
	rs := book.NewResourceSet(
		[]*book.Resource{r},
		[]book.ReadingOrderItem{{Resource: r, Linear: true}},
		nil,
	)

	head := &headOnlyRecorder{limit: 16}
	if err := ScanReadingOrder(context.Background(), rs, head); err != nil {
		t.Fatal(err)
	}
	if head.calls != 1 || head.head != content[:16] {
		t.Fatalf("head = %q (calls %d), want the first 16 bytes", head.head, head.calls)
	}
	if read >= size {
		t.Fatalf("read %d bytes, want less than the whole %d-byte stream", read, size)
	}
}

func TestScanReadingOrderRejectsUnsupportedProcessor(t *testing.T) {
	rs := newTestResources(t, []string{"content"})
	if err := ScanReadingOrder(context.Background(), rs, struct{}{}); err == nil {
		t.Fatal("expected an error for a processor without capabilities")
	}
}

func TestScanReadingOrderFinishOnlySkipsReading(t *testing.T) {
	opened := false
	r := &book.Resource{
		Id:           "c1",
		MediaType:    mediatype.XHTML,
		ResolvedHref: "c1",
		Open: func() (io.ReadCloser, error) {
			opened = true
			return io.NopCloser(strings.NewReader("<p>content</p>")), nil
		},
	}
	rs := book.NewResourceSet(
		[]*book.Resource{r},
		[]book.ReadingOrderItem{{Resource: r, Linear: true}},
		nil,
	)

	fin := &finishOnlyRecorder{}
	if err := ScanReadingOrder(context.Background(), rs, fin); err != nil {
		t.Fatal(err)
	}
	if opened {
		t.Fatal("reading order was opened for a finish-only processor")
	}
	if !fin.called {
		t.Fatal("Finish was not called")
	}
}

func TestScanReadingOrderWrapsProcessorError(t *testing.T) {
	rs := newTestResources(t, []string{"content"})
	boom := errors.New("boom")
	err := ScanReadingOrder(context.Background(), rs, &failingProcessor{err: boom})
	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want wrapped boom", err)
	}
	if !strings.Contains(err.Error(), "failingProcessor") {
		t.Fatalf("error = %v, want the processor type", err)
	}
}

func TestScanReadingOrderContextCanceled(t *testing.T) {
	rs := newTestResources(t, []string{"content"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ScanReadingOrder(ctx, rs, NewTextHead(4)); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestScanReadingOrderNilResources(t *testing.T) {
	if err := ScanReadingOrder(context.Background(), nil); err == nil {
		t.Fatal("expected an error for nil resource set")
	}
}

func TestFingerprintProcessorMatchesFingerprintContent(t *testing.T) {
	rs := newTestResources(t, []string{"<p>Hello world</p>", "<p>Second chapter content here</p>"})
	opts := []FingerprintOption{
		WithMinHash(MinHashConfig{NumHashes: 16, ShingleSize: 2}),
		WithSimHash(),
	}

	want, err := FingerprintContent(context.Background(), &fakeBook{rs: rs}, opts...)
	if err != nil {
		t.Fatal(err)
	}
	proc, err := NewFingerprintProcessor(opts...)
	if err != nil {
		t.Fatal(err)
	}
	if err := ScanReadingOrder(context.Background(), rs, proc); err != nil {
		t.Fatal(err)
	}
	got := proc.Result()
	if !slices.Equal(got.MinHash, want.MinHash) {
		t.Errorf("MinHash = %v, want %v", got.MinHash, want.MinHash)
	}
	if got.SimHash != want.SimHash || got.HasSimHash != want.HasSimHash {
		t.Errorf("SimHash = (%v, %v), want (%v, %v)", got.SimHash, got.HasSimHash, want.SimHash, want.HasSimHash)
	}
}

func TestISBNProcessorHead(t *testing.T) {
	r := testScanResource("c1", mediatype.XHTML, `<p>ISBN 978-0-306-40615-7</p>`)
	rs := book.NewResourceSet([]*book.Resource{r}, []book.ReadingOrderItem{{Resource: r, Linear: true}}, nil)

	proc := NewISBNProcessor(64*1024, 1)
	if err := ScanReadingOrder(context.Background(), rs, proc); err != nil {
		t.Fatal(err)
	}
	if want := []string{"9780306406157"}; !slices.Equal(canonicalStrings(proc.Result()), want) {
		t.Fatalf("ISBNs = %v, want %v", canonicalStrings(proc.Result()), want)
	}
}

func TestISBNProcessorSkipsPlaceholder(t *testing.T) {
	r := testScanResource("c1", mediatype.XHTML, `<p>ISBN 978-1-234567-89-7 and 978-0-306-40615-7</p>`)
	b := testScanBook([]*book.Resource{r}, []book.ReadingOrderItem{{Resource: r, Linear: true}})

	proc := NewISBNProcessor(64*1024, 1)
	if err := ScanReadingOrder(context.Background(), b.Resources(), proc); err != nil {
		t.Fatal(err)
	}
	if want := []string{"9780306406157"}; !slices.Equal(canonicalStrings(proc.Result()), want) {
		t.Fatalf("ISBNs = %v, want %v", canonicalStrings(proc.Result()), want)
	}
}

func TestISBNProcessorFallback(t *testing.T) {
	linear := testScanResource("c1", mediatype.XHTML, `<p>No identifier here.</p>`)
	notes := testScanResource("c2", mediatype.XHTML, `<p>ISBN 978-0-306-40615-7</p>`)
	rs := book.NewResourceSet(
		[]*book.Resource{linear, notes},
		[]book.ReadingOrderItem{{Resource: linear, Linear: true}, {Resource: notes, Linear: false}},
		nil,
	)

	proc := NewISBNProcessor(64*1024, 1)
	if err := ScanReadingOrder(context.Background(), rs, proc); err != nil {
		t.Fatal(err)
	}
	if want := []string{"9780306406157"}; !slices.Equal(canonicalStrings(proc.Result()), want) {
		t.Fatalf("ISBNs = %v, want %v", canonicalStrings(proc.Result()), want)
	}
}

func TestISBNProcessorHeadWinsWithoutFallback(t *testing.T) {
	linear := testScanResource("c1", mediatype.XHTML, `<p>ISBN 978-0-306-40615-7</p>`)
	notes := testScanResource("c2", mediatype.XHTML, `<p>ISBN 9780375704024</p>`)
	opened := false
	notes.Open = func() (io.ReadCloser, error) {
		opened = true
		return io.NopCloser(strings.NewReader(`<p>ISBN 9780375704024</p>`)), nil
	}
	rs := book.NewResourceSet(
		[]*book.Resource{linear, notes},
		[]book.ReadingOrderItem{{Resource: linear, Linear: true}, {Resource: notes, Linear: false}},
		nil,
	)

	proc := NewISBNProcessor(64*1024, 1)
	if err := ScanReadingOrder(context.Background(), rs, proc); err != nil {
		t.Fatal(err)
	}
	if want := []string{"9780306406157"}; !slices.Equal(canonicalStrings(proc.Result()), want) {
		t.Fatalf("ISBNs = %v, want %v", canonicalStrings(proc.Result()), want)
	}
	if opened {
		t.Fatal("non-linear resource was opened although the head had an ISBN")
	}
}

type headOnlyRecorder struct {
	limit int
	head  string
	calls int
}

func (p *headOnlyRecorder) HeadLimit() int { return p.limit }

func (p *headOnlyRecorder) ProcessHead(head []byte) error {
	p.calls++
	p.head = string(head)
	return nil
}

type countingReadCloser struct {
	r io.Reader
	n *int
}

func (c *countingReadCloser) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	*c.n += n
	return n, err
}

func (c *countingReadCloser) Close() error { return nil }

type failingProcessor struct {
	err error
}

func (p *failingProcessor) Process([]byte) error { return p.err }

type finishOnlyRecorder struct {
	called bool
}

func (p *finishOnlyRecorder) Finish(context.Context) error {
	p.called = true
	return nil
}
