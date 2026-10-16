package chm

import (
	"encoding/binary"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
)

func openFixture(t *testing.T, fx fixture) *container {
	t.Helper()
	c, err := openContainer(book.NewBytesBlob(buildFixture(fx)), chmTestConfig().MaxResourceSize)
	if err != nil {
		t.Fatalf("openContainer: %v", err)
	}
	return c
}

func TestResolveCaseInsensitive(t *testing.T) {
	c := openFixture(t, fixture{files: map[string][]byte{
		"/#SYSTEM": buildSystem(3, []sysEntry{{sysTitle, []byte("T\x00")}}),
	}})
	for _, name := range []string{"/#SYSTEM", "/#system", "/#System"} {
		raw, err := c.getFile(name)
		if err != nil {
			t.Errorf("getFile(%q): %v", name, err)
			continue
		}
		if len(raw) < 4 || binary.LittleEndian.Uint32(raw[:4]) != 3 {
			t.Errorf("getFile(%q): bad #SYSTEM version", name)
		}
	}
}

func TestResolveMissing(t *testing.T) {
	c := openFixture(t, fixture{files: map[string][]byte{
		"/#SYSTEM": buildSystem(3, nil),
	}})
	if _, err := c.getFile("/nope"); err == nil {
		t.Error("getFile(/nope): want error")
	}
}

func TestResolvePMGIWalk(t *testing.T) {
	const blkLen = 0x1000
	sys := buildSystem(3, []sysEntry{{sysTitle, []byte("T\x00")}})
	stringsRaw := []byte("\x00fallback\x00")
	files := map[string][]byte{"/#SYSTEM": sys, "/#STRINGS": stringsRaw}
	// Names sorted: /#STRINGS < /#SYSTEM, so leaf 1 holds both and leaf 2 the rest.
	leaf1 := buildPMGLNext([]dirEntry{
		{"/#STRINGS", 0, 0, uint64(len(stringsRaw))},
		{"/#SYSTEM", 0, uint64(len(stringsRaw)), uint64(len(sys))},
	}, blkLen, 2)
	leaf2 := buildPMGL([]dirEntry{
		{"/zzz", 0, 0, 1},
	}, blkLen)
	orphan := buildPMGL([]dirEntry{
		{"/orphan", 0, 0, 1},
	}, blkLen)
	root := buildPMGI([]string{"/#STRINGS", "/zzz"}, []uint64{1, 2}, blkLen)
	c := openFixture(t, fixture{
		files:    files,
		dir:      [][]byte{root, leaf1, leaf2, orphan},
		useIndex: true, indexRoot: 0, indexHead: 1,
	})

	raw, err := c.getFile("/#SYSTEM")
	if err != nil {
		t.Fatalf("getFile via PMGI: %v", err)
	}
	if len(raw) != len(sys) {
		t.Errorf("getFile via PMGI: got %d bytes, want %d", len(raw), len(sys))
	}
	if _, err := c.getFile("/#STRINGS"); err != nil {
		t.Errorf("getFile(/#STRINGS) via PMGI: %v", err)
	}
	listed, err := c.enumerate()
	if err != nil {
		t.Fatalf("enumerate: %v", err)
	}
	for _, e := range listed {
		if e.name == "/orphan" {
			t.Error("enumerate listed orphan page entry /orphan outside the leaf chain")
		}
	}
}

// compressedFilesFixture builds a CHM whose space-1 payloads share one LZX
// stream, with real directory starts for each.
func compressedFilesFixture(t *testing.T, files map[string][]byte, spaces map[string]uint64) fixture {
	t.Helper()
	var names []string
	for name, sp := range spaces {
		if sp == 1 {
			names = append(names, name)
		}
	}
	sortStrings(names)
	var plain []byte
	starts := map[string]uint64{}
	lengths := map[string]uint64{}
	for _, n := range names {
		starts[n] = uint64(len(plain))
		lengths[n] = uint64(len(files[n]))
		plain = append(plain, files[n]...)
		files[n] = nil
	}
	if len(plain)%2 != 0 {
		plain = append(plain, 0)
	}
	comp := lzxStored(plain)
	rt := make([]byte, lzxcResetTableLen+8)
	binary.LittleEndian.PutUint32(rt[0:], 2)
	binary.LittleEndian.PutUint32(rt[4:], 1)
	binary.LittleEndian.PutUint32(rt[12:], lzxcResetTableLen)
	binary.LittleEndian.PutUint64(rt[16:], uint64(len(plain)))
	binary.LittleEndian.PutUint64(rt[24:], uint64(len(comp)))
	binary.LittleEndian.PutUint64(rt[32:], 0x8000)
	ctl := make([]byte, 0x1C)
	binary.LittleEndian.PutUint32(ctl[0:], 0x1C)
	copy(ctl[4:], "LZXC")
	binary.LittleEndian.PutUint32(ctl[8:], 1)
	binary.LittleEndian.PutUint32(ctl[12:], 0x8000)
	binary.LittleEndian.PutUint32(ctl[16:], 0x8000)
	binary.LittleEndian.PutUint32(ctl[20:], 1)
	files[resetTablePath] = rt
	files[controlPath] = ctl
	files[contentPath] = comp
	over := map[string]dirEntry{}
	for _, n := range names {
		over[n] = dirEntry{n, 1, starts[n], lengths[n]}
	}
	return fixture{files: files, spaces: spaces, over: over}
}
func compressedFixture(t *testing.T, plain []byte) fixture {
	t.Helper()
	comp := lzxStored(plain)
	rt := make([]byte, lzxcResetTableLen+8) // header plus one table entry (offset 0)
	binary.LittleEndian.PutUint32(rt[0:], 2)
	binary.LittleEndian.PutUint32(rt[4:], 1)
	binary.LittleEndian.PutUint32(rt[12:], lzxcResetTableLen)
	binary.LittleEndian.PutUint64(rt[16:], uint64(len(plain)))
	binary.LittleEndian.PutUint64(rt[24:], uint64(len(comp)))
	binary.LittleEndian.PutUint64(rt[32:], 0x8000)
	ctl := make([]byte, 0x1C)
	binary.LittleEndian.PutUint32(ctl[0:], 0x1C)
	copy(ctl[4:], "LZXC")
	binary.LittleEndian.PutUint32(ctl[8:], 1)
	binary.LittleEndian.PutUint32(ctl[12:], 0x8000)
	binary.LittleEndian.PutUint32(ctl[16:], 0x8000)
	binary.LittleEndian.PutUint32(ctl[20:], 1)
	return fixture{
		files: map[string][]byte{
			"/#SYSTEM": buildSystem(3, []sysEntry{
				{sysTitle, []byte("Compressed Title\x00")},
				{sysDefaultTopic, []byte("/content.html\x00")},
				{sysInfo, sysInfoEntry(1033)},
			}),
			"/content.html": nil, // payload lives in the LZX stream, not the data area
			resetTablePath:  rt,
			controlPath:     ctl,
			contentPath:     comp,
		},
		spaces: map[string]uint64{"/content.html": 1},
		over: map[string]dirEntry{
			"/content.html": {"/content.html", 1, 0, uint64(len(plain))},
		},
	}
}

func TestCompressedSystem(t *testing.T) {
	// Compressed #SYSTEM exercises cachedSystem -> getFile -> ensureComp,
	// which must not self-deadlock on the container mutex.
	sys := buildSystem(3, []sysEntry{
		{sysTitle, []byte("Squeezed Title\x00")},
		{sysDefaultTopic, []byte("/default.html\x00")},
		{sysInfo, sysInfoEntry(1033)},
	})
	def := []byte(`<html><head><title>Doc</title></head><body>x</body></html>`)
	fx := compressedFilesFixture(t,
		map[string][]byte{"/#SYSTEM": sys, "/default.html": def},
		map[string]uint64{"/#SYSTEM": 1, "/default.html": 1},
	)
	r := NewChmReader(chmTestConfig())
	b, err := r.Read(book.NewBytesBlob(buildFixture(fx)))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got := b.Metadata().Title; got != "Squeezed Title" {
		t.Errorf("Title = %q, want Squeezed Title", got)
	}
	if got := len(b.Resources().ReadingOrder()); got != 1 {
		t.Errorf("order len = %d, want 1", got)
	}
}

func TestReadCompressed(t *testing.T) {
	plain := []byte("<html><head><title>Hi</title></head><body>x</body></html>")
	if len(plain)%2 != 0 {
		plain = append(plain, ' ')
	}
	c := openFixture(t, compressedFixture(t, plain))
	raw, err := c.getFile("/content.html")
	if err != nil {
		t.Fatalf("getFile compressed: %v", err)
	}
	if string(raw) != string(plain) {
		t.Errorf("getFile compressed: got %q, want %q", raw, plain)
	}
}
func TestCompressedBogusContentLength(t *testing.T) {
	// Real files store an unreliable (too large) Content length; control
	// loading must not measure it against the file size.
	plain := []byte("<html><body>hi</body></html>")
	fx := compressedFixture(t, plain)
	c, err := openContainer(book.NewBytesBlob(buildFixture(fx)), chmTestConfig().MaxResourceSize)
	if err != nil {
		t.Fatalf("openContainer: %v", err)
	}
	cn, err := c.resolveRaw(contentPath)
	if err != nil {
		t.Fatalf("resolveRaw content: %v", err)
	}
	fx.over[contentPath] = dirEntry{contentPath, 0, cn.start, 1 << 30}
	c = openFixture(t, fx)
	raw, err := c.getFile("/content.html")
	if err != nil {
		t.Fatalf("getFile compressed: %v", err)
	}
	if string(raw) != string(plain) {
		t.Errorf("getFile compressed: got %q, want %q", raw, plain)
	}
}

func TestTruncatedTailFailsLoud(t *testing.T) {
	// A truncated compressed tail must error, never serve short/garbage data.
	plain := []byte("<html><head><title>Hi</title></head><body>x</body></html>")
	if len(plain)%2 != 0 {
		plain = append(plain, ' ')
	}
	full := buildFixture(compressedFixture(t, plain))
	c, err := openContainer(book.NewBytesBlob(full[:len(full)-10]), chmTestConfig().MaxResourceSize)
	if err != nil {
		t.Fatalf("openContainer truncated: %v", err)
	}
	if _, err := c.getFile("/content.html"); err == nil {
		t.Error("getFile truncated tail: want error, not short data")
	}
}
