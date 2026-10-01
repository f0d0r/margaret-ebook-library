package compress

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
)

// ---------------------------------------------------------------------------
// Test-side bit writer: MSB-first bits into 16-bit little-endian units,
// mirroring the decoder's consumption order. Only used to build test
// vectors; the hand-computed golden test below anchors the convention
// independently of this helper.
// ---------------------------------------------------------------------------

type lzxTestWriter struct {
	hold uint32
	n    int
	out  []byte
	// frameOut counts output bytes across blocks for frame padding.
	frameOut int64
}

func (w *lzxTestWriter) bits(v uint32, n int) {
	w.hold |= v << (32 - w.n - n)
	w.n += n
	for w.n >= 16 {
		u := w.hold >> 16
		w.out = append(w.out, byte(u), byte(u>>8))
		w.hold <<= 16
		w.n -= 16
	}
}

// padFrame emits zero bits to the next 16-bit boundary. LZX frames are
// unit-aligned in the stream, so the writer must pad whenever cumulative
// output crosses a 32KB frame boundary mid-stream.
func (w *lzxTestWriter) padFrame() {
	for w.n%16 != 0 {
		w.bits(0, 1)
	}
}

// noteOut records n output bytes, padding on frame crossings.
func (w *lzxTestWriter) noteOut(n int) {
	prev := w.frameOut
	w.frameOut += int64(n)
	if prev/32768 != w.frameOut/32768 {
		w.padFrame()
	}
}

func (w *lzxTestWriter) bytes() []byte {
	if w.n > 0 {
		u := w.hold >> 16
		w.out = append(w.out, byte(u), byte(u>>8))
		w.hold, w.n = 0, 0
	}
	return w.out
}

// testCanonical assigns canonical Huffman codes for the test encoder.
func testCanonical(nSyms int, lens []uint8) map[int]struct {
	code, length int
} {
	var count [17]int
	for _, l := range lens {
		count[l]++
	}
	count[0] = 0 // unused symbols consume no code space
	start := make([]int, 18)
	code := 0
	for b := 1; b <= 16; b++ {
		code = (code + count[b-1]) << 1
		start[b] = code
	}
	res := map[int]struct {
		code, length int
	}{}
	for s := 0; s < nSyms; s++ {
		if lens[s] != 0 {
			res[s] = struct {
				code, length int
			}{start[lens[s]], int(lens[s])}
			start[lens[s]]++
		}
	}
	return res
}

func (w *lzxTestWriter) huffLensSlice(lens []uint8) {
	w.huffLensOld(len(lens), lens, nil)
}

// huffLensOld encodes code lengths as deltas against old (nil = zeros,
// i.e. a first block). Runs of equal nonzero targets use code 19.
func (w *lzxTestWriter) huffLensOld(nSyms int, lens, old []uint8) {
	pre := make([]uint8, 20)
	// Determine needed pretree symbols: deltas and run codes. A 19-run
	// carries a trailing value symbol (rep).
	type run struct {
		sym, extra, nbits int
		rep               int
	}
	var ops []run
	used := map[int]bool{}
	cur := make([]uint8, nSyms)
	copy(cur, old)
	delta := func(pos int, v uint8) int {
		dz := (int(cur[pos]) - int(v)) % 17
		if dz < 0 {
			dz += 17
		}
		return dz
	}
	for x := 0; x < nSyms; {
		if lens[x] == 0 {
			z := x
			for z < nSyms && lens[z] == 0 {
				z++
			}
			n := z - x
			for n > 0 {
				switch {
				case n >= 20+31:
					ops = append(ops, run{sym: 18, extra: 31, nbits: 5})
					n -= 20 + 31
				case n >= 20:
					ops = append(ops, run{sym: 18, extra: n - 20, nbits: 5})
					n = 0
				case n >= 4+15:
					ops = append(ops, run{sym: 17, extra: 15, nbits: 4})
					n -= 4 + 15
				case n >= 4:
					ops = append(ops, run{sym: 17, extra: n - 4, nbits: 4})
					n = 0
				default:
					ops = append(ops, run{sym: 0})
					n--
				}
			}
			for i := x; i < z; i++ {
				cur[i] = 0
			}
			x = z
			continue
		}
		// Run of equal nonzero targets: first literal, rest via 19.
		v := lens[x]
		z := x + 1
		for z < nSyms && lens[z] == v {
			z++
		}
		ops = append(ops, run{sym: delta(x, v), rep: -1})
		cur[x] = v
		x++
		rem := z - x
		for rem >= 4 {
			k := 5
			if rem == 4 || rem == 6 || rem == 7 || rem == 8 {
				k = 4
			}
			ops = append(ops, run{sym: 19, extra: k - 4, nbits: 1, rep: delta(x, v)})
			for i := 0; i < k; i++ {
				cur[x+i] = v
			}
			x += k
			rem -= k
		}
		for ; x < z; x++ {
			ops = append(ops, run{sym: delta(x, v), rep: -1})
			cur[x] = v
		}
	}
	for _, op := range ops {
		used[op.sym] = true
		if op.rep >= 0 {
			used[op.rep] = true
		}
	}
	// Assign tiny pretree lengths in symbol order.
	order := []int{}
	for s := 0; s < 20; s++ {
		if used[s] {
			order = append(order, s)
		}
	}
	// Assign one shared length that always fits: the smallest L with
	// 2^L >= number of used symbols (an incomplete but valid tree).
	L := 1
	for (1 << L) < len(order) {
		L++
	}
	for _, s := range order {
		pre[s] = uint8(L)
	}
	for _, l := range pre {
		w.bits(uint32(l), 4)
	}
	codes := testCanonical(20, pre)
	for _, op := range ops {
		c := codes[op.sym]
		w.bits(uint32(c.code), c.length)
		switch op.sym {
		case 17:
			w.bits(uint32(op.extra), 4)
		case 18:
			w.bits(uint32(op.extra), 5)
		case 19:
			w.bits(uint32(op.extra), 1)
			r := codes[op.rep]
			w.bits(uint32(r.code), r.length)
		}
	}
}

type lzxTestElem struct {
	lit        bool
	b          byte
	matchLen   int
	matchSlot  int
	verbatim   uint32
	verbatimN  int
	alignedSym int
	useAligned bool
	foot       int
	hasFoot    bool
}

func (w *lzxTestWriter) verbatimBlock(mainLens []uint8, lenLens []uint8, elems []lzxTestElem, outLen int) {
	w.verbatimBlockOld(mainLens, lenLens, elems, outLen, nil, nil)
}

func (w *lzxTestWriter) verbatimBlockOld(mainLens []uint8, lenLens []uint8, elems []lzxTestElem, outLen int, oldMain, oldLen []uint8) {
	w.bits(1, 3) // verbatim
	w.bits(uint32(outLen>>8), 16)
	w.bits(uint32(outLen&0xFF), 8)
	nMain := len(mainLens)
	w.huffLensOld(256, mainLens[:256], sliceOld(oldMain, 0, 256))
	w.huffLensOld(nMain-256, mainLens[256:], sliceOld(oldMain, 256, nMain))
	w.huffLensOld(len(lenLens), lenLens, oldLen)
	mainCodes := testCanonical(nMain, mainLens)
	lenCodes := testCanonical(len(lenLens), lenLens)
	for _, e := range elems {
		if e.lit {
			c := mainCodes[int(e.b)]
			w.bits(uint32(c.code), c.length)
			w.noteOut(1)
			continue
		}
		lh := e.matchLen - 2
		el := 256 + (e.matchSlot << 3)
		var foot int
		hasFoot := false
		if lh >= 7 {
			el += 7
			foot, hasFoot = e.foot, true
		} else {
			el += lh
		}
		c := mainCodes[el]
		w.bits(uint32(c.code), c.length)
		if hasFoot {
			f := lenCodes[foot]
			w.bits(uint32(f.code), f.length)
		}
		if e.matchSlot >= 4 || e.useAligned {
			w.bits(e.verbatim, e.verbatimN)
			if e.useAligned {
				w.bits(uint32(e.alignedSym), 3) // fixed 3-bit aligned syms in tests
			}
		}
		w.noteOut(e.matchLen)
	}
}

func sliceOld(old []uint8, from, to int) []uint8 {
	if old == nil {
		return nil
	}
	return old[from:to]
}

// lzxSlotFor finds a verbatim position slot encoding offset.
func lzxSlotFor(offset, posnSlots int) (slot int, verbatim uint32, n int, ok bool) {
	for s := posnSlots - 1; s >= 4; s-- {
		base := int64(lzxPositionBase[s]) - 2
		if int64(offset) < base {
			continue
		}
		if vb := int64(offset) - base; vb < int64(1)<<lzxExtraBits[s] {
			return s, uint32(vb), int(lzxExtraBits[s]), true
		}
	}
	return 0, 0, 0, false
}

func (w *lzxTestWriter) alignedBlock(mainLens []uint8, lenLens []uint8, alignedLens []uint8, elems []lzxTestElem, outLen int) {
	w.bits(2, 3) // aligned
	w.bits(uint32(outLen>>8), 16)
	w.bits(uint32(outLen&0xFF), 8)
	for _, l := range alignedLens {
		w.bits(uint32(l), 3)
	}
	nMain := len(mainLens)
	w.huffLensSlice(mainLens[:256])
	w.huffLensSlice(mainLens[256:])
	w.huffLensSlice(lenLens)
	mainCodes := testCanonical(nMain, mainLens)
	lenCodes := testCanonical(len(lenLens), lenLens)
	_ = lenCodes
	for _, e := range elems {
		if e.lit {
			c := mainCodes[int(e.b)]
			w.bits(uint32(c.code), c.length)
			w.noteOut(1)
			continue
		}
		lh := e.matchLen - 2
		el := 256 + (e.matchSlot << 3)
		if lh >= 7 {
			el += 7
		} else {
			el += lh
		}
		c := mainCodes[el]
		w.bits(uint32(c.code), c.length)
		if e.hasFoot {
			f := lenCodes[e.foot]
			w.bits(uint32(f.code), f.length)
		}
		if e.matchSlot >= 3 {
			// Test aligned lens are all 3 bits; extra handling mirrors
			// the table: slot 10 has extra 4 -> 1 verbatim bit + aligned.
			w.bits(e.verbatim, e.verbatimN)
			if e.useAligned {
				ac := testCanonical(8, alignedLens)
				a := ac[e.alignedSym]
				w.bits(uint32(a.code), a.length)
			}
		}
		w.noteOut(e.matchLen)
	}
}

func decodeAll(t *testing.T, in []byte, windowBits int, expectOut int64, maxOut int64) []byte {
	t.Helper()
	rc, err := NewLZXReader(bytes.NewReader(in), windowBits, expectOut, maxOut)
	if err != nil {
		t.Fatalf("NewLZXReader() error: %v", err)
	}
	defer func() { _ = rc.Close() }()
	out, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	return out
}

// TestLZXGoldenUncompressed anchors the bitstream convention with a
// hand-computed vector: intel=0, uncompressed block length 1, R0=R1=R2=1,
// raw byte 0x41. Header bits: 0 | 011 | 0x0000 | 0x01 -> bytes 00 30 10 00.
func TestLZXGoldenUncompressed(t *testing.T) {
	in := []byte{
		0x00, 0x30, 0x10, 0x00,
		0x01, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x00, 0x00,
		0x41,
	}
	got := decodeAll(t, in, 15, 1, 1<<20)
	if !bytes.Equal(got, []byte{0x41}) {
		t.Fatalf("decode = %x, want 41", got)
	}
}

// TestLZXWindowBitsMatrix decodes the golden vector under every window size.
func TestLZXWindowBitsMatrix(t *testing.T) {
	in := []byte{
		0x00, 0x30, 0x10, 0x00,
		0x01, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x00, 0x00,
		0x41,
	}
	for wb := 15; wb <= 21; wb++ {
		if got := decodeAll(t, in, wb, 1, 1<<20); !bytes.Equal(got, []byte{0x41}) {
			t.Errorf("windowBits=%d: decode = %x, want 41", wb, got)
		}
	}
	for _, wb := range []int{0, 1, 14, 22, 32, -1} {
		if _, err := NewLZXReader(bytes.NewReader(in), wb, 1, 1<<20); err == nil {
			t.Errorf("windowBits=%d: expected error, got nil", wb)
		} else if !errors.Is(err, book.ErrCorrupt) {
			t.Errorf("windowBits=%d: error = %v, want ErrCorrupt", wb, err)
		}
	}
}

// TestLZXVerbatimLiterals decodes a literals-only verbatim block.
func TestLZXVerbatimLiterals(t *testing.T) {
	var w lzxTestWriter
	w.bits(0, 1) // no E8
	mainLens := make([]uint8, 256+30*8)
	mainLens['H'], mainLens['e'], mainLens['l'], mainLens['o'] = 2, 2, 2, 3
	lenLens := make([]uint8, 249)
	lenLens[0] = 1
	msg := []byte("Hello")
	var elems []lzxTestElem
	for _, b := range msg {
		elems = append(elems, lzxTestElem{lit: true, b: b})
	}
	w.verbatimBlock(mainLens, lenLens, elems, len(msg))
	if got := decodeAll(t, w.bytes(), 15, int64(len(msg)), 1<<20); !bytes.Equal(got, msg) {
		t.Fatalf("decode = %q, want %q", got, msg)
	}
}

// TestLZXVerbatimMatch exercises a verbatim match plus R0 repeat.
func TestLZXVerbatimMatch(t *testing.T) {
	var w lzxTestWriter
	w.bits(0, 1)
	mainLens := make([]uint8, 256+30*8)
	mainLens['A'], mainLens['B'] = 2, 2
	// 256 + slot<<3 + lenHeader for slot 4 header 2 and slot 0 header 0.
	mainLens[256+(4<<3)+2] = 2
	mainLens[256+(0<<3)+0] = 3
	lenLens := make([]uint8, 249)
	lenLens[0] = 1
	elems := []lzxTestElem{
		{lit: true, b: 'A'},
		{lit: true, b: 'B'},
		{matchLen: 4, matchSlot: 4, verbatim: 0, verbatimN: 1}, // offset 2 (extra=1)
		{matchLen: 2, matchSlot: 0},                            // R0 repeat
	}
	// "AB" + 4 bytes from offset 2 ("ABAB") + 2 bytes ("AB") = "ABABABAB".
	w.verbatimBlock(mainLens, lenLens, elems, 8)
	if got, want := decodeAll(t, w.bytes(), 15, 8, 1<<20), []byte("ABABABAB"); !bytes.Equal(got, want) {
		t.Fatalf("decode = %q, want %q", got, want)
	}
}

// TestLZXAlignedMatch exercises the aligned block offset path
// (slot 10: 1 verbatim bit + 3 aligned bits, offset 30..45).
func TestLZXAlignedMatch(t *testing.T) {
	var w lzxTestWriter
	w.bits(0, 1)
	mainLens := make([]uint8, 256+30*8)
	for _, b := range []byte("wxyz") {
		mainLens[b] = 3
	}
	mainLens[256+(10<<3)+2] = 3 // match len 4, slot 10
	lenLens := make([]uint8, 249)
	lenLens[0] = 1
	alignedLens := []uint8{3, 3, 3, 3, 3, 3, 3, 3}
	prefix := bytes.Repeat([]byte("wxyz"), 8) // 32 bytes
	var elems []lzxTestElem
	for _, b := range prefix {
		elems = append(elems, lzxTestElem{lit: true, b: b})
	}
	// Offset 32 back, length 4: 30 + (0<<3) + 2.
	elems = append(elems, lzxTestElem{matchLen: 4, matchSlot: 10, verbatim: 0, verbatimN: 1, alignedSym: 2, useAligned: true, hasFoot: false})
	w.alignedBlock(mainLens, lenLens, alignedLens, elems, 36)
	want := append(append([]byte(nil), prefix...), prefix[:4]...)
	if got := decodeAll(t, w.bytes(), 15, 36, 1<<20); !bytes.Equal(got, want) {
		t.Fatalf("decode = %q, want %q", got, want)
	}
}

// TestLZXLongCodes forces Huffman tree extension nodes with a 15-bit code.
func TestLZXLongCodes(t *testing.T) {
	var w lzxTestWriter
	w.bits(0, 1)
	mainLens := make([]uint8, 256+30*8)
	mainLens['A'] = 2
	mainLens['Z'] = 15
	lenLens := make([]uint8, 249)
	lenLens[0] = 1
	elems := []lzxTestElem{
		{lit: true, b: 'A'},
		{lit: true, b: 'A'},
		{lit: true, b: 'A'},
		{lit: true, b: 'Z'},
	}
	w.verbatimBlock(mainLens, lenLens, elems, 4)
	if got, want := decodeAll(t, w.bytes(), 15, 4, 1<<20), []byte("AAAZ"); !bytes.Equal(got, want) {
		t.Fatalf("decode = %q, want %q", got, want)
	}
}

// TestLZXLengthFooter exercises matches with length header 7 plus a LENGTH
// tree footer (here: length 10 = 7 + 1 + 2).
func TestLZXLengthFooter(t *testing.T) {
	var w lzxTestWriter
	w.bits(0, 1)
	mainLens := make([]uint8, 256+30*8)
	mainLens['A'] = 1
	mainLens[256+(3<<3)+7] = 2
	lenLens := make([]uint8, 249)
	lenLens[1] = 1
	elems := []lzxTestElem{
		{lit: true, b: 'A'},
		{matchLen: 10, matchSlot: 3, foot: 1},
	}
	w.verbatimBlock(mainLens, lenLens, elems, 11)
	if got, want := decodeAll(t, w.bytes(), 15, 11, 1<<20), bytes.Repeat([]byte("A"), 11); !bytes.Equal(got, want) {
		t.Fatalf("decode len = %d, want 11", len(got))
	}
}

// TestLZXMultiFrame spans two 32KB frames with one uncompressed block.
func TestLZXMultiFrame(t *testing.T) {
	const n = 32768 + 7232
	raw := make([]byte, n)
	for i := range raw {
		raw[i] = byte(i * 7)
	}
	var w lzxTestWriter
	w.bits(0, 1)
	w.bits(3, 3)
	w.bits(uint32(n>>8), 16)
	w.bits(uint32(n&0xFF), 8)
	w.out = append(w.bytes(), 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00)
	w.out = append(w.out, raw...)
	if got := decodeAll(t, w.out, 15, n, 1<<26); !bytes.Equal(got, raw) {
		t.Fatalf("decode len = %d, want %d (equal=%v)", len(got), n, bytes.Equal(got, raw))
	}
}

// TestLZXOddUncompressedPad requires a pad-byte skip between an odd-length
// and the following uncompressed block.
func TestLZXOddUncompressedPad(t *testing.T) {
	var w lzxTestWriter
	w.bits(0, 1)
	writeRaw := func(data []byte) {
		w.bits(3, 3)
		w.bits(uint32(len(data)>>8), 16)
		w.bits(uint32(len(data)&0xFF), 8)
		w.out = append(w.bytes(), 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00)
		w.out = append(w.out, data...)
	}
	writeRaw([]byte("ABC"))
	w.out = append(w.bytes(), 0xFF) // pad byte
	writeRaw([]byte("DE"))
	if got, want := decodeAll(t, w.out, 15, 5, 1<<20), []byte("ABCDE"); !bytes.Equal(got, want) {
		t.Fatalf("decode = %q, want %q", got, want)
	}
}

// TestLZXE8 verifies absolute->relative CALL translation and the
// last-10-bytes exclusion.
func TestLZXE8(t *testing.T) {
	build := func(data []byte, filesize uint32) []byte {
		var w lzxTestWriter
		w.bits(1, 1)
		w.bits(filesize>>16, 16)
		w.bits(filesize&0xFFFF, 16)
		w.bits(3, 3)
		w.bits(uint32(len(data)>>8), 16)
		w.bits(uint32(len(data)&0xFF), 8)
		out := append(w.bytes(), 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00)
		return append(out, data...)
	}
	t.Run("translated", func(t *testing.T) {
		// E8 at index 2 (curpos 2), abs 16, filesize 0x100 -> rel 14.
		data := append([]byte{'A', 'B', 0xE8, 0x10, 0x00, 0x00, 0x00}, make([]byte, 16)...)
		want := append([]byte{'A', 'B', 0xE8, 0x0E, 0x00, 0x00, 0x00}, make([]byte, 16)...)
		if got := decodeAll(t, build(data, 0x100), 15, int64(len(data)), 1<<20); !bytes.Equal(got, want) {
			t.Fatalf("decode = %x, want %x", got, want)
		}
	})
	t.Run("negative", func(t *testing.T) {
		// abs -2 (0xFFFFFFFE) with curpos 5, filesize 0x100 -> rel 254.
		data := append([]byte{'A', 'B', 'C', 'D', 'E', 0xE8, 0xFE, 0xFF, 0xFF, 0xFF}, make([]byte, 16)...)
		want := append([]byte{'A', 'B', 'C', 'D', 'E', 0xE8, 0xFE, 0x00, 0x00, 0x00}, make([]byte, 16)...)
		if got := decodeAll(t, build(data, 0x100), 15, int64(len(data)), 1<<20); !bytes.Equal(got, want) {
			t.Fatalf("decode = %x, want %x", got, want)
		}
	})
	t.Run("tail excluded", func(t *testing.T) {
		// E8 within the last 10 bytes is never translated.
		data := append(make([]byte, 12), 0xE8, 0x10, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00)
		if got := decodeAll(t, build(data, 0x100), 15, int64(len(data)), 1<<20); !bytes.Equal(got, data) {
			t.Fatalf("decode = %x, want unchanged %x", got, data)
		}
	})
}

// TestLZXUnknownLength decodes without a declared size.
func TestLZXUnknownLength(t *testing.T) {
	in := []byte{
		0x00, 0x30, 0x10, 0x00,
		0x01, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x00, 0x00,
		0x41,
	}
	if got := decodeAll(t, in, 15, -1, 1<<20); !bytes.Equal(got, []byte{0x41}) {
		t.Fatalf("decode = %x, want 41", got)
	}
}

type countingReader struct {
	r io.Reader
	n int
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += n
	return n, err
}

// TestLZXLazy verifies no input is consumed before the first Read and that
// small reads stream correctly.
func TestLZXLazy(t *testing.T) {
	in := []byte{
		0x00, 0x30, 0x10, 0x00,
		0x01, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x00, 0x00,
		0x41,
	}
	cr := &countingReader{r: bytes.NewReader(in)}
	rc, err := NewLZXReader(cr, 15, 1, 1<<20)
	if err != nil {
		t.Fatalf("NewLZXReader() error: %v", err)
	}
	defer func() { _ = rc.Close() }()
	if cr.n != 0 {
		t.Fatalf("constructor consumed %d input bytes, want 0", cr.n)
	}
	var out []byte
	var one [1]byte
	for {
		n, err := rc.Read(one[:])
		out = append(out, one[:n]...)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read() error: %v", err)
		}
	}
	if !bytes.Equal(out, []byte{0x41}) {
		t.Fatalf("decode = %x, want 41", out)
	}
}

func TestLZXZeroLength(t *testing.T) {
	rc, err := NewLZXReader(bytes.NewReader(nil), 15, 0, 1<<20)
	if err != nil {
		t.Fatalf("NewLZXReader() error: %v", err)
	}
	defer func() { _ = rc.Close() }()
	if _, err := io.ReadAll(rc); err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
}

func TestLZXLimits(t *testing.T) {
	in := []byte{
		0x00, 0x30, 0x10, 0x00,
		0x01, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x00, 0x00,
		0x41,
	}
	t.Run("declared exceeds max eager", func(t *testing.T) {
		if _, err := NewLZXReader(bytes.NewReader(in), 15, 2, 1); err == nil {
			t.Fatal("expected ErrLimitExceeded, got nil")
		} else if !errors.Is(err, book.ErrLimitExceeded) {
			t.Fatalf("error = %v, want ErrLimitExceeded", err)
		}
	})
	t.Run("unknown length exceeds max", func(t *testing.T) {
		big := make([]byte, 40)
		for i := range big {
			big[i] = 0x41
		}
		var w lzxTestWriter
		w.bits(0, 1)
		w.bits(3, 3)
		w.bits(uint32(len(big)>>8), 16)
		w.bits(uint32(len(big)&0xFF), 8)
		out := append(w.bytes(), 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00)
		out = append(out, big...)
		rc2, err := NewLZXReader(bytes.NewReader(out), 15, -1, 10)
		if err != nil {
			t.Fatalf("NewLZXReader() error: %v", err)
		}
		defer func() { _ = rc2.Close() }()
		if _, err := io.ReadAll(rc2); !errors.Is(err, book.ErrLimitExceeded) {
			t.Fatalf("ReadAll() error = %v, want ErrLimitExceeded", err)
		}
	})
}

func TestLZXCorrupt(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		if _, err := NewLZXReader(nil, 15, 1, 1<<20); !errors.Is(err, book.ErrCorrupt) {
			t.Fatalf("error = %v, want ErrCorrupt", err)
		}
	})
	t.Run("truncated", func(t *testing.T) {
		in := []byte{0x00, 0x30}
		rc, err := NewLZXReader(bytes.NewReader(in), 15, 1, 1<<20)
		if err != nil {
			t.Fatalf("NewLZXReader() error: %v", err)
		}
		defer func() { _ = rc.Close() }()
		if _, err := io.ReadAll(rc); !errors.Is(err, book.ErrCorrupt) {
			t.Fatalf("ReadAll() error = %v, want ErrCorrupt", err)
		}
	})
	t.Run("invalid block type", func(t *testing.T) {
		// intel=0 then type 000 (invalid), rest zeros.
		in := bytes.Repeat([]byte{0x00}, 8)
		rc, err := NewLZXReader(bytes.NewReader(in), 15, 10, 1<<20)
		if err != nil {
			t.Fatalf("NewLZXReader() error: %v", err)
		}
		defer func() { _ = rc.Close() }()
		if _, err := io.ReadAll(rc); !errors.Is(err, book.ErrCorrupt) {
			t.Fatalf("ReadAll() error = %v, want ErrCorrupt", err)
		}
	})
	t.Run("empty unknown length", func(t *testing.T) {
		rc, err := NewLZXReader(bytes.NewReader(nil), 15, -1, 1<<20)
		if err != nil {
			t.Fatalf("NewLZXReader() error: %v", err)
		}
		defer func() { _ = rc.Close() }()
		out, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("ReadAll() error: %v", err)
		}
		if len(out) != 0 {
			t.Fatalf("decode len = %d, want 0", len(out))
		}
	})
}

// TestLZXChunksIndependent mirrors the LIT reset-window usage: one reader
// per chunk, concatenated.
func TestLZXChunksIndependent(t *testing.T) {
	mk := func(b byte) []byte {
		return []byte{
			0x00, 0x30, 0x10, 0x00,
			0x01, 0x00, 0x00, 0x00,
			0x01, 0x00, 0x00, 0x00,
			0x01, 0x00, 0x00, 0x00,
			b,
		}
	}
	var out []byte
	for _, b := range []byte("ABCD") {
		got := decodeAll(t, mk(b), 15, 1, 1<<20)
		out = append(out, got...)
	}
	if string(out) != "ABCD" {
		t.Fatalf("decode = %q, want ABCD", out)
	}
}

// TestLZXLength19 covers pretree code 19 (repeated length runs) with a run
// of six identical lengths.
func TestLZXLength19(t *testing.T) {
	var w lzxTestWriter
	w.bits(0, 1)
	mainLens := make([]uint8, 256+30*8)
	mainLens['A'] = 2
	for i := 65; i < 71; i++ {
		mainLens[i] = 4
	}
	lenLens := make([]uint8, 249)
	lenLens[0] = 1
	var elems []lzxTestElem
	elems = append(elems, lzxTestElem{lit: true, b: 'A'})
	for i := 65; i < 71; i++ {
		elems = append(elems, lzxTestElem{lit: true, b: byte(i)})
	}
	w.verbatimBlock(mainLens, lenLens, elems, 7)
	if got, want := decodeAll(t, w.bytes(), 15, 7, 1<<20), []byte("AABCDEF"); !bytes.Equal(got, want) {
		t.Fatalf("decode = %q, want %q", got, want)
	}
}

// TestLZXMultiBlock covers cross-block table deltas and R0 persistence:
// block 2 reuses block 1's repeat offset without setting it.
func TestLZXMultiBlock(t *testing.T) {
	var w lzxTestWriter
	w.bits(0, 1)
	mainLens1 := make([]uint8, 256+30*8)
	mainLens1['A'], mainLens1['B'] = 2, 2
	mainLens1[256+(4<<3)+2] = 2
	lenLens := make([]uint8, 249)
	lenLens[0] = 1
	w.verbatimBlock(mainLens1, lenLens, []lzxTestElem{
		{lit: true, b: 'A'},
		{lit: true, b: 'B'},
		{matchLen: 4, matchSlot: 4, verbatim: 0, verbatimN: 1},
	}, 6)
	mainLens2 := make([]uint8, 256+30*8)
	mainLens2['C'] = 2
	mainLens2[256+(0<<3)+0] = 2
	w.verbatimBlockOld(mainLens2, lenLens, []lzxTestElem{
		{lit: true, b: 'C'},
		{matchLen: 2, matchSlot: 0},
	}, 3, mainLens1, lenLens)
	if got, want := decodeAll(t, w.bytes(), 15, 9, 1<<20), []byte("ABABABCBC"); !bytes.Equal(got, want) {
		t.Fatalf("decode = %q, want %q", got, want)
	}
}

// TestLZXAlignedRSlots covers aligned-block R0/R1/R2 repeat offsets.
func TestLZXAlignedRSlots(t *testing.T) {
	var w lzxTestWriter
	w.bits(0, 1)
	mainLens := make([]uint8, 256+30*8)
	for _, b := range []byte("ABCD") {
		mainLens[b] = 3
	}
	mainLens[256+(5<<3)+2] = 4
	mainLens[256+(0<<3)+2] = 4
	mainLens[256+(1<<3)+0] = 4
	mainLens[256+(2<<3)+0] = 4
	lenLens := make([]uint8, 249)
	lenLens[0] = 1
	alignedLens := []uint8{3, 3, 3, 3, 3, 3, 3, 3}
	var elems []lzxTestElem
	for _, b := range []byte("ABCD") {
		elems = append(elems, lzxTestElem{lit: true, b: b})
	}
	elems = append(elems,
		lzxTestElem{matchLen: 4, matchSlot: 5, verbatim: 0, verbatimN: 1},
		lzxTestElem{matchLen: 4, matchSlot: 0},
		lzxTestElem{matchLen: 2, matchSlot: 1},
		lzxTestElem{matchLen: 2, matchSlot: 2},
	)
	w.alignedBlock(mainLens, lenLens, alignedLens, elems, 16)
	if got, want := decodeAll(t, w.bytes(), 15, 16, 1<<20), []byte("ABCDABCDABCDDDDD"); !bytes.Equal(got, want) {
		t.Fatalf("decode = %q, want %q", got, want)
	}
}

// TestLZXWindowWrap covers matches reaching past the window start after a
// frame wrap, including the two-run edge copy.
func TestLZXWindowWrap(t *testing.T) {
	const windowBits = 15
	const windowSize = 1 << windowBits
	var w lzxTestWriter
	w.bits(0, 1)
	mainLens := make([]uint8, 256+30*8)
	for i := 0; i < 250; i++ {
		mainLens[i] = 8
	}
	lenLens := make([]uint8, 249)
	lenLens[0] = 1
	lits := make([]byte, windowSize)
	var elems []lzxTestElem
	for i := range lits {
		lits[i] = byte(i % 250)
		elems = append(elems, lzxTestElem{lit: true, b: lits[i]})
	}
	w.verbatimBlock(mainLens, lenLens, elems, windowSize)
	slot, vb, _, ok := lzxSlotFor(20000, windowBits<<1)
	if !ok {
		t.Fatal("no slot for offset 20000")
	}
	mainLens2 := make([]uint8, 256+30*8)
	mainLens2[256+(slot<<3)+6] = 8
	mainLens2[256+(7<<3)+6] = 8
	extra := int(lzxExtraBits[slot])
	w.verbatimBlockOld(mainLens2, lenLens, []lzxTestElem{
		{matchLen: 8, matchSlot: slot, verbatim: vb, verbatimN: extra},
		{matchLen: 8, matchSlot: 7, verbatim: uint32(10 - (int(lzxPositionBase[7]) - 2)), verbatimN: int(lzxExtraBits[7])},
	}, 16, mainLens, lenLens)
	want := append(append([]byte(nil), lits...), lits[windowSize-20000:windowSize-20000+8]...)
	m1out := lits[windowSize-20000 : windowSize-20000+8]
	// m2 wraps the window edge: last 2 window bytes, then the bytes m1
	// just wrote at the window start.
	want = append(want, lits[windowSize-2:]...)
	want = append(want, m1out[:6]...)
	if got := decodeAll(t, w.bytes(), windowBits, int64(windowSize+16), 1<<26); !bytes.Equal(got, want) {
		t.Fatalf("decode len = %d, want %d (equal=%v)", len(got), len(want), bytes.Equal(got, want))
	}
}

// TestLZXZeroBlocks covers empty blocks: a lone one is tolerated, a run of
// them is rejected as a crafted-stream guard.
func TestLZXZeroBlocks(t *testing.T) {
	emptyMain := make([]uint8, 256+30*8)
	emptyLen := make([]uint8, 249)
	buildZeros := func(n int) []byte {
		var w lzxTestWriter
		w.bits(0, 1)
		for i := 0; i < n; i++ {
			w.verbatimBlock(emptyMain, emptyLen, nil, 0)
		}
		return w.bytes()
	}
	t.Run("tolerated once", func(t *testing.T) {
		var w lzxTestWriter
		w.bits(0, 1)
		w.verbatimBlock(emptyMain, emptyLen, nil, 0)
		mainLens := make([]uint8, 256+30*8)
		mainLens['Z'] = 1
		w.verbatimBlockOld(mainLens, emptyLen, []lzxTestElem{{lit: true, b: 'Z'}}, 1, emptyMain, emptyLen)
		if got := decodeAll(t, w.bytes(), 15, 1, 1<<20); !bytes.Equal(got, []byte("Z")) {
			t.Fatalf("decode = %q, want Z", got)
		}
	})
	t.Run("run rejected", func(t *testing.T) {
		rc, err := NewLZXReader(bytes.NewReader(buildZeros(17)), 15, 100, 1<<20)
		if err != nil {
			t.Fatalf("NewLZXReader() error: %v", err)
		}
		defer func() { _ = rc.Close() }()
		if _, err := io.ReadAll(rc); !errors.Is(err, book.ErrCorrupt) {
			t.Fatalf("ReadAll() error = %v, want ErrCorrupt", err)
		}
	})
}
