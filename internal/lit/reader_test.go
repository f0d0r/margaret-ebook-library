package lit

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
	"github.com/f0d0r/margaret-ebook-library/internal/opf"
)

// ---------------------------------------------------------------------------
// Synthetic LIT builder: a minimal container with section-0 entries.
// ---------------------------------------------------------------------------

func ubOpen(tag byte) []byte { return []byte{0x00, 0x01, tag} }

// ubLeaf opens a childless element: OPENING|CLOSING leaves goingDown false
// so the attribute terminator writes " />".
func ubLeaf(tag byte) []byte { return []byte{0x00, 0x03, tag} }
func ubClose() []byte        { return []byte{0x00, 0x02, 0x00} }
func ubAttr(id byte, v string) []byte {
	out := []byte{id, byte(len(v) + 1)}
	return append(out, v...)
}
func ubText(s string) []byte { return []byte(s) }

func fixtureMeta() []byte {
	var b []byte
	b = append(b, ubOpen(1)...) // package
	b = append(b, 0x00)
	b = append(b, ubOpen(20)...) // metadata
	b = append(b, 0x00)
	b = append(b, ubOpen(21)...) // dc-metadata
	b = append(b, 0x00)
	b = append(b, ubOpen(2)...) // dc:Title
	b = append(b, 0x00)
	b = append(b, ubText("Test Title")...)
	b = append(b, ubClose()...)
	b = append(b, ubOpen(3)...) // dc:Creator
	b = append(b, ubAttr(0x0D, "aut")...)
	b = append(b, 0x00)
	b = append(b, ubText("Test Author")...)
	b = append(b, ubClose()...)
	b = append(b, ubOpen(23)...) // dc:Description
	b = append(b, 0x00)
	b = append(b, ubText("A description")...)
	b = append(b, ubClose()...)
	b = append(b, ubOpen(31)...) // dc:Language
	b = append(b, 0x00)
	b = append(b, ubText("en")...)
	b = append(b, ubClose()...)
	b = append(b, ubClose()...)  // dc-metadata
	b = append(b, ubOpen(35)...) // x-metadata
	b = append(b, 0x00)
	b = append(b, ubLeaf(36)...) // meta
	b = append(b, ubAttr(0x14, "cover")...)
	b = append(b, ubAttr(0x15, "cover")...)
	b = append(b, 0x00)          // self-close
	b = append(b, ubClose()...)  // x-metadata
	b = append(b, ubClose()...)  // metadata
	b = append(b, ubOpen(16)...) // manifest
	b = append(b, 0x00)
	b = append(b, ubLeaf(17)...) // item
	b = append(b, ubAttr(0x06, "c1")...)
	b = append(b, 0x07, 0x04, 0x02, 'c', '1') // href with 1-char prefix
	b = append(b, ubAttr(0x08, "text/html")...)
	b = append(b, 0x00)          // self-close
	b = append(b, ubLeaf(17)...) // cover item
	b = append(b, ubAttr(0x06, "cover")...)
	b = append(b, 0x07, 0x0C, 0x02)
	b = append(b, "cover.jpeg"...)
	b = append(b, ubAttr(0x08, "image/jpeg")...)
	b = append(b, 0x00)          // self-close
	b = append(b, ubClose()...)  // manifest
	b = append(b, ubOpen(18)...) // spine
	b = append(b, 0x00)
	b = append(b, ubLeaf(19)...) // itemref
	b = append(b, ubAttr(0x0A, "c1")...)
	b = append(b, 0x00)
	b = append(b, ubLeaf(19)...) // ghost itemref (no manifest item)
	b = append(b, ubAttr(0x0A, "ghost")...)
	b = append(b, 0x00)
	b = append(b, ubClose()...) // spine
	b = append(b, ubClose()...) // package
	return b
}

func fixtureManifest() []byte {
	var b []byte
	b = append(b, 0x01, 'r')
	b = append(b, 0x01, 0x00, 0x00, 0x00) // spine: 1 file
	b = append(b, 0x00, 0x00, 0x00, 0x00) // offset
	b = append(b, 0x02, 'c', '1')
	b = append(b, 0x09)
	b = append(b, "chap.html"...)
	b = append(b, 0x09)
	b = append(b, "text/html"...)
	b = append(b, 0x00)
	b = append(b, 0x00, 0x00, 0x00, 0x00) // not spine: 0
	b = append(b, 0x00, 0x00, 0x00, 0x00) // css: 0
	b = append(b, 0x02, 0x00, 0x00, 0x00) // images: 2 files
	b = append(b, 0x00, 0x00, 0x00, 0x00) // offset
	b = append(b, 0x03, 'c', 'o', 'v')
	b = append(b, 0x0A)
	b = append(b, "cover.jpeg"...)
	b = append(b, 0x0A)
	b = append(b, "image/jpeg"...)
	b = append(b, 0x00)
	// second images entry unused by the OPF (proves extra entries are fine)
	b = append(b, 0x00, 0x00, 0x00, 0x00) // offset
	b = append(b, 0x02, 'x', '2')
	b = append(b, 0x09)
	b = append(b, "other.png"...)
	b = append(b, 0x09)
	b = append(b, "image/png"...)
	b = append(b, 0x00)
	b = append(b, 0x00) // end
	return b
}

func fixtureNameList() []byte {
	var b []byte
	b = append(b, 0x00, 0x00, 0x01, 0x00)
	b = append(b, 0x0C, 0x00)
	for _, c := range "Uncompressed" {
		b = append(b, byte(c), 0x00)
	}
	return append(b, 0x00, 0x00)
}

type litDirEntry struct {
	name string
	data []byte
}

func u32le(v int64) []byte {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], uint32(v))
	return b[:]
}

// enc encodes a big-endian 7-bit group varint.
func enc(v int64) []byte {
	var groups []byte
	groups = append(groups, byte(v&0x7F))
	v >>= 7
	for v > 0 {
		groups = append(groups, byte(v&0x7F)|0x80)
		v >>= 7
	}
	for i, j := 0, len(groups)-1; i < j; i, j = i+1, j-1 {
		groups[i], groups[j] = groups[j], groups[i]
	}
	return groups
}

// buildLit assembles a minimal valid LIT file. All entries live in section
// 0 (raw). extra entries are appended after the standard three.
func buildLit(extra []litDirEntry) []byte {
	entries := []litDirEntry{
		{metaEntry, fixtureMeta()},
		{"/manifest", fixtureManifest()},
		{nameListEntry, fixtureNameList()},
	}
	entries = append(entries, extra...)

	const contentOff = int64(352)
	const countSize = int64(16)
	const guidSize = int64(16)

	// The directory embeds payload offsets, so converge in two passes
	// (magnitudes are stable after the first).
	var dirLen int64 = 32 + 512
	offsets := make([]int64, len(entries))
	var eb []byte
	for pass := 0; pass < 2; pass++ {
		// Directory offsets are content-relative.
		off := dirLen + 16 + 16 + 16
		eb = nil
		for i, e := range entries {
			offsets[i] = off
			off += int64(len(e.data))
			eb = append(eb, byte(len(e.name)))
			eb = append(eb, e.name...)
			eb = append(eb, 0x00)
			eb = append(eb, enc(offsets[i])...)
			eb = append(eb, enc(int64(len(e.data)))...)
		}
		dirLen = 32 + int64(50+len(eb))
	}
	chunkSize := 50 + len(eb)
	r := int64(chunkSize) - 48 - int64(len(eb))
	chunk := []byte("AOLL")
	chunk = append(chunk, u32le(r)...)
	chunk = append(chunk, make([]byte, 40)...)
	chunk = append(chunk, eb...)
	chunk = append(chunk, byte(len(entries)), 0x00)

	dirPiece := []byte("IFCM")
	dirPiece = append(dirPiece, make([]byte, 4)...)
	dirPiece = append(dirPiece, u32le(int64(chunkSize))...)
	dirPiece = append(dirPiece, make([]byte, 12)...)
	dirPiece = append(dirPiece, 0x01, 0x00, 0x00, 0x00)
	dirPiece = append(dirPiece, make([]byte, 4)...)
	dirPiece = append(dirPiece, chunk...)

	countPiece := make([]byte, 16)
	binary.LittleEndian.PutUint32(countPiece[8:], 0x200)
	countPiece[12] = 0x02

	sec := make([]byte, 232)
	binary.LittleEndian.PutUint32(sec[4:], 152)
	copy(sec[152:], "CAOL")
	binary.LittleEndian.PutUint32(sec[152+4:], 2)
	binary.LittleEndian.PutUint32(sec[152+8:], 80)
	binary.LittleEndian.PutUint32(sec[152+20:], uint32(chunkSize))
	binary.LittleEndian.PutUint32(sec[152+24:], 0x200)
	sec[152+28] = 0x00
	sec[152+32] = 0x02
	copy(sec[200:], "ITSF")
	binary.LittleEndian.PutUint32(sec[200+4:], 4)
	binary.LittleEndian.PutUint32(sec[200+8:], 32)
	binary.LittleEndian.PutUint32(sec[200+12:], 1)
	binary.LittleEndian.PutUint32(sec[200+16:], uint32(contentOff))
	binary.LittleEndian.PutUint32(sec[200+28:], 0x409)

	head := make([]byte, 40)
	copy(head, "ITOLITLS")
	binary.LittleEndian.PutUint32(head[8:], 1)
	binary.LittleEndian.PutUint32(head[12:], 40)
	binary.LittleEndian.PutUint32(head[16:], 5)
	binary.LittleEndian.PutUint32(head[20:], 232)

	piece := func(off, size int64) []byte {
		var p [16]byte
		binary.LittleEndian.PutUint32(p[0:], uint32(off))
		binary.LittleEndian.PutUint32(p[8:], uint32(size))
		return p[:]
	}
	payloadOff := contentOff + dirLen + countSize + 2*guidSize
	var file []byte
	file = append(file, head...)
	file = append(file, piece(contentOff, payloadOff+totalPayload(entries)-contentOff)...)
	file = append(file, piece(contentOff, dirLen)...)
	file = append(file, piece(contentOff+dirLen, countSize)...)
	file = append(file, piece(contentOff+dirLen+countSize, guidSize)...)
	file = append(file, piece(contentOff+dirLen+countSize+guidSize, guidSize)...)
	file = append(file, sec...)
	file = append(file, dirPiece...)
	file = append(file, countPiece...)
	file = append(file, make([]byte, guidSize)...)
	file = append(file, make([]byte, guidSize)...)
	for _, e := range entries {
		file = append(file, e.data...)
	}
	return file
}

func totalPayload(entries []litDirEntry) int64 {
	var n int64
	for _, e := range entries {
		n += int64(len(e.data))
	}
	return n
}

func TestLitReadMetadata(t *testing.T) {
	r := NewLitReader(config.DefaultConfig())
	b := book.NewBytesBlob(buildLit(nil))
	if !r.Supports(b) {
		t.Fatal("Supports = false for synthetic LIT")
	}
	ebook, err := r.Read(b)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	md := ebook.Metadata()
	if md.Title != "Test Title" {
		t.Errorf("Title = %q, want %q", md.Title, "Test Title")
	}
	if len(md.Authors) != 1 || md.Authors[0] != "Test Author" {
		t.Errorf("Authors = %q, want [Test Author]", md.Authors)
	}
	if md.Description != "A description" {
		t.Errorf("Description = %q, want %q", md.Description, "A description")
	}
	if len(md.Languages) != 1 || md.Languages[0] != "en" {
		t.Errorf("Languages = %q, want [en]", md.Languages)
	}
	if ebook.FileType() != book.LIT {
		t.Errorf("FileType = %q, want lit", ebook.FileType())
	}
	if ebook.Version() != "1" {
		t.Errorf("Version = %q, want 1", ebook.Version())
	}
	if n := ebook.Resources().Len(); n != 0 {
		t.Errorf("Resources().Len() = %d, want 0", n)
	}
}

func TestLitReadResolvesHref(t *testing.T) {
	b := book.NewBytesBlob(buildLit(nil))
	c, err := openContainer(b, config.DefaultConfig().MaxResourceSize)
	if err != nil {
		t.Fatalf("openContainer: %v", err)
	}
	raw, err := c.getFile(metaEntry)
	if err != nil {
		t.Fatalf("getFile: %v", err)
	}
	paths := map[string]string{}
	for id, item := range c.manifest {
		paths[id] = item.path
	}
	dec, err := decodeUnbinary(raw, &opfTables, paths, nil, "")
	if err != nil {
		t.Fatalf("decodeUnbinary: %v", err)
	}
	p, err := opf.Parse(bytes.NewReader(dec))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	item := p.ItemById("c1")
	if item == nil {
		t.Fatal("ItemById(c1) = nil")
	}
	if item.Href != "chap.html" {
		t.Errorf("Href = %q, want chap.html", item.Href)
	}
}

func TestLitReadDRM(t *testing.T) {
	r := NewLitReader(config.DefaultConfig())
	b := book.NewBytesBlob(buildLit([]litDirEntry{{"/DRMStorage/Licenses/EUL", []byte{}}}))
	if !r.Supports(b) {
		t.Fatal("Supports = false")
	}
	if _, err := r.Read(b); !errors.Is(err, book.ErrDRM) {
		t.Fatalf("Read err = %v, want ErrDRM", err)
	}
}

func TestLitReadCorrupt(t *testing.T) {
	r := NewLitReader(config.DefaultConfig())
	for name, data := range map[string][]byte{
		"empty":    {},
		"short":    []byte("ITOLITLS"),
		"badmagic": append([]byte("NOTALIT!"), make([]byte, 40)...),
	} {
		b := book.NewBytesBlob(data)
		if r.Supports(b) {
			t.Errorf("%s: Supports = true, want false", name)
		}
		if _, err := r.Read(b); !errors.Is(err, book.ErrCorrupt) {
			t.Errorf("%s: Read err = %v, want ErrCorrupt", name, err)
		}
	}
}

func fixtureContent() []byte {
	var b []byte
	b = append(b, ubOpen(74)...) // p
	b = append(b, 0x00)
	b = append(b, ubText("Hi ")...)
	b = append(b, ubOpen(3)...) // a
	b = append(b, 0x01)         // attr href
	b = append(b, 0x03, 0x02, 'u')
	b = append(b, 0x00)
	b = append(b, ubText("y")...)
	b = append(b, ubClose()...) // a
	b = append(b, ubClose()...) // p
	return b
}

func TestLitReadResources(t *testing.T) {
	coverBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0, 'c', 'o', 'v', 'e', 'r'}
	contentBin := fixtureContent()
	b := book.NewBytesBlob(buildLit([]litDirEntry{
		{"/data/c1/content", contentBin},
		{"/data/cov", coverBytes},
	}))
	ebook, err := NewLitReader(config.DefaultConfig()).Read(b)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	rs := ebook.Resources()
	if n := rs.Len(); n != 2 {
		t.Fatalf("Resources().Len() = %d, want 2", n)
	}
	ro := rs.ReadingOrder()
	if len(ro) != 1 {
		t.Fatalf("len(ReadingOrder) = %d, want 1 (ghost skipped)", len(ro))
	}
	ch := ro[0].Resource
	if ch.Id != "c1" || ch.MediaType != "text/html" || !ro[0].Linear {
		t.Errorf("chapter = %+v, want c1/text/html linear", ch)
	}
	if ch.Size != int64(len(contentBin)) {
		t.Errorf("Size = %d, want %d", ch.Size, len(contentBin))
	}
	if _, ok := rs.GetByID("ghost"); ok {
		t.Errorf("ghost resource should be skipped")
	}

	cover, ok := rs.CoverImage()
	if !ok {
		t.Fatal("CoverImage not found")
	}
	if cover.Id != "cover" || cover.MediaType != "image/jpeg" {
		t.Errorf("cover = %+v, want cover/image/jpeg", cover)
	}
	rc, err := cover.Open()
	if err != nil {
		t.Fatalf("cover.Open: %v", err)
	}
	gotCover, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("cover read: %v", err)
	}
	if !bytes.Equal(gotCover, coverBytes) {
		t.Errorf("cover bytes = %x, want %x", gotCover, coverBytes)
	}

	rc, err = ch.Open()
	if err != nil {
		t.Fatalf("chapter.Open: %v", err)
	}
	gotDoc, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("chapter read: %v", err)
	}
	if !bytes.HasPrefix(gotDoc, []byte(contentHTMLDecl+`<p>Hi <a href="u">y</a></p>`)) {
		t.Errorf("chapter doc = %q, want decl + paragraph", gotDoc)
	}

	trc, err := ch.OpenAs(context.Background(), "text/plain")
	if err != nil {
		t.Fatalf("OpenAs: %v", err)
	}
	gotText, err := io.ReadAll(trc)
	_ = trc.Close()
	if err != nil {
		t.Fatalf("text read: %v", err)
	}
	if !strings.Contains(string(gotText), "Hi") || !strings.Contains(string(gotText), "y") {
		t.Errorf("text = %q, want Hi/y content", gotText)
	}
}

// ---------------------------------------------------------------------------
// Transformed-section fixture: one spine document served from an
// LZX-compressed section (uncompressed LZX blocks, no encoder needed).
// ---------------------------------------------------------------------------

var litLZXGUID = []byte{0xC6, 0x07, 0x90, 0x0A, 0x76, 0x40, 0xD3, 0x11, 0x87, 0x89, 0x00, 0x00, 0xF8, 0x10, 0x57, 0x54}

const litLZXGUIDText = "{0A9007C6-4076-11D3-8789-0000F8105754}"

// lzxRawBlock wraps raw bytes in a single uncompressed LZX block chunk
// (intel prelude 0, type 3, 24-bit length, R0/R1/R2, raw data).
func lzxRawBlock(raw []byte) []byte {
	n := len(raw)
	hi, lo := n>>8, n&0xFF
	u0 := 0x3000 | (hi >> 4)
	u1 := (((hi & 0xF) << 8) | lo) << 4
	b := []byte{byte(u0), byte(u0 >> 8), byte(u1), byte(u1 >> 8)}
	b = append(b, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00)
	return append(b, raw...)
}

func fixtureSectionMeta() []byte {
	var b []byte
	b = append(b, ubOpen(1)...) // package
	b = append(b, 0x00)
	b = append(b, ubOpen(20)...) // metadata
	b = append(b, 0x00)
	b = append(b, ubOpen(21)...) // dc-metadata
	b = append(b, 0x00)
	b = append(b, ubOpen(2)...) // dc:Title
	b = append(b, 0x00)
	b = append(b, ubText("Sec")...)
	b = append(b, ubClose()...)
	b = append(b, ubClose()...)  // dc-metadata
	b = append(b, ubClose()...)  // metadata
	b = append(b, ubOpen(16)...) // manifest
	b = append(b, 0x00)
	b = append(b, ubLeaf(17)...) // item s9
	b = append(b, ubAttr(0x06, "s9")...)
	b = append(b, 0x07, 0x04, 0x02, 's', '9')
	b = append(b, ubAttr(0x08, "text/html")...)
	b = append(b, 0x00)
	b = append(b, ubClose()...)  // manifest
	b = append(b, ubOpen(18)...) // spine
	b = append(b, 0x00)
	b = append(b, ubLeaf(19)...) // itemref s9
	b = append(b, ubAttr(0x0A, "s9")...)
	b = append(b, 0x00)
	b = append(b, ubClose()...) // spine
	b = append(b, ubClose()...) // package
	return b
}

func fixtureSectionManifest() []byte {
	var b []byte
	b = append(b, 0x01, 'r')
	b = append(b, 0x01, 0x00, 0x00, 0x00)
	b = append(b, 0x00, 0x00, 0x00, 0x00)
	b = append(b, 0x02, 's', '9')
	b = append(b, 0x08)
	b = append(b, "doc.html"...)
	b = append(b, 0x09)
	b = append(b, "text/html"...)
	b = append(b, 0x00)
	b = append(b, 0x00, 0x00, 0x00, 0x00)
	b = append(b, 0x00, 0x00, 0x00, 0x00)
	b = append(b, 0x00, 0x00, 0x00, 0x00)
	b = append(b, 0x00)
	return b
}

func fixtureNameList2() []byte {
	var b []byte
	b = append(b, 0x00, 0x00, 0x02, 0x00)
	b = append(b, 0x0C, 0x00)
	for _, c := range "Uncompressed" {
		b = append(b, byte(c), 0x00)
	}
	b = append(b, 0x00, 0x00)
	b = append(b, 0x02, 0x00)
	for _, c := range "S1" {
		b = append(b, byte(c), 0x00)
	}
	return append(b, 0x00, 0x00)
}

type secEntry struct {
	name    string
	section int64
	off     int64 // content-relative for stored, explicit for views
	size    int64
	data    []byte // stored payload (section 0 only)
}

// buildLitSections assembles a container with transformed sections.
func buildLitSections(t *testing.T, entries []secEntry) []byte {
	t.Helper()
	const contentOff = int64(352)
	// The directory embeds payload offsets, so converge in two passes.
	dirLen := int64(32 + 512)
	var eb []byte
	for pass := 0; pass < 2; pass++ {
		base := dirLen + 16 + 16 + 16
		eb = nil
		cur := base
		for _, e := range entries {
			o := e.off
			sz := e.size
			if e.data != nil {
				o = cur
				sz = int64(len(e.data))
				cur += sz
			}
			eb = append(eb, byte(len(e.name)))
			eb = append(eb, e.name...)
			eb = append(eb, enc(e.section)...)
			eb = append(eb, enc(o)...)
			eb = append(eb, enc(sz)...)
		}
		dirLen = 32 + int64(50+len(eb))
	}
	chunkSize := 50 + len(eb)
	r := int64(chunkSize) - 48 - int64(len(eb))
	chunk := []byte("AOLL")
	chunk = append(chunk, u32le(r)...)
	chunk = append(chunk, make([]byte, 40)...)
	chunk = append(chunk, eb...)
	chunk = append(chunk, byte(len(entries)), 0x00)

	dirPiece := []byte("IFCM")
	dirPiece = append(dirPiece, make([]byte, 4)...)
	dirPiece = append(dirPiece, u32le(int64(chunkSize))...)
	dirPiece = append(dirPiece, make([]byte, 12)...)
	dirPiece = append(dirPiece, 0x01, 0x00, 0x00, 0x00)
	dirPiece = append(dirPiece, make([]byte, 4)...)
	dirPiece = append(dirPiece, chunk...)

	countPiece := make([]byte, 16)
	binary.LittleEndian.PutUint32(countPiece[8:], 0x200)
	countPiece[12] = 0x02

	sec := make([]byte, 232)
	binary.LittleEndian.PutUint32(sec[4:], 152)
	copy(sec[152:], "CAOL")
	binary.LittleEndian.PutUint32(sec[152+4:], 2)
	binary.LittleEndian.PutUint32(sec[152+8:], 80)
	binary.LittleEndian.PutUint32(sec[152+20:], uint32(chunkSize))
	binary.LittleEndian.PutUint32(sec[152+24:], 0x200)
	sec[152+32] = 0x02
	copy(sec[200:], "ITSF")
	binary.LittleEndian.PutUint32(sec[200+4:], 4)
	binary.LittleEndian.PutUint32(sec[200+8:], 32)
	binary.LittleEndian.PutUint32(sec[200+12:], 1)
	binary.LittleEndian.PutUint32(sec[200+16:], uint32(contentOff))
	binary.LittleEndian.PutUint32(sec[200+28:], 0x409)

	head := make([]byte, 40)
	copy(head, "ITOLITLS")
	binary.LittleEndian.PutUint32(head[8:], 1)
	binary.LittleEndian.PutUint32(head[12:], 40)
	binary.LittleEndian.PutUint32(head[16:], 5)
	binary.LittleEndian.PutUint32(head[20:], 232)

	piece := func(off, size int64) []byte {
		var p [16]byte
		binary.LittleEndian.PutUint32(p[0:], uint32(off))
		binary.LittleEndian.PutUint32(p[8:], uint32(size))
		return p[:]
	}
	var file []byte
	file = append(file, head...)
	payloadStart := contentOff + dirLen + 16 + 16 + 16
	payloadEnd := payloadStart
	for _, e := range entries {
		if e.data != nil {
			payloadEnd += int64(len(e.data))
		}
	}
	file = append(file, piece(contentOff, payloadEnd-contentOff)...)
	file = append(file, piece(contentOff, dirLen)...)
	file = append(file, piece(contentOff+dirLen, 16)...)
	file = append(file, piece(contentOff+dirLen+16, 16)...)
	file = append(file, piece(contentOff+dirLen+32, 16)...)
	file = append(file, sec...)
	file = append(file, dirPiece...)
	file = append(file, countPiece...)
	file = append(file, make([]byte, 16)...)
	file = append(file, make([]byte, 16)...)
	for _, e := range entries {
		file = append(file, e.data...)
	}
	return file
}

func TestLitLZXSection(t *testing.T) {
	docBin := fixtureContent()
	chunk := lzxRawBlock(docBin)
	control := make([]byte, 32)
	binary.LittleEndian.PutUint32(control[0:], 7)
	copy(control[4:], "LZXC")
	binary.LittleEndian.PutUint32(control[12:], 1)
	reset := make([]byte, 40)
	binary.LittleEndian.PutUint32(reset[0:], 3)
	binary.LittleEndian.PutUint32(reset[12:], 0x28)
	binary.LittleEndian.PutUint32(reset[16:], uint32(len(docBin)))
	binary.LittleEndian.PutUint32(reset[32:], 32768)
	rtName := "::DataSpace/Storage/S1/Transform/" + litLZXGUIDText + "/InstanceData/ResetTable"
	entries := []secEntry{
		{metaEntry, 0, 0, 0, fixtureSectionMeta()},
		{"/manifest", 0, 0, 0, fixtureSectionManifest()},
		{nameListEntry, 0, 0, 0, fixtureNameList2()},
		{"::DataSpace/Storage/S1/Transform/List", 0, 0, 0, litLZXGUID},
		{"::DataSpace/Storage/S1/Content", 0, 0, 0, chunk},
		{"::DataSpace/Storage/S1/ControlData", 0, 0, 0, control},
		{rtName, 0, 0, 0, reset},
		{"/data/s9/content", 1, 0, int64(len(docBin)), nil},
	}
	b := book.NewBytesBlob(buildLitSections(t, entries))
	ebook, err := NewLitReader(config.DefaultConfig()).Read(b)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got := ebook.Metadata().Title; got != "Sec" {
		t.Errorf("Title = %q, want Sec", got)
	}
	rs := ebook.Resources()
	ro := rs.ReadingOrder()
	if len(ro) != 1 || ro[0].Resource.Id != "s9" {
		t.Fatalf("ReadingOrder = %+v, want single s9", ro)
	}
	if ro[0].Resource.MediaType != "text/html" {
		t.Errorf("MediaType = %q, want text/html", ro[0].Resource.MediaType)
	}
	rc, err := ro[0].Resource.Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	got, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if want := contentHTMLDecl + `<p>Hi <a href="u">y</a></p>`; string(got) != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func TestSplicePackageTail(t *testing.T) {
	// Shape observed in the Gutenberg corpus: one unbalanced close ends
	// the package early, leaving manifest and spine as trailing siblings.
	// The splice moves the first </package> past them.
	in := `<package unique-identifier="uid"><metadata>` +
		`<dc-metadata><dc:Title>Gettysburg Address</dc:Title></dc-metadata>` +
		`<dc:Language>en</dc:Language></metadata></package>` +
		`<manifest><item id="cover" href="cover" media-type="text/x-oeb1-document" />` +
		`<item id="Content" href="Content" media-type="text/x-oeb1-document" /></manifest>` +
		`<spine><itemref idref="cover" /><itemref idref="Content" /></spine>` +
		`<tours></tours><guide></guide>`
	got := splicePackageTail([]byte(in))
	p, err := opf.Parse(bytes.NewReader(got))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(p.Manifest.Items) != 2 {
		t.Errorf("manifest items = %d, want 2", len(p.Manifest.Items))
	}
	if len(p.Spine.ItemRefs) != 2 {
		t.Errorf("spine refs = %d, want 2", len(p.Spine.ItemRefs))
	}
	if gotTitle := p.Title(); gotTitle != "Gettysburg Address" {
		t.Errorf("Title = %q", gotTitle)
	}
	if langs := p.Languages(); len(langs) != 1 || langs[0] != "en" {
		t.Errorf("Languages = %q, want [en]", langs)
	}
}

func TestSplicePackageTailBalanced(t *testing.T) {
	// Balanced documents pass through byte-identical.
	in := `<package><metadata><dc:title>T</dc:title></metadata></package>`
	if got := splicePackageTail([]byte(in)); string(got) != in {
		t.Errorf("splice changed balanced input: %q", got)
	}
	// Trailing content without manifest/spine is left alone.
	in2 := `<package></package><tours></tours>`
	if got := splicePackageTail([]byte(in2)); string(got) != in2 {
		t.Errorf("splice changed manifest-less input: %q", got)
	}
}
