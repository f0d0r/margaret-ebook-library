package chm

import (
	"context"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

// buildURLSTR assembles a #URLSTR file from (local, url) pairs, returning the
// file bytes and each pair's entry offset. With zeroURL the URL offsets are
// zero, meaning the URL is the Local itself (the common case).
// buildURLSTR assembles a #URLSTR file from (local, url) pairs, returning the
// file bytes and each pair's entry offset. Entries are contiguous
// header+local+url runs like in real files. With zeroURL the URL offsets
// are zero, meaning the URL is the Local itself (the common case).
func buildURLSTR(pairs [][2]string, zeroURL bool) ([]byte, []uint32) {
	var raw []byte
	var offs []uint32
	for _, p := range pairs {
		offs = append(offs, uint32(len(raw)))
		urlPos := len(raw) + 8 + len(p[0]) + 1
		var hdr [8]byte
		if !zeroURL {
			binary.LittleEndian.PutUint32(hdr[0:], uint32(urlPos))
		}
		raw = append(raw, hdr[:]...)
		raw = append(raw, p[0]...)
		raw = append(raw, 0)
		raw = append(raw, p[1]...)
		raw = append(raw, 0)
	}
	return raw, offs
}

// buildURLTBL assembles a #URLTBL file: one 12-byte slot per topics index.
func buildURLTBL(urlOffs []uint32) []byte {
	raw := make([]byte, len(urlOffs)*12)
	for i, off := range urlOffs {
		binary.LittleEndian.PutUint32(raw[i*12+4:], uint32(i))
		binary.LittleEndian.PutUint32(raw[i*12+8:], off)
	}
	return raw
}

// buildTopics assembles a #TOPICS file from #URLTBL offsets.
func buildTopics(urltblOffs []uint32) []byte {
	raw := make([]byte, len(urltblOffs)*16)
	for i, off := range urltblOffs {
		binary.LittleEndian.PutUint32(raw[i*16+4:], 0xFFFFFFFF)
		binary.LittleEndian.PutUint32(raw[i*16+8:], off)
	}
	return raw
}

func contentFixture() fixture {
	hhc := `<html><body><ul><li>` +
		`<object type="text/sitemap"><param name="Name" value="B">` +
		`<param name="Local" value="b.html"></object>` +
		`<ul><li><object type="text/sitemap"><param name="Name" value="A">` +
		`<param name="Local" value="a.html#frag"></object></li></ul>` +
		`<li><object type="text/sitemap"><param name="Name" value="Gone">` +
		`<param name="Local" value="gone.html"></object></li>` +
		`<li><object type="text/sitemap"><param name="Name" value="Pic">` +
		`<param name="Local" value="pic.png"></object></li>` +
		`</li></ul></body></html>`
	return fixture{files: map[string][]byte{
		"/#SYSTEM": buildSystem(3, []sysEntry{
			{sysTitle, []byte("Content Book\x00")},
			{sysDefaultTopic, []byte("/default.html\x00")},
			{sysInfo, sysInfoEntry(1033)},
		}),
		"/default.html": []byte(`<html><head><title>D</title></head><body>home</body></html>`),
		"/a.html":       []byte(`<html><body><p>Alpha</p></body></html>`),
		"/b.html":       []byte(`<html><body><p>Beta</p></body></html>`),
		"/style.css":    []byte(`p { color: red; }`),
		"/pic.png":      []byte("\x89PNG\r\n\x1a\n"),
		"/index.hhc":    []byte(hhc),
		"/emptydir/":    {},
	}}
}

func allHrefs(b book.Book) []string {
	var out []string
	for _, r := range b.Resources().All() {
		out = append(out, r.ResolvedHref)
	}
	return out
}

func orderHrefs(b book.Book) []string {
	var out []string
	for _, it := range b.Resources().ReadingOrder() {
		out = append(out, it.Resource.ResolvedHref)
	}
	return out
}

func TestManifest(t *testing.T) {
	b := readFixture(t, contentFixture())
	media := map[string]string{}
	for _, r := range b.Resources().All() {
		media[r.ResolvedHref] = r.MediaType
	}
	want := map[string]string{
		"default.html": mediatype.HTML,
		"a.html":       mediatype.HTML,
		"b.html":       mediatype.HTML,
		"style.css":    mediatype.CSS,
		"pic.png":      mediatype.PNG,
		"index.hhc":    mediatype.HTML,
	}
	for href, mt := range want {
		if media[href] != mt {
			t.Errorf("manifest %q media = %q, want %q", href, media[href], mt)
		}
	}
	for _, href := range allHrefs(b) {
		if strings.HasPrefix(href, "#") {
			t.Errorf("control entry %q listed in manifest", href)
		}
		if href == "." || strings.HasSuffix(href, "emptydir") {
			t.Errorf("directory entry %q listed in manifest", href)
		}
	}
}

func TestReadingOrderHHC(t *testing.T) {
	b := readFixture(t, contentFixture())
	got := orderHrefs(b)
	want := []string{"b.html", "a.html"}
	if len(got) != len(want) {
		t.Fatalf("order = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %q, want %q", got, want)
		}
	}
	for _, it := range b.Resources().ReadingOrder() {
		if !it.Linear {
			t.Errorf("%q Linear = false, want true", it.Resource.ResolvedHref)
		}
	}
}

func TestReadingOrderTopicsFallback(t *testing.T) {
	urlstr, offs := buildURLSTR([][2]string{{"l2", "page2.html"}, {"l1", "page1.html"}}, false)
	fx := fixture{files: map[string][]byte{
		"/#SYSTEM": buildSystem(3, []sysEntry{
			{sysTitle, []byte("T\x00")},
			{sysDefaultTopic, []byte("/page1.html\x00")},
			{sysInfo, sysInfoEntry(1033)},
		}),
		"/page1.html": []byte(`<html><body>one</body></html>`),
		"/page2.html": []byte(`<html><body>two</body></html>`),
		"/#URLSTR":    urlstr,
		"/#URLTBL":    buildURLTBL(offs),
		"/#TOPICS":    buildTopics([]uint32{0, 12}),
	}}
	b := readFixture(t, fx)
	got := orderHrefs(b)
	if len(got) != 2 || got[0] != "page2.html" || got[1] != "page1.html" {
		t.Errorf("order = %q, want [page2.html page1.html]", got)
	}
}

func TestReadingOrderTopicsLocalFallback(t *testing.T) {
	// Zero URL offsets: the URL is the Local itself (the common real-file case).
	urlstr, offs := buildURLSTR([][2]string{{"page2.html", ""}, {"page1.html", ""}}, true)
	fx := fixture{files: map[string][]byte{
		"/#SYSTEM": buildSystem(3, []sysEntry{
			{sysTitle, []byte("T\x00")},
			{sysDefaultTopic, []byte("/page1.html\x00")},
		}),
		"/page1.html": []byte(`<html><body>one</body></html>`),
		"/page2.html": []byte(`<html><body>two</body></html>`),
		"/#URLSTR":    urlstr,
		"/#URLTBL":    buildURLTBL(offs),
		"/#TOPICS":    buildTopics([]uint32{0, 12}),
	}}
	b := readFixture(t, fx)
	got := orderHrefs(b)
	if len(got) != 2 || got[0] != "page2.html" || got[1] != "page1.html" {
		t.Errorf("order = %q, want [page2.html page1.html]", got)
	}
}

func TestOpenContainerTinyBlock(t *testing.T) {
	fx := fixture{
		blkLen: 8,
		files:  map[string][]byte{"/#SYSTEM": buildSystem(3, nil)},
	}
	if _, err := openContainer(book.NewBytesBlob(buildFixture(fx)), chmTestConfig().MaxResourceSize); err == nil {
		t.Error("openContainer with tiny block_len: want error")
	}
}

func TestAbsurdCoords(t *testing.T) {
	fx := fixture{
		files: map[string][]byte{"/evil": nil},
		over: map[string]dirEntry{
			"/evil": {"/evil", 0, 1 << 62, 1 << 62},
		},
	}
	c := openFixture(t, fx)
	if _, err := c.getFile("/evil"); err == nil {
		t.Error("getFile with absurd coords: want error, no panic")
	}
}

func TestCheckCoordsCompressedStream(t *testing.T) {
	// Compressed entries address the uncompressed stream, which may exceed
	// the file size (e.g. 3 MB of content in a 876 KB file).
	if _, err := checkCoords("x", entry{space: 1, start: 1 << 20, length: 100}, 1024); err != nil {
		t.Errorf("checkCoords compressed beyond file size: %v, want nil", err)
	}
	if _, err := checkCoords("x", entry{space: 0, start: 1 << 20, length: 100}, 1024); err == nil {
		t.Error("checkCoords uncompressed beyond file size: want error")
	}
	if _, err := checkCoords("x", entry{space: 2, start: 0, length: 0}, 1024); err == nil {
		t.Error("checkCoords unknown section: want error")
	}
}

func TestReadingOrderDefaultFallback(t *testing.T) {
	b := readFixture(t, fixture{files: map[string][]byte{
		"/#SYSTEM": buildSystem(3, []sysEntry{
			{sysTitle, []byte("T\x00")},
			{sysDefaultTopic, []byte("/only.html\x00")},
		}),
		"/only.html": []byte(`<html><body>only</body></html>`),
	}})
	got := orderHrefs(b)
	if len(got) != 1 || got[0] != "only.html" {
		t.Errorf("order = %q, want [only.html]", got)
	}
}

func TestOpenRawBytes(t *testing.T) {
	raw := []byte("<html><body>Caf\xe9</body></html>")
	b := readFixture(t, fixture{files: map[string][]byte{
		"/#SYSTEM": buildSystem(3, []sysEntry{{sysTitle, []byte("T\x00")}}),
		"/p.html":  raw,
	}})
	res, ok := b.Resources().GetByHref("p.html")
	if !ok {
		t.Fatal("GetByHref(p.html) missing")
	}
	got, err := res.Data()
	if err != nil {
		t.Fatalf("Data: %v", err)
	}
	if string(got) != string(raw) {
		t.Errorf("Open bytes changed: got %q, want %q", got, raw)
	}
}

func TestOpenAsPlainText(t *testing.T) {
	b := readFixture(t, contentFixture())
	res, ok := b.Resources().GetByHref("a.html")
	if !ok {
		t.Fatal("GetByHref(a.html) missing")
	}
	got, err := res.DataAs(context.Background(), mediatype.PlainText)
	if err != nil {
		t.Fatalf("DataAs: %v", err)
	}
	if !strings.Contains(string(got), "Alpha") {
		t.Errorf("plain text = %q, want it to contain Alpha", got)
	}
}

func TestCoverAltHeuristic(t *testing.T) {
	def := `<html><head><title>D</title></head><body>` +
		`<img src="nav.gif" alt="next">` +
		`<img src="cover.jpg" alt="Front Cover">` +
		`</body></html>`
	b := readFixture(t, fixture{files: map[string][]byte{
		"/#SYSTEM": buildSystem(3, []sysEntry{
			{sysTitle, []byte("T\x00")},
			{sysDefaultTopic, []byte("/default.html\x00")},
		}),
		"/default.html": []byte(def),
		"/cover.jpg":    []byte("\xff\xd8\xff"),
		"/nav.gif":      []byte("GIF89a"),
	}})
	cover, ok := b.Resources().CoverImage()
	if !ok {
		t.Fatal("CoverImage missing, want cover.jpg")
	}
	if cover.ResolvedHref != "cover.jpg" {
		t.Errorf("cover = %q, want cover.jpg", cover.ResolvedHref)
	}
	if cover.MediaType != mediatype.JPEG {
		t.Errorf("cover media = %q, want image/jpeg", cover.MediaType)
	}
}

func TestCoverAbsent(t *testing.T) {
	b := readFixture(t, fixture{files: map[string][]byte{
		"/#SYSTEM": buildSystem(3, []sysEntry{
			{sysTitle, []byte("T\x00")},
			{sysDefaultTopic, []byte("/default.html\x00")},
		}),
		"/default.html": []byte(`<html><body><img src="pic.png" alt="logo"></body></html>`),
		"/pic.png":      []byte("\x89PNG\r\n\x1a\n"),
	}})
	if _, ok := b.Resources().CoverImage(); ok {
		t.Error("CoverImage present, want none without alt=cover")
	}
}
