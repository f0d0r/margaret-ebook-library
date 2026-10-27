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

func testScanResource(id, mediaType, content string) *book.Resource {
	return &book.Resource{
		Id:           id,
		MediaType:    mediaType,
		ResolvedHref: id,
		Open: func() (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(content)), nil
		},
	}
}

func testScanBook(all []*book.Resource, order []book.ReadingOrderItem) *fakeBook {
	return &fakeBook{rs: book.NewResourceSet(all, order, nil)}
}

func TestFindBookISBNsLinearHead(t *testing.T) {
	r := testScanResource("c1", mediatype.XHTML, `<p>ISBN 978-0-306-40615-7</p>`)
	b := testScanBook([]*book.Resource{r}, []book.ReadingOrderItem{{Resource: r, Linear: true}})

	got, err := FindBookISBNs(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"9780306406157"}; !slices.Equal(canonicalStrings(got), want) {
		t.Fatalf("FindBookISBNs() = %v, want %v", canonicalStrings(got), want)
	}
}

func TestFindBookISBNsNonLinear(t *testing.T) {
	linear := testScanResource("c1", mediatype.XHTML, `<p>No identifier here.</p>`)
	notes := testScanResource("c2", mediatype.XHTML, `<p>ISBN 978-0-306-40615-7</p>`)
	b := testScanBook(
		[]*book.Resource{linear, notes},
		[]book.ReadingOrderItem{{Resource: linear, Linear: true}, {Resource: notes, Linear: false}},
	)

	got, err := FindBookISBNs(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"9780306406157"}; !slices.Equal(canonicalStrings(got), want) {
		t.Fatalf("FindBookISBNs() = %v, want %v", canonicalStrings(got), want)
	}
}

func TestFindBookISBNsManifestOnly(t *testing.T) {
	r := testScanResource("c1", mediatype.XHTML, `<p>ISBN 978-0-306-40615-7</p>`)
	b := testScanBook([]*book.Resource{r}, nil)

	got, err := FindBookISBNs(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"9780306406157"}; !slices.Equal(canonicalStrings(got), want) {
		t.Fatalf("FindBookISBNs() = %v, want %v", canonicalStrings(got), want)
	}
}

func TestFindBookISBNsWindow(t *testing.T) {
	filler := strings.Repeat("filler ", 10_000)
	first := testScanResource("c1", mediatype.XHTML, `<p>`+filler+`</p>`)
	second := testScanResource("c2", mediatype.XHTML, `<p>ISBN 978-0-306-40615-7</p>`)
	order := []book.ReadingOrderItem{{Resource: first, Linear: true}, {Resource: second, Linear: true}}
	b := testScanBook([]*book.Resource{first, second}, order)

	got, err := FindBookISBNs(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("FindBookISBNs() = %v, want none", canonicalStrings(got))
	}

	got, err = FindBookISBNs(context.Background(), b, WithISBNScanBytes(100*1024))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"9780306406157"}; !slices.Equal(canonicalStrings(got), want) {
		t.Fatalf("FindBookISBNs() = %v, want %v", canonicalStrings(got), want)
	}
}

func TestFindBookISBNsLimitStopsEarly(t *testing.T) {
	linear := testScanResource("c1", mediatype.XHTML, `<p>No identifier here.</p>`)
	first := testScanResource("c2", mediatype.XHTML, `<p>ISBN 978-0-306-40615-7</p>`)
	second := testScanResource("c3", mediatype.XHTML, `<p>ISBN 9780375704024</p>`)
	secondOpened := false
	second.Open = func() (io.ReadCloser, error) {
		secondOpened = true
		return io.NopCloser(strings.NewReader(`<p>ISBN 9780375704024</p>`)), nil
	}
	b := testScanBook(
		[]*book.Resource{linear, first, second},
		[]book.ReadingOrderItem{
			{Resource: linear, Linear: true},
			{Resource: first, Linear: false},
			{Resource: second, Linear: false},
		},
	)

	got, err := FindBookISBNs(context.Background(), b, WithISBNLimit(1))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"9780306406157"}; !slices.Equal(canonicalStrings(got), want) {
		t.Fatalf("FindBookISBNs() = %v, want %v", canonicalStrings(got), want)
	}
	if secondOpened {
		t.Fatal("FindBookISBNs opened a resource after reaching the limit")
	}
}

func TestFindBookISBNsSkipsNonText(t *testing.T) {
	linear := testScanResource("c1", mediatype.XHTML, `<p>No identifier here.</p>`)
	image := testScanResource("c2", mediatype.JPEG, `ISBN 978-0-306-40615-7`)
	b := testScanBook(
		[]*book.Resource{linear, image},
		[]book.ReadingOrderItem{{Resource: linear, Linear: true}, {Resource: image, Linear: false}},
	)

	got, err := FindBookISBNs(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("FindBookISBNs() = %v, want none", canonicalStrings(got))
	}
}

func TestFindBookISBNsDeduplicates(t *testing.T) {
	linear := testScanResource("c1", mediatype.XHTML, `<p>No identifier here.</p>`)
	first := testScanResource("c2", mediatype.XHTML, `<p>ISBN 0-306-40615-2</p>`)
	second := testScanResource("c3", mediatype.XHTML, `<p>ISBN 978-0-306-40615-7</p>`)
	b := testScanBook(
		[]*book.Resource{linear, first, second},
		[]book.ReadingOrderItem{
			{Resource: linear, Linear: true},
			{Resource: first, Linear: false},
			{Resource: second, Linear: false},
		},
	)

	got, err := FindBookISBNs(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"9780306406157"}; !slices.Equal(canonicalStrings(got), want) {
		t.Fatalf("FindBookISBNs() = %v, want %v", canonicalStrings(got), want)
	}
}

func TestFindBookISBNsContextCanceled(t *testing.T) {
	r := testScanResource("c1", mediatype.XHTML, `<p>ISBN 978-0-306-40615-7</p>`)
	b := testScanBook([]*book.Resource{r}, []book.ReadingOrderItem{{Resource: r, Linear: true}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := FindBookISBNs(ctx, b); !errors.Is(err, context.Canceled) {
		t.Fatalf("FindBookISBNs() error = %v, want context.Canceled", err)
	}
}

func TestFindBookISBNsNilBook(t *testing.T) {
	if _, err := FindBookISBNs(context.Background(), nil); err == nil {
		t.Fatal("FindBookISBNs(nil) succeeded, want an error")
	}
}

func TestFindBookISBNsEmptyBook(t *testing.T) {
	b := &fakeBook{rs: book.NewResourceSet(nil, nil, nil)}
	got, err := FindBookISBNs(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("FindBookISBNs() = %v, want none", canonicalStrings(got))
	}
}
