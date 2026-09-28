package fb2

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

const richCoverB64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

var richJpegBytes = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01, 0x02, 0x03}

func richFb2() string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0" xmlns:l="http://www.w3.org/1999/xlink">
<description><title-info>
<author><first-name>Anna</first-name><last-name>Author</last-name></author>
<book-title>Rich Book</book-title>
<coverpage><image l:href="#cover"/></coverpage>
<lang>en</lang>
</title-info></description>
<body>
<title><p>Book Header</p></title>
<section id="s1"><title><p>First</p></title>
<p>Hello <strong>world</strong>.</p>
<image l:href="#pic1"/>
<p>See <a l:href="#s2">second</a>, <a l:href="#s1">self</a> and <a l:href="https://example.com">ext</a>.</p>
<section id="s1a"><title><p>Nested</p></title><p>Deep.</p></section>
</section>
<section id="s2"><title><p>Second</p></title>
<poem><stanza><v>Line one</v><v>Line two</v></stanza><text-author>Poet</text-author></poem>
<table><tr><th>H</th><td>C</td></tr></table>
<p>Ghost <image l:href="#ghost"/> pic.</p>
<p>Lost <a l:href="#nowhere">link</a>.</p>
</section>
</body>
<body name="notes"><section id="n1"><title><p>Notes</p></title><p>Footnote text.</p></section></body>
<binary id="cover" content-type="image/png">%s</binary>
<binary id="pic1" content-type="image/jpeg">%s</binary>
<binary id="broken" content-type="image/png">!!!not-base64!!!</binary>
<binary id="doc" content-type="application/pdf">aGVsbG8=</binary>
</FictionBook>`,
		richCoverB64,
		base64.StdEncoding.EncodeToString(richJpegBytes),
	)
}

func readRichBook(t *testing.T) book.Book {
	t.Helper()
	ebook, err := NewFb2Reader(config.DefaultConfig()).Read(book.NewBytesBlob([]byte(richFb2())))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	return ebook
}

func TestFb2ReadRichStructure(t *testing.T) {
	ebook := readRichBook(t)

	if ebook.FileType() != book.FB2 {
		t.Errorf("FileType() = %q, want fb2", ebook.FileType())
	}
	if ebook.Version() != "2.0" {
		t.Errorf("Version() = %q, want 2.0", ebook.Version())
	}
	if ebook.Metadata().Title != "Rich Book" {
		t.Errorf("Title = %q, want Rich Book", ebook.Metadata().Title)
	}

	rs := ebook.Resources()
	if rs.Len() != 4 {
		t.Fatalf("Len() = %d, want 4 (2 bodies + 2 images)", rs.Len())
	}
	ro := rs.ReadingOrder()
	if len(ro) != 2 {
		t.Fatalf("len(ReadingOrder) = %d, want 2", len(ro))
	}
	wantLinear := []bool{true, false}
	for i, item := range ro {
		if item.Linear != wantLinear[i] {
			t.Errorf("ReadingOrder[%d].Linear = %v, want %v", i, item.Linear, wantLinear[i])
		}
	}
	wantNames := []string{"body0001.xml", "body0002.xml"}
	for i, item := range ro {
		if item.Resource.Name != wantNames[i] {
			t.Errorf("ReadingOrder[%d].Name = %q, want %q", i, item.Resource.Name, wantNames[i])
		}
		if item.Resource.MediaType != mediatype.FB2Body {
			t.Errorf("ReadingOrder[%d].MediaType = %q, want %q", i, item.Resource.MediaType, mediatype.FB2Body)
		}
	}

	all := rs.All()
	if all[0].Name != "body0001.xml" || all[2].Name != "cover.png" || all[3].Name != "pic1.jpg" {
		names := make([]string, len(all))
		for i, r := range all {
			names[i] = r.Name
		}
		t.Errorf("All() names = %q, want content first then images", names)
	}
}

func TestFb2BodyRawFidelity(t *testing.T) {
	ebook := readRichBook(t)
	rs := ebook.Resources()
	src := richFb2()

	body := func(openTag string) string {
		t.Helper()
		start := strings.Index(src, openTag)
		if start < 0 {
			t.Fatalf("fixture missing %q", openTag)
		}
		start += len(openTag)
		end := strings.Index(src[start:], "</body>")
		if end < 0 {
			t.Fatalf("fixture missing closing body for %q", openTag)
		}
		return src[start : start+end]
	}
	wantBody1 := body("<body>")
	wantNotes := body(`<body name="notes">`)

	data := func(name string) []byte {
		t.Helper()
		r, ok := rs.GetByHref(name)
		if !ok {
			t.Fatalf("GetByHref(%q) not found", name)
		}
		b, err := r.Data()
		if err != nil {
			t.Fatalf("Data(%q) error: %v", name, err)
		}
		return b
	}

	// Strongest raw-principle assert: resource bytes equal the stored slice.
	if got := data("body0001.xml"); !bytes.Equal(got, []byte(wantBody1)) {
		t.Errorf("body0001.xml differs from stored bytes:\ngot  %q\nwant %q", got, wantBody1)
	}
	if got := data("body0002.xml"); !bytes.Equal(got, []byte(wantNotes)) {
		t.Errorf("body0002.xml differs from stored bytes:\ngot  %q\nwant %q", got, wantNotes)
	}

	// No metadata or binary payloads may leak into content resources.
	for name, content := range map[string][]byte{
		"body0001.xml": data("body0001.xml"),
		"body0002.xml": data("body0002.xml"),
	} {
		for _, banned := range []string{"Anna", "Rich Book", "iVBOR", "<binary", "<description"} {
			if strings.Contains(string(content), banned) {
				t.Errorf("%s contains %q from outside its body", name, banned)
			}
		}
	}

	r, _ := rs.GetByHref("body0001.xml")
	if r.Size != int64(len(wantBody1)) {
		t.Errorf("body0001 Size = %d, want %d", r.Size, len(wantBody1))
	}
}

func TestFb2PlainTextViaTransformers(t *testing.T) {
	ebook := readRichBook(t)
	rs := ebook.Resources()
	ctx := context.Background()

	r, ok := rs.GetByHref("body0001.xml")
	if !ok {
		t.Fatal("body0001.xml not found")
	}
	text, err := r.DataAs(ctx, mediatype.PlainText)
	if err != nil {
		t.Fatalf("DataAs failed: %v", err)
	}
	for _, want := range []string{"Hello", "world", "Deep.", "Book Header", "First", "second", "self", "ext"} {
		if !strings.Contains(string(text), want) {
			t.Errorf("DataAs(body0001) missing %q in %q", want, string(text))
		}
	}

	rc, err := rs.OpenReadingOrderAs(ctx, mediatype.PlainText)
	if err != nil {
		t.Fatalf("OpenReadingOrderAs failed: %v", err)
	}
	full, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	for _, want := range []string{"Hello", "Line one", "Line two", "Poet"} {
		if !strings.Contains(string(full), want) {
			t.Errorf("OpenReadingOrderAs missing %q", want)
		}
	}
	if strings.Contains(string(full), "Footnote") {
		t.Errorf("OpenReadingOrderAs should skip non-linear notes, got %q", string(full))
	}

	notes, ok := rs.GetByHref("body0002.xml")
	if !ok {
		t.Fatal("body0002.xml not found")
	}
	notesText, err := notes.DataAs(ctx, mediatype.PlainText)
	if err != nil {
		t.Fatalf("DataAs(notes) failed: %v", err)
	}
	for _, want := range []string{"Notes", "Footnote text."} {
		if !strings.Contains(string(notesText), want) {
			t.Errorf("notes DataAs missing %q in %q", want, string(notesText))
		}
	}
}

func TestFindBodyRanges(t *testing.T) {
	t.Run("two bodies", func(t *testing.T) {
		data := `<FictionBook><description/><body><p>one</p></body><body name="notes"><p>two</p></body></FictionBook>`
		ranges, err := findBodyRanges([]byte(data))
		if err != nil {
			t.Fatalf("findBodyRanges() error: %v", err)
		}
		if len(ranges) != 2 {
			t.Fatalf("len(ranges) = %d, want 2", len(ranges))
		}
		if got := string([]byte(data)[ranges[0][0]:ranges[0][1]]); got != "<p>one</p>" {
			t.Errorf("ranges[0] = %q, want %q", got, "<p>one</p>")
		}
		if got := string([]byte(data)[ranges[1][0]:ranges[1][1]]); got != "<p>two</p>" {
			t.Errorf("ranges[1] = %q, want %q", got, "<p>two</p>")
		}
	})

	t.Run("markup lookalikes ignored", func(t *testing.T) {
		data := `<FictionBook><description><!-- <body> fake --></description><body><p>a body of water</p></body></FictionBook>`
		ranges, err := findBodyRanges([]byte(data))
		if err != nil {
			t.Fatalf("findBodyRanges() error: %v", err)
		}
		if len(ranges) != 1 {
			t.Fatalf("len(ranges) = %d, want 1", len(ranges))
		}
	})

	t.Run("prefixed body", func(t *testing.T) {
		data := `<fb:FictionBook xmlns:fb="urn:x"><fb:body><fb:p>x</fb:p></fb:body></fb:FictionBook>`
		ranges, err := findBodyRanges([]byte(data))
		if err != nil {
			t.Fatalf("findBodyRanges() error: %v", err)
		}
		if len(ranges) != 1 {
			t.Fatalf("len(ranges) = %d, want 1", len(ranges))
		}
	})

	t.Run("no bodies", func(t *testing.T) {
		ranges, err := findBodyRanges([]byte(`<FictionBook><description/></FictionBook>`))
		if err != nil {
			t.Fatalf("findBodyRanges() error: %v", err)
		}
		if len(ranges) != 0 {
			t.Errorf("len(ranges) = %d, want 0", len(ranges))
		}
	})

	t.Run("unclosed body extends to EOF", func(t *testing.T) {
		data := `<FictionBook><body><p>open`
		ranges, err := findBodyRanges([]byte(data))
		if err != nil {
			t.Fatalf("findBodyRanges() error: %v", err)
		}
		if len(ranges) != 1 {
			t.Fatalf("len(ranges) = %d, want 1", len(ranges))
		}
		if got := string([]byte(data)[ranges[0][0]:ranges[0][1]]); got != "<p>open" {
			t.Errorf("ranges[0] = %q, want %q", got, "<p>open")
		}
	})

	t.Run("mismatched tags error", func(t *testing.T) {
		if _, err := findBodyRanges([]byte(`<FictionBook><body><p>x</div></body></FictionBook>`)); err == nil {
			t.Error("findBodyRanges() expected error for mismatched tags, got nil")
		}
	})
}

func TestFb2ReadAmpRetryEndToEnd(t *testing.T) {
	// Bare ampersand in the description forces the fixed-buffer path;
	// offsets, metadata and binaries must all come from that same buffer.
	doc := strings.Replace(richFb2(), "Rich Book", "R & D", 1)
	ebook, err := NewFb2Reader(config.DefaultConfig()).Read(book.NewBytesBlob([]byte(doc)))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if ebook.Metadata().Title != "R & D" {
		t.Errorf("Title = %q, want %q", ebook.Metadata().Title, "R & D")
	}
	ro := ebook.Resources().ReadingOrder()
	if len(ro) != 2 {
		t.Fatalf("len(ReadingOrder) = %d, want 2", len(ro))
	}
	r, ok := ebook.Resources().GetByHref("body0001.xml")
	if !ok {
		t.Fatal("body0001.xml not found")
	}
	data, err := r.Data()
	if err != nil {
		t.Fatalf("Data() error: %v", err)
	}
	if !strings.Contains(string(data), "Hello") {
		t.Errorf("body0001 content missing, got %q", string(data))
	}
}

func TestFb2CoverAndBinaries(t *testing.T) {
	ebook := readRichBook(t)
	rs := ebook.Resources()

	cover, ok := rs.CoverImage()
	if !ok || cover == nil {
		t.Fatal("CoverImage not found")
	}
	if cover.MediaType != mediatype.PNG {
		t.Errorf("cover MediaType = %q, want image/png", cover.MediaType)
	}
	if cover.Properties != "cover-image" {
		t.Errorf("cover Properties = %q, want cover-image", cover.Properties)
	}
	coverData, err := cover.Data()
	if err != nil {
		t.Fatalf("cover Data() error: %v", err)
	}
	if len(coverData) < 4 || string(coverData[:4]) != "\x89PNG" {
		t.Errorf("cover data missing PNG magic")
	}
	byHref, ok := rs.GetByHref("images/cover.png")
	if !ok || byHref != cover {
		t.Error("cover should be the same pointer as images/cover.png in All()")
	}

	pic, ok := rs.GetByHref("images/pic1.jpg")
	if !ok {
		t.Fatal("images/pic1.jpg not found")
	}
	if pic.MediaType != mediatype.JPEG {
		t.Errorf("pic MediaType = %q, want image/jpeg", pic.MediaType)
	}
	picData, err := pic.Data()
	if err != nil {
		t.Fatalf("pic Data() error: %v", err)
	}
	if !bytes.Equal(picData, richJpegBytes) {
		t.Errorf("pic data round-trip mismatch: got %d bytes", len(picData))
	}
	if pic.Size != int64(len(richJpegBytes)) {
		t.Errorf("pic Size = %d, want %d", pic.Size, len(richJpegBytes))
	}

	for _, href := range []string{"images/broken.png", "images/doc.pdf"} {
		if _, ok := rs.GetByHref(href); ok {
			t.Errorf("GetByHref(%q) should be absent (skipped binary)", href)
		}
	}
}

func TestFb2ReadLimits(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MaxResourceSize = 10
	reader := NewFb2Reader(cfg)

	if _, err := reader.Read(book.NewBytesBlob([]byte(richFb2()))); !errors.Is(err, book.ErrLimitExceeded) {
		t.Errorf("Read() error = %v, want ErrLimitExceeded", err)
	}

	zipped := buildZipBlob(t, []zipEntry{{"book.fb2", richFb2()}})
	if _, err := reader.Read(zipped); !errors.Is(err, book.ErrLimitExceeded) {
		t.Errorf("Read(zipped) error = %v, want ErrLimitExceeded", err)
	}
}

func TestFb2NoCoverpage(t *testing.T) {
	ebook, err := NewFb2Reader(config.DefaultConfig()).Read(book.NewBytesBlob([]byte(minimalFB2)))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if _, ok := ebook.Resources().CoverImage(); ok {
		t.Error("CoverImage should be absent without coverpage")
	}
}

func TestFb2SupportsReadConsistentFallback(t *testing.T) {
	// No .fb2 entry: Supports sniffs past the first entry, and Read must
	// open that same entry rather than the first one.
	blob := buildZipBlob(t, []zipEntry{
		{"readme.txt", "just a readme"},
		{"book.xml", minimalFB2},
	})
	reader := NewFb2Reader(config.DefaultConfig())
	if !reader.Supports(blob) {
		t.Fatal("Supports() = false, want true")
	}
	ebook, err := reader.Read(blob)
	if err != nil {
		t.Fatalf("Read() error: %v (Supports was true)", err)
	}
	if ebook.Metadata().Title != "Test" {
		t.Errorf("Title = %q, want Test", ebook.Metadata().Title)
	}
}

func TestFb2BinaryIDWithExtension(t *testing.T) {
	jpegB64 := base64.StdEncoding.EncodeToString(richJpegBytes)
	doc := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">
<description><title-info>
<author><nickname>a</nickname></author>
<book-title>T</book-title>
<coverpage><image l:href="#cover.jpg" xmlns:l="http://www.w3.org/1999/xlink"/></coverpage>
<lang>en</lang>
</title-info></description>
<body><section><p>x <image l:href="#cover.jpg" xmlns:l="http://www.w3.org/1999/xlink"/></p></section></body>
<binary id="cover.jpg" content-type="image/jpeg">%s</binary>
<binary id="PIC.JPG" content-type="image/jpeg">%s</binary>
</FictionBook>`, jpegB64, jpegB64)

	ebook, err := NewFb2Reader(config.DefaultConfig()).Read(book.NewBytesBlob([]byte(doc)))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	rs := ebook.Resources()
	if _, ok := rs.GetByHref("images/cover.jpg"); !ok {
		t.Error("images/cover.jpg not found (extension must not double)")
	}
	if _, ok := rs.GetByHref("images/cover.jpg.jpg"); ok {
		t.Error("images/cover.jpg.jpg must not exist")
	}
	if _, ok := rs.GetByHref("images/PIC.JPG"); !ok {
		t.Error("images/PIC.JPG not found (suffix check is case-insensitive)")
	}
	cover, ok := rs.CoverImage()
	if !ok || cover == nil {
		t.Fatal("CoverImage not found")
	}
	if cover.MediaType != mediatype.JPEG {
		t.Errorf("cover MediaType = %q, want image/jpeg", cover.MediaType)
	}
}

func TestSanitizeImageID(t *testing.T) {
	tests := []struct{ in, want string }{
		{"cover", "cover"},
		{"a/b\\c:d", "a_b_c_d"},
		{"", "image"},
		{"photo 1", "photo_1"},
		{"borító", "borító"},
		{"cover.jpg", "cover.jpg"},
	}
	for _, tt := range tests {
		if got := sanitizeImageID(tt.in); got != tt.want {
			t.Errorf("sanitizeImageID(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
