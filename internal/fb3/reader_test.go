package fb3

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

const (
	minimalContentTypes = `<?xml version="1.0" encoding="UTF-8"?>` +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="png" ContentType="image/png"/>` +
		`<Default Extension="jpg" ContentType="image/jpeg"/>` +
		`<Override PartName="/fb3/description.xml" ContentType="application/fb3-description+xml"/>` +
		`<Override PartName="/fb3/body.xml" ContentType="application/fb3-body+xml"/>` +
		`</Types>`

	minimalRootRels = `<?xml version="1.0" encoding="UTF-8"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="meta/core.xml"/>` +
		`<Relationship Id="rId2" Type="http://www.fictionbook.org/FictionBook3/relationships/Book" Target="fb3/description.xml"/>` +
		`<Relationship Id="rId0" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/thumbnail" Target="fb3/img/cover.png"/>` +
		`</Relationships>`

	minimalDescription = `<?xml version="1.0" encoding="UTF-8"?>` +
		`<fb3-description xmlns="http://www.fictionbook.org/FictionBook3/description" version="1.0">` +
		`<title><main>Test Book</main><sub>ignored sub</sub></title>` +
		`<fb3-relations>` +
		`<subject link="author"><title><main>John Ronald Doe</main></title><first-name>John</first-name><middle-name>Ronald</middle-name><last-name>Doe</last-name></subject>` +
		`<subject link="author"><title><main>Acme Press</main></title></subject>` +
		`<subject link="editor"><first-name>Ed</first-name><last-name>Itor</last-name></subject>` +
		`</fb3-relations>` +
		`<fb3-classification><subject>test</subject></fb3-classification>` +
		`<lang>hu</lang>` +
		`<annotation><p>First <strong>para</strong>.</p><br/><p>Second para.</p></annotation>` +
		`</fb3-description>`

	minimalDescRels = `<?xml version="1.0" encoding="UTF-8"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://www.fictionbook.org/FictionBook3/relationships/body" Target="body.xml"/>` +
		`</Relationships>`

	minimalBody = `<?xml version="1.0" encoding="UTF-8"?>` +
		`<fb3-body xmlns="http://www.fictionbook.org/FictionBook3/body" id="6079f35b-49cc-4211-adba-919ccaaff4f2">` +
		`<title><p>Test Book</p></title>` +
		`<section><p>Hello <strong>world</strong>.</p><p><img src="pic1"/>caption</p></section>` +
		`</fb3-body>`

	minimalBodyRels = `<?xml version="1.0" encoding="UTF-8"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="cover" Type="http://www.fictionbook.org/FictionBook3/relationships/image" Target="img/cover.png"/>` +
		`<Relationship Id="pic1" Type="http://www.fictionbook.org/FictionBook3/relationships/image" Target="img/pic.jpg"/>` +
		`</Relationships>`

	minimalFB2 = `<?xml version="1.0" encoding="UTF-8"?>` +
		`<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">` +
		`<description><title-info><book-title>Test</book-title></title-info></description>` +
		`<body><section><p>Hello</p></section></body></FictionBook>`
)

// minimalFb3Files returns a complete minimal FB3 package.
func minimalFb3Files() []fb3File {
	return []fb3File{
		{"[Content_Types].xml", minimalContentTypes},
		{"_rels/.rels", minimalRootRels},
		{"fb3/description.xml", minimalDescription},
		{"fb3/_rels/description.xml.rels", minimalDescRels},
		{"fb3/body.xml", minimalBody},
		{"fb3/_rels/body.xml.rels", minimalBodyRels},
		{"fb3/img/cover.png", "PNGDATA"},
		{"fb3/img/pic.jpg", "JPGDATA"},
	}
}

func TestFb3ReaderSupports(t *testing.T) {
	reader := NewFb3Reader(config.DefaultConfig())

	hardcoreFiles := []fb3File{
		{"[Content_Types].xml", strings.Replace(minimalContentTypes,
			`<Override PartName="/fb3/description.xml" ContentType="application/fb3-description+xml"/>`,
			`<Override PartName="/%D1%84/foo/meta.xml" ContentType="application/fb3-description+xml"/>`, 1)},
		{"_rels/.rels", `<?xml version="1.0" encoding="UTF-8"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId2" Type="http://www.fictionbook.org/FictionBook3/relationships/Book" Target="%D1%84/FOO/meta.xml"/>` +
			`</Relationships>`},
		{"%D1%84/foo/meta.xml", minimalDescription},
		{"%D1%84/foo/_RELS/META.XML.RELS", `<?xml version="1.0" encoding="UTF-8"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId0" Type="http://www.fictionbook.org/FictionBook3/relationships/body" Target="../../%D0%BA%D0%BD%D0%B8%D0%B3%D0%B0"/>` +
			`</Relationships>`},
		{"%D0%BA%D0%BD%D0%B8%D0%B3%D0%B0", minimalBody},
	}

	tests := []struct {
		name  string
		files []fb3File
		raw   []byte
		want  bool
	}{
		{"minimal package", minimalFb3Files(), nil, true},
		{"hardcore naming", hardcoreFiles, nil, true},
		{"no content-types no rels falls back to scan", []fb3File{
			{"fb3/description.xml", minimalDescription},
			{"fb3/body.xml", minimalBody},
		}, nil, true},
		{"macosx noise ignored", append(minimalFb3Files(),
			fb3File{"__MACOSX/._x", "noise"},
			fb3File{"__MACOSX/fb3/._body.xml", "noise"},
			fb3File{".DS_Store", "noise"},
		), nil, true},
		{"epub-like zip is not fb3", []fb3File{
			{"mimetype", "application/epub+zip"},
			{"META-INF/container.xml", `<container version="1.0"/>`},
		}, nil, false},
		{"fb2 zip is not fb3", []fb3File{
			{"book.fb2", minimalFB2},
		}, nil, false},
		{"zip of html is not fb3", []fb3File{
			{"index.html", "<html><body>hi</body></html>"},
		}, nil, false},
		{"empty zip", nil, nil, false},
		{"plain text", nil, []byte("just some text"), false},
		{"truncated zip header", nil, []byte{0x50, 0x4B, 0x03, 0x04, 0x00}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b book.Blob
			if tt.raw != nil || tt.files == nil {
				b = book.NewBytesBlob(tt.raw)
			} else {
				b = buildFb3Blob(t, tt.files)
			}
			if got := reader.Supports(b); got != tt.want {
				t.Errorf("Supports() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFb3ReaderRead(t *testing.T) {
	reader := NewFb3Reader(config.DefaultConfig())

	t.Run("full package", func(t *testing.T) {
		ebook, err := reader.Read(buildFb3Blob(t, minimalFb3Files()))
		if err != nil {
			t.Fatalf("Read() error: %v", err)
		}
		if ebook.FileType() != book.FB {
			t.Errorf("FileType() = %q, want %q", ebook.FileType(), book.FB)
		}
		if ebook.Version() != "3.0" {
			t.Errorf("Version() = %q, want %q", ebook.Version(), "3.0")
		}
		md := ebook.Metadata()
		if md.Title != "Test Book" {
			t.Errorf("Title = %q, want %q", md.Title, "Test Book")
		}
		if len(md.Authors) != 2 || md.Authors[0] != "John Ronald Doe" || md.Authors[1] != "Acme Press" {
			t.Errorf("Authors = %q, want [John Ronald Doe Acme Press]", md.Authors)
		}
		if md.Description != "First para.\n\nSecond para." {
			t.Errorf("Description = %q", md.Description)
		}
		if len(md.Languages) != 1 || md.Languages[0] != "hu" {
			t.Errorf("Languages = %q, want [hu]", md.Languages)
		}

		rs := ebook.Resources()
		if rs.Len() != 3 {
			t.Fatalf("Len() = %d, want 3 (body + 2 images)", rs.Len())
		}
		ro := rs.ReadingOrder()
		if len(ro) != 1 || !ro[0].Linear || ro[0].Resource.MediaType != mediatype.FB3Body {
			t.Fatalf("reading order = %+v, want single linear fb3-body", ro)
		}
		cover, ok := rs.CoverImage()
		if !ok {
			t.Fatalf("CoverImage() missing, want cover.png")
		}
		if cover.Properties != "cover-image" {
			t.Errorf("cover properties = %q, want cover-image", cover.Properties)
		}
		if !strings.HasSuffix(cover.Href, "img/cover.png") {
			t.Errorf("cover href = %q", cover.Href)
		}

		pic, ok := rs.GetByID("pic1")
		if !ok {
			t.Fatalf("GetByID(pic1) missing")
		}
		if pic.MediaType != mediatype.JPEG {
			t.Errorf("pic type = %q, want jpeg", pic.MediaType)
		}
		rc, err := pic.Open()
		if err != nil {
			t.Fatalf("pic Open(): %v", err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("pic read: %v", err)
		}
		if string(data) != "JPGDATA" || int64(len(data)) != pic.Size {
			t.Errorf("pic data = %q size %d, declared %d", data, len(data), pic.Size)
		}

		text, err := ro[0].Resource.DataAs(context.Background(), mediatype.PlainText)
		if err != nil {
			t.Fatalf("body DataAs: %v", err)
		}
		if string(text) != "Test Book\n\nHello world.\ncaption" {
			t.Errorf("body text = %q", text)
		}
	})

	t.Run("missing body errors", func(t *testing.T) {
		files := []fb3File{
			{"[Content_Types].xml", minimalContentTypes},
			{"_rels/.rels", minimalRootRels},
			{"fb3/description.xml", minimalDescription},
			{"fb3/_rels/description.xml.rels", strings.Replace(minimalDescRels, "body.xml", "gone.xml", 1)},
		}
		if _, err := reader.Read(buildFb3Blob(t, files)); err == nil {
			t.Error("Read() expected error for missing body, got nil")
		}
	})

	t.Run("missing description errors", func(t *testing.T) {
		files := []fb3File{
			{"fb3/body.xml", minimalBody},
		}
		if _, err := reader.Read(buildFb3Blob(t, files)); err == nil {
			t.Error("Read() expected error for missing description, got nil")
		}
	})

	t.Run("unresolvable thumbnail means no cover", func(t *testing.T) {
		files := minimalFb3Files()
		files[1] = fb3File{"_rels/.rels", strings.Replace(minimalRootRels, "fb3/img/cover.png", "img/gone.png", 1)}
		ebook, err := reader.Read(buildFb3Blob(t, files))
		if err != nil {
			t.Fatalf("Read() error: %v", err)
		}
		if _, ok := ebook.Resources().CoverImage(); ok {
			t.Error("CoverImage() present, want none")
		}
	})

	t.Run("missing image target skipped", func(t *testing.T) {
		files := minimalFb3Files()
		files[5] = fb3File{"fb3/_rels/body.xml.rels", strings.Replace(minimalBodyRels, "img/pic.jpg", "img/gone.jpg", 1)}
		ebook, err := reader.Read(buildFb3Blob(t, files))
		if err != nil {
			t.Fatalf("Read() error: %v", err)
		}
		if got := ebook.Resources().Len(); got != 2 {
			t.Errorf("Len() = %d, want 2 (body + cover)", got)
		}
	})

	t.Run("tiny limit errors", func(t *testing.T) {
		small := NewFb3Reader(config.Config{MaxResourceSize: 10})
		if _, err := small.Read(buildFb3Blob(t, minimalFb3Files())); err == nil {
			t.Error("Read() expected limit error, got nil")
		}
	})

	t.Run("large body rels resolve fully", func(t *testing.T) {
		const n = 300
		var rels strings.Builder
		rels.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
		files := []fb3File{
			{"[Content_Types].xml", minimalContentTypes},
			{"_rels/.rels", strings.Replace(minimalRootRels,
				`<Relationship Id="rId0" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/thumbnail" Target="fb3/img/cover.png"/>`, "", 1)},
			{"fb3/description.xml", minimalDescription},
			{"fb3/_rels/description.xml.rels", minimalDescRels},
			{"fb3/body.xml", minimalBody},
		}
		for i := 0; i < n; i++ {
			id := "img" + strconv.Itoa(i)
			rels.WriteString(`<Relationship Id="` + id + `" Type="http://www.fictionbook.org/FictionBook3/relationships/image" Target="img/` + id + `.jpg"/>`)
			files = append(files, fb3File{"fb3/img/" + id + ".jpg", "x"})
		}
		rels.WriteString(`</Relationships>`)
		if rels.Len() <= 32*1024 {
			t.Fatalf("fixture too small (%d bytes), want >32KB", rels.Len())
		}
		files = append(files, fb3File{"fb3/_rels/body.xml.rels", rels.String()})

		ebook, err := reader.Read(buildFb3Blob(t, files))
		if err != nil {
			t.Fatalf("Read() error: %v", err)
		}
		if got := ebook.Resources().Len(); got != n+1 {
			t.Errorf("Len() = %d, want %d (body + images)", got, n+1)
		}
		if _, ok := ebook.Resources().GetByID("img42"); !ok {
			t.Error("GetByID(img42) missing")
		}
	})

	t.Run("thumbnail outside body rels becomes cover", func(t *testing.T) {
		files := minimalFb3Files()
		files[1] = fb3File{"_rels/.rels", strings.Replace(minimalRootRels, "fb3/img/cover.png", "cover.jpg", 1)}
		files = append(files, fb3File{"cover.jpg", "JPGDATA"})
		ebook, err := reader.Read(buildFb3Blob(t, files))
		if err != nil {
			t.Fatalf("Read() error: %v", err)
		}
		rs := ebook.Resources()
		if got := rs.Len(); got != 4 {
			t.Fatalf("Len() = %d, want 4 (body + 2 images + cover)", got)
		}
		cover, ok := rs.CoverImage()
		if !ok {
			t.Fatalf("CoverImage() missing")
		}
		if cover.Properties != "cover-image" || cover.Href != "cover.jpg" {
			t.Errorf("cover = %+v, want cover.jpg with cover-image", cover)
		}
		if cover.MediaType != mediatype.JPEG {
			t.Errorf("cover type = %q, want jpeg", cover.MediaType)
		}
	})
}

type fb3File struct {
	name    string
	content string
}

// buildFb3Blob packs entries (in order) into an in-memory zip blob.
func buildFb3Blob(t *testing.T, files []fb3File) book.Blob {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, f := range files {
		fw, err := w.Create(f.name)
		if err != nil {
			t.Fatalf("Create(%q) error: %v", f.name, err)
		}
		if _, err := fw.Write([]byte(f.content)); err != nil {
			t.Fatalf("Write(%q) error: %v", f.name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
	return book.NewBytesBlob(buf.Bytes())
}
