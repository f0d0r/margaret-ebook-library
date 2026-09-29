package epub

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/converter"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

// kepubChapter is a realistic kepubify-style content document: body wrapped
// in div#book-columns/div#book-inner, every sentence fragment wrapped in a
// koboSpan with per-paragraph ids, mixed with inline markup. Kept compact
// (no inter-tag whitespace) so the exact-output test isolates koboSpan
// boundary behaviour from source-formatting whitespace.
const kepubChapter = `<?xml version="1.0" encoding="utf-8"?><html xmlns="http://www.w3.org/1999/xhtml"><head><title>Chapter 1</title></head><body><div id="book-columns"><div id="book-inner"><p><span class="koboSpan" id="kobo.1.1">Hello </span><span class="koboSpan" id="kobo.1.2">brave <em>new</em> </span><span class="koboSpan" id="kobo.1.3">world.</span></p><p><span class="koboSpan" id="kobo.2.1">Second paragraph here.</span></p></div></div></body></html>`

func kepubOPF(coverForm string) string {
	coverManifest := ""
	coverMeta := ""
	switch coverForm {
	case "property":
		coverManifest = `<item id="cover" media-type="image/jpeg" href="cover.jpg" properties="cover-image" />`
	case "meta":
		coverManifest = `<item id="cover" media-type="image/jpeg" href="cover.jpg" />`
		coverMeta = `<meta name="cover" content="cover" />`
	}
	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Kepub Test Book</dc:title>
    <dc:creator opf:role="aut" xmlns:opf="http://www.idpf.org/2007/opf">Kepub Author</dc:creator>
    <dc:description>A book with Kobo span markup.</dc:description>
    <dc:language>en</dc:language>
    ` + coverMeta + `
  </metadata>
  <manifest>
    <item id="ch1" media-type="application/xhtml+xml" href="ch1.xhtml" />
    ` + coverManifest + `
  </manifest>
  <spine>
    <itemref idref="ch1" />
  </spine>
</package>`
}

func createKepub(t *testing.T, path, coverForm string, extra []testFile) {
	t.Helper()
	createKepubWithChapter(t, path, coverForm, kepubChapter, extra)
}

func createKepubWithChapter(t *testing.T, path, coverForm, chapter string, extra []testFile) {
	t.Helper()
	files := []testFile{
		{name: "mimetype", content: "application/epub+zip", method: zip.Store},
		{name: "META-INF/container.xml", content: createContainerXML("OEBPS/content.opf"), method: zip.Deflate},
		{name: "OEBPS/content.opf", content: kepubOPF(coverForm), method: zip.Deflate},
		{name: "OEBPS/ch1.xhtml", content: chapter, method: zip.Deflate},
		{name: "OEBPS/cover.jpg", content: string(createTestImageData()), method: zip.Deflate},
	}
	files = append(files, extra...)
	createTestZIP(t, path, files)
}

func TestKepubSupports(t *testing.T) {
	tmpDir := t.TempDir()
	kepubPath := filepath.Join(tmpDir, "book.kepub.epub")
	createKepub(t, kepubPath, "property", nil)

	reader := NewEpubReader(config.DefaultConfig())
	if !reader.Supports(book.NewPathBlob(kepubPath)) {
		t.Error("Supports() = false for KEPUB archive, want true (KEPUB is a valid EPUB)")
	}
}

func TestKepubReadsAsEPUB(t *testing.T) {
	tmpDir := t.TempDir()
	kepubPath := filepath.Join(tmpDir, "book.kepub.epub")
	createKepub(t, kepubPath, "property", nil)

	ebook, err := NewEpubReader(config.DefaultConfig()).Read(book.NewPathBlob(kepubPath))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if ebook.FileType() != book.EPUB {
		t.Errorf("FileType = %q, want %q (KEPUB reads as EPUB, option A)", ebook.FileType(), book.EPUB)
	}
	if ebook.Version() != "3.0" {
		t.Errorf("Version = %q, want %q", ebook.Version(), "3.0")
	}
	meta := ebook.Metadata()
	if meta.Title != "Kepub Test Book" {
		t.Errorf("Title = %q, want %q", meta.Title, "Kepub Test Book")
	}
	if len(meta.Authors) != 1 || meta.Authors[0] != "Kepub Author" {
		t.Errorf("Authors = %q, want [Kepub Author]", meta.Authors)
	}
	if meta.Description != "A book with Kobo span markup." {
		t.Errorf("Description = %q", meta.Description)
	}
	if len(meta.Languages) != 1 || meta.Languages[0] != "en" {
		t.Errorf("Languages = %q, want [en]", meta.Languages)
	}

	cover, ok := ebook.Resources().CoverImage()
	if !ok || cover == nil {
		t.Fatal("CoverImage() = nil, want cover via properties=\"cover-image\"")
	}
	if cover.Id != "cover" {
		t.Errorf("cover Id = %q, want %q", cover.Id, "cover")
	}

	ro := ebook.Resources().ReadingOrder()
	if len(ro) != 1 {
		t.Fatalf("len(ReadingOrder) = %d, want 1", len(ro))
	}
	if ro[0].Resource.Id != "ch1" {
		t.Errorf("ReadingOrder[0].Id = %q, want ch1", ro[0].Resource.Id)
	}
	if len(ebook.Resources().All()) != 2 {
		t.Errorf("len(All) = %d, want 2 (ch1 + cover)", len(ebook.Resources().All()))
	}
}

func TestKepubLegacyMetaCover(t *testing.T) {
	tmpDir := t.TempDir()
	kepubPath := filepath.Join(tmpDir, "legacy.kepub.epub")
	createKepub(t, kepubPath, "meta", nil)

	ebook, err := NewEpubReader(config.DefaultConfig()).Read(book.NewPathBlob(kepubPath))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	cover, ok := ebook.Resources().CoverImage()
	if !ok || cover == nil {
		t.Fatal("CoverImage() = nil, want cover via legacy <meta name=\"cover\">")
	}
	if cover.Id != "cover" {
		t.Errorf("cover Id = %q, want %q", cover.Id, "cover")
	}
}

func TestKepubMatchesPlainEPUBStructure(t *testing.T) {
	tmpDir := t.TempDir()
	kepubPath := filepath.Join(tmpDir, "book.kepub.epub")
	createKepub(t, kepubPath, "property", nil)

	plainPath := filepath.Join(tmpDir, "plain.epub")
	plainChapter := `<html><body><p>Hello brave new world.</p><p>Second paragraph here.</p></body></html>`
	plainOPF := `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Kepub Test Book</dc:title>
  </metadata>
  <manifest>
    <item id="ch1" media-type="application/xhtml+xml" href="ch1.xhtml" />
    <item id="cover" media-type="image/jpeg" href="cover.jpg" properties="cover-image" />
  </manifest>
  <spine><itemref idref="ch1" /></spine>
</package>`
	createTestZIP(t, plainPath, []testFile{
		{name: "mimetype", content: "application/epub+zip", method: zip.Store},
		{name: "META-INF/container.xml", content: createContainerXML("OEBPS/content.opf"), method: zip.Deflate},
		{name: "OEBPS/content.opf", content: plainOPF, method: zip.Deflate},
		{name: "OEBPS/ch1.xhtml", content: plainChapter, method: zip.Deflate},
		{name: "OEBPS/cover.jpg", content: string(createTestImageData()), method: zip.Deflate},
	})

	reader := NewEpubReader(config.DefaultConfig())
	kepub, err := reader.Read(book.NewPathBlob(kepubPath))
	if err != nil {
		t.Fatalf("Read(kepub) error: %v", err)
	}
	plain, err := reader.Read(book.NewPathBlob(plainPath))
	if err != nil {
		t.Fatalf("Read(plain) error: %v", err)
	}

	kRO := kepub.Resources().ReadingOrder()
	pRO := plain.Resources().ReadingOrder()
	if len(kRO) != len(pRO) {
		t.Fatalf("reading order len: kepub=%d plain=%d", len(kRO), len(pRO))
	}
	for i := range kRO {
		if kRO[i].Resource.Id != pRO[i].Resource.Id ||
			kRO[i].Resource.ResolvedHref != pRO[i].Resource.ResolvedHref ||
			kRO[i].Resource.MediaType != pRO[i].Resource.MediaType ||
			kRO[i].Linear != pRO[i].Linear {
			t.Errorf("reading order [%d] differs: kepub=%+v plain=%+v", i, kRO[i], pRO[i])
		}
	}
	if len(kepub.Resources().All()) != len(plain.Resources().All()) {
		t.Errorf("manifest len: kepub=%d plain=%d", len(kepub.Resources().All()), len(plain.Resources().All()))
	}
}

// A .kepub.epub blob must never be misclassified as FB2/MOBI: content
// sniffing sees a valid EPUB container, so FileType stays EPUB.
func TestKepubNotMisclassified(t *testing.T) {
	tmpDir := t.TempDir()
	kepubPath := filepath.Join(tmpDir, "book.kepub.epub")
	createKepub(t, kepubPath, "property", nil)

	ebook, err := NewEpubReader(config.DefaultConfig()).Read(book.NewPathBlob(kepubPath))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if ebook.FileType() != book.EPUB {
		t.Errorf("FileType = %q, want EPUB", ebook.FileType())
	}
	if ebook.FileType() == book.FB2 {
		t.Errorf("KEPUB misclassified as FB2")
	}
}

func TestKepubTextExtractionExact(t *testing.T) {
	tmpDir := t.TempDir()
	kepubPath := filepath.Join(tmpDir, "book.kepub.epub")
	createKepub(t, kepubPath, "property", nil)

	ebook, err := NewEpubReader(config.DefaultConfig()).Read(book.NewPathBlob(kepubPath))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	ctx := context.Background()
	rc, err := ebook.Resources().OpenReadingOrderAs(ctx, mediatype.PlainText)
	if err != nil {
		t.Fatalf("OpenReadingOrderAs() error: %v", err)
	}
	data, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	// Fragment-internal spaces are preserved, no artefacts are added at
	// koboSpan boundaries and no inline text is dropped; paragraphs
	// concatenate like plain EPUB (the transformer adds no block
	// separators, and head <title> text is included as with plain EPUB).
	const want = "Chapter 1Hello brave new world.Second paragraph here."
	if string(data) != want {
		t.Errorf("plain text = %q, want %q", string(data), want)
	}
}

func TestKepubMidWordSplit(t *testing.T) {
	// A word split across two koboSpans must not gain whitespace.
	input := `<p><span class="koboSpan" id="kobo.1.1">un</span><span class="koboSpan" id="kobo.1.2">believable</span></p>`
	tr, ok := converter.DefaultRegistry.Find(mediatype.XHTML, mediatype.PlainText)
	if !ok {
		t.Fatal("no transformer for XHTML -> plain text")
	}
	rc, err := tr.Transform(context.Background(), strings.NewReader(input))
	if err != nil {
		t.Fatalf("Transform() error: %v", err)
	}
	data, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	if string(data) != "unbelievable" {
		t.Errorf("mid-word split = %q, want %q", string(data), "unbelievable")
	}
}

func TestKepubSupportsPreservesPosition(t *testing.T) {
	tmpDir := t.TempDir()
	kepubPath := filepath.Join(tmpDir, "book.kepub.epub")
	createKepub(t, kepubPath, "property", nil)

	b := book.NewPathBlob(kepubPath)
	reader := NewEpubReader(config.DefaultConfig())
	if !reader.Supports(b) {
		t.Error("Supports() = false for KEPUB, want true")
	}
}

// Supports stays content-based and DRM-agnostic: even a DRM-locked archive
// reports support; only Read rejects it with ErrDRM.
func TestKepubDRMDoesNotAffectSupports(t *testing.T) {
	tmpDir := t.TempDir()
	drmPath := filepath.Join(tmpDir, "drm.kepub.epub")
	// Encrypted spine content: no <?xml, <html or koboSpan marker.
	createKepubWithChapter(t, drmPath, "property", "\x00\x01\x02\x03\x04\x05encrypted-binary-payload\xff\xfe",
		[]testFile{
			{name: "rights.xml", content: "<rights>locked</rights>", method: zip.Deflate},
		})

	reader := NewEpubReader(config.DefaultConfig())
	if !reader.Supports(book.NewPathBlob(drmPath)) {
		t.Error("Supports() = false for DRM KEPUB, want true (Read must reject, not Supports)")
	}
	if _, err := reader.Read(book.NewPathBlob(drmPath)); !errors.Is(err, book.ErrDRM) {
		t.Errorf("Read() error = %v, want ErrDRM", err)
	}
}
