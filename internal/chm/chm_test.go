package chm

import (
	"encoding/binary"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
)

// encCword encodes a big-endian 7-bit group varint (chmlib cword).
func encCword(v uint64) []byte {
	var tmp [10]byte
	n := 0
	for {
		tmp[n] = byte(v & 0x7F)
		n++
		v >>= 7
		if v == 0 {
			break
		}
	}
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		b := tmp[n-1-i]
		if i < n-1 {
			b |= 0x80
		}
		out[i] = b
	}
	return out
}

type dirEntry struct {
	name   string
	space  uint64
	start  uint64
	length uint64
}

func buildPMGL(entries []dirEntry, blkLen int) []byte {
	return buildPMGLNext(entries, blkLen, -1)
}

func buildPMGLNext(entries []dirEntry, blkLen int, next int32) []byte {
	block := make([]byte, blkLen)
	if blkLen < pmglLen {
		return block
	}
	copy(block, "PMGL")
	binary.LittleEndian.PutUint32(block[8:], 0)
	binary.LittleEndian.PutUint32(block[12:], 0xFFFFFFFF)
	binary.LittleEndian.PutUint32(block[16:], uint32(next))
	pos := pmglLen
	for _, e := range entries {
		pos += copy(block[pos:], encCword(uint64(len(e.name))))
		pos += copy(block[pos:], e.name)
		pos += copy(block[pos:], encCword(e.space))
		pos += copy(block[pos:], encCword(e.start))
		pos += copy(block[pos:], encCword(e.length))
	}
	binary.LittleEndian.PutUint32(block[4:], uint32(blkLen-pos))
	return block
}

func buildPMGI(keys []string, pages []uint64, blkLen int) []byte {
	block := make([]byte, blkLen)
	if blkLen < pmgiLen {
		return block
	}
	copy(block, "PMGI")
	pos := pmgiLen
	for i, k := range keys {
		pos += copy(block[pos:], encCword(uint64(len(k))))
		pos += copy(block[pos:], k)
		pos += copy(block[pos:], encCword(pages[i]))
	}
	binary.LittleEndian.PutUint32(block[4:], uint32(blkLen-pos))
	return block
}

type sysEntry struct {
	code uint16
	data []byte
}

func buildSystem(version uint32, entries []sysEntry) []byte {
	out := make([]byte, 4)
	binary.LittleEndian.PutUint32(out, version)
	for _, e := range entries {
		var hdr [4]byte
		binary.LittleEndian.PutUint16(hdr[0:], e.code)
		binary.LittleEndian.PutUint16(hdr[2:], uint16(len(e.data)))
		out = append(out, hdr[:]...)
		out = append(out, e.data...)
	}
	return out
}

func sysInfoEntry(lcid uint32) []byte {
	// 28-byte code 4 struct with the LCID first.
	info := make([]byte, 28)
	binary.LittleEndian.PutUint32(info, lcid)
	return info
}

// fixture assembles a minimal CHM: ITSF + ITSP + directory pages + data area.
type fixture struct {
	files     map[string][]byte
	spaces    map[string]uint64
	over      map[string]dirEntry
	dir       [][]byte
	blkLen    int
	indexRoot int64
	indexHead int64
	useIndex  bool
}

func buildFixture(fx fixture) []byte {
	blkLen := fx.blkLen
	if blkLen == 0 {
		blkLen = 0x1000
	}
	names := []string{}
	for name := range fx.files {
		names = append(names, name)
	}
	sortStrings(names)

	data := []byte{}
	entries := []dirEntry{}
	for _, name := range names {
		space := fx.spaces[name]
		if o, ok := fx.over[name]; ok {
			entries = append(entries, o)
			data = append(data, fx.files[name]...)
			continue
		}
		entries = append(entries, dirEntry{name, space, uint64(len(data)), uint64(len(fx.files[name]))})
		data = append(data, fx.files[name]...)
	}

	dirBlocks := fx.dir
	if dirBlocks == nil {
		dirBlocks = [][]byte{buildPMGL(entries, blkLen)}
	}

	itsfLen := itsfV3Len
	dirOff := int64(itsfLen)
	dirLen := int64(itspLen + len(dirBlocks)*blkLen)
	dataOff := dirOff + dirLen

	itsf := make([]byte, itsfLen)
	copy(itsf, "ITSF")
	binary.LittleEndian.PutUint32(itsf[4:], 3)
	binary.LittleEndian.PutUint32(itsf[8:], uint32(itsfLen))
	binary.LittleEndian.PutUint64(itsf[72:], uint64(dirOff))
	binary.LittleEndian.PutUint64(itsf[80:], uint64(dirLen))
	binary.LittleEndian.PutUint64(itsf[88:], uint64(dataOff))

	itsp := make([]byte, itspLen)
	copy(itsp, "ITSP")
	binary.LittleEndian.PutUint32(itsp[4:], 1)
	binary.LittleEndian.PutUint32(itsp[8:], itspLen)
	binary.LittleEndian.PutUint32(itsp[16:], uint32(blkLen))
	root, head := int64(-1), int64(0)
	if fx.useIndex {
		root, head = fx.indexRoot, fx.indexHead
	}
	binary.LittleEndian.PutUint32(itsp[28:], uint32(root))
	binary.LittleEndian.PutUint32(itsp[32:], uint32(head))

	out := append(itsf, itsp...)
	for _, b := range dirBlocks {
		out = append(out, b...)
	}
	return append(out, data...)
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// lzxStored wraps plain bytes in a minimal LZX stream with a single
// uncompressed block (even lengths only, to skip pad handling).
func lzxStored(plain []byte) []byte {
	if len(plain)%2 != 0 {
		plain = append(append([]byte{}, plain...), 0)
	}
	length := len(plain)
	word1 := uint16(0x3000 | (length >> 12))
	word2 := uint16((length & 0xFFF) << 4)
	out := []byte{byte(word1), byte(word1 >> 8), byte(word2), byte(word2 >> 8)}
	out = append(out, make([]byte, 12)...)
	return append(out, plain...)
}

func chmTestConfig() config.Config {
	return config.DefaultConfig()
}

func TestSupports(t *testing.T) {
	r := NewChmReader(chmTestConfig())
	v3 := make([]byte, 0x60)
	copy(v3, "ITSF")
	binary.LittleEndian.PutUint32(v3[4:], 3)
	binary.LittleEndian.PutUint32(v3[8:], 0x60)
	v2 := make([]byte, 0x58)
	copy(v2, "ITSF")
	binary.LittleEndian.PutUint32(v2[4:], 2)
	binary.LittleEndian.PutUint32(v2[8:], 0x58)
	badver := append([]byte{}, v3...)
	binary.LittleEndian.PutUint32(badver[4:], 4)
	shortlen := append([]byte{}, v3...)
	binary.LittleEndian.PutUint32(shortlen[8:], 0x20)

	cases := map[string]struct {
		blob book.Blob
		want bool
	}{
		"v3":        {book.NewBytesBlob(v3), true},
		"v2":        {book.NewBytesBlob(v2), true},
		"badver":    {book.NewBytesBlob(badver), false},
		"shortlen":  {book.NewBytesBlob(shortlen), false},
		"smallfile": {book.NewBytesBlob(v3[:10]), false},
		"lit":       {book.NewBytesBlob(append([]byte("ITOLITLS"), make([]byte, 32)...)), false},
		"text":      {book.NewBytesBlob([]byte("plain text")), false},
		"empty":     {book.NewBytesBlob(nil), false},
	}
	for name, tc := range cases {
		if got := r.Supports(tc.blob); got != tc.want {
			t.Errorf("%s: Supports = %v, want %v", name, got, tc.want)
		}
	}
}
