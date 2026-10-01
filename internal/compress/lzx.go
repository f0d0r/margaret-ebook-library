// LZX decompressor.
//
// The decoder is streaming and lazy: NewLZXReader performs no input reads,
// and decompression advances only as the caller reads. Output is bounded by
// maxOut with book.ErrLimitExceeded on overflow.

package compress

import (
	"fmt"
	"io"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
)

const (
	lzxMinWindowBits = 15
	lzxMaxWindowBits = 21

	lzxFrameSize = 32768

	lzxMinMatch = 2
	lzxNumChars = 256

	lzxNumPrimaryLens   = 7
	lzxNumSecondaryLens = 249

	lzxBlockVerbatim     = 1
	lzxBlockAligned      = 2
	lzxBlockUncompressed = 3

	// Direct lookup widths for the Huffman fast path. Symbols with longer
	// codes (up to 16 bits) resolve by walking extension nodes.
	lzxPretreeBits = 6
	lzxMainBits    = 12
	lzxLengthBits  = 12
	lzxAlignedBits = 7
	lzxMaxCodeLen  = 16
	lzxPretreeSyms = 20
	lzxAlignedSyms = 8
)

// lzxExtraBits and lzxPositionBase map a match position slot to its base
// offset and the number of trailing verbatim offset bits, per the LZX slot
// table definition.
var lzxExtraBits [51]uint8
var lzxPositionBase [51]uint32

func init() {
	// Pairs (0,1)..(48,49); slot 50 is never addressed (at most 50 slots).
	for i, j := 0, 0; i < 50; i += 2 {
		lzxExtraBits[i] = uint8(j)
		lzxExtraBits[i+1] = uint8(j)
		if i != 0 && j < 17 {
			j++
		}
	}
	for i, j := 0, 0; i < 51; i++ {
		lzxPositionBase[i] = uint32(j)
		j += 1 << lzxExtraBits[i]
	}
}

// lzxPositionSlots reports how many position slots a window of windowBits
// carries.
func lzxPositionSlots(windowBits int) int {
	switch windowBits {
	case 21:
		return 50
	case 20:
		return 42
	default:
		return windowBits << 1
	}
}

// ---------------------------------------------------------------------------
// Canonical Huffman tables with a direct-lookup fast path.
// ---------------------------------------------------------------------------

// lzxHuffman decodes symbols MSB-first from the bitstream. Codes of
// tableBits or fewer bits resolve with a single table lookup; longer codes
// (up to 16 bits) resolve by walking extension nodes.
type lzxHuffman struct {
	nSyms int
	bits  uint
	lens  []uint8
	table []int32 // len 1<<bits: symbol (>=0), -(node+2), or -1 (unset)
	nodes [][2]int32
	empty bool
}

func newLzxHuffman(nSyms int, tableBits uint, lens []uint8) (*lzxHuffman, error) {
	h := &lzxHuffman{nSyms: nSyms, bits: tableBits, lens: append([]uint8(nil), lens...)}
	empty := true
	for _, l := range lens {
		if l != 0 {
			empty = false
			break
		}
	}
	if empty {
		h.empty = true
		return h, nil
	}
	// Canonical code assignment from the length counts.
	var count [lzxMaxCodeLen + 1]int
	for _, l := range lens {
		if l > lzxMaxCodeLen {
			return nil, fmt.Errorf("compress: lzx code length %d exceeds maximum: %w", l, book.ErrCorrupt)
		}
		count[l]++
	}
	// Canonical code assignment. Zero-length (unused) symbols consume no
	// code space, so count[0] is excluded per the standard algorithm.
	count[0] = 0
	start := make([]int, lzxMaxCodeLen+2)
	code := 0
	for b := 1; b <= lzxMaxCodeLen; b++ {
		code = (code + count[b-1]) << 1
		start[b] = code
	}
	total := 0
	for b := 1; b <= lzxMaxCodeLen; b++ {
		total += count[b] << (lzxMaxCodeLen - b)
	}
	if total > 1<<lzxMaxCodeLen {
		return nil, fmt.Errorf("compress: lzx over-subscribed huffman lengths: %w", book.ErrCorrupt)
	}
	codes := make([]int, nSyms)
	for s := range nSyms {
		if l := lens[s]; l != 0 {
			codes[s] = start[l]
			start[l]++
		}
	}
	h.table = make([]int32, 1<<tableBits)
	for i := range h.table {
		h.table[i] = -1
	}
	newNode := func() int {
		h.nodes = append(h.nodes, [2]int32{-1, -1})
		return len(h.nodes) - 1
	}
	for sym := 0; sym < nSyms; sym++ {
		l := int(lens[sym])
		if l == 0 {
			continue
		}
		c := codes[sym]
		if uint(l) <= tableBits {
			prefix := c << (tableBits - uint(l))
			if prefix+(1<<(tableBits-uint(l))) > len(h.table) {
				return nil, fmt.Errorf("compress: lzx huffman code overflows table: %w", book.ErrCorrupt)
			}
			for i := 0; i < 1<<(tableBits-uint(l)); i++ {
				if h.table[prefix+i] != -1 {
					return nil, fmt.Errorf("compress: lzx duplicate huffman code: %w", book.ErrCorrupt)
				}
				h.table[prefix+i] = int32(sym)
			}
			continue
		}
		// Long code: walk/create extension nodes past the direct prefix.
		idx := c >> (uint(l) - tableBits)
		var node int
		switch e := h.table[idx]; {
		case e == -1:
			node = newNode()
			h.table[idx] = -(int32(node) + 2)
		case e < 0:
			node = int(-(e + 2))
		default:
			return nil, fmt.Errorf("compress: lzx conflicting huffman code: %w", book.ErrCorrupt)
		}
		for b := int(tableBits) + 1; b < l; b++ {
			bit := (c >> (uint(l) - uint(b) - 1)) & 1
			switch next := h.nodes[node][bit]; {
			case next == -1:
				n := newNode()
				h.nodes[node][bit] = -(int32(n) + 2)
				node = n
			case next < 0:
				node = int(-(next + 2))
			default:
				return nil, fmt.Errorf("compress: lzx conflicting huffman code: %w", book.ErrCorrupt)
			}
		}
		if bit := c & 1; h.nodes[node][bit] != -1 {
			return nil, fmt.Errorf("compress: lzx duplicate huffman code: %w", book.ErrCorrupt)
		} else {
			h.nodes[node][bit] = int32(sym)
		}
	}
	return h, nil
}

// ---------------------------------------------------------------------------
// Bit reader: MSB-first symbols out of 16-bit little-endian units.
// ---------------------------------------------------------------------------

// lzxBitReader feeds the decoder pulling byte pairs from src on demand.
// Past the end of src it yields up to 16 zero padding bits once (covering
// overshooting 16-bit reads) and reports truncation afterwards.
type lzxBitReader struct {
	src      io.Reader
	pending  []byte
	pos      int
	eof      bool
	padGiven bool
	hold     uint32
	nbits    int
	// realBits counts buffered bits backed by real input. Padding bits are
	// always the newest, so consumption eats real bits first.
	realBits int
	// lastPair remembers the most recently pulled byte pair so the
	// uncompressed-block path can hand it back for raw consumption.
	lastPair [2]byte
	hasPair  bool
}

func (b *lzxBitReader) fill() error {
	if b.pos > 0 {
		b.pending = append([]byte(nil), b.pending[b.pos:]...)
		b.pos = 0
	}
	var tmp [4096]byte
	n, err := b.src.Read(tmp[:])
	if n > 0 {
		b.pending = append(b.pending, tmp[:n]...)
		return nil
	}
	if err == io.EOF {
		b.eof = true
		return nil
	}
	return err
}

// ensure makes n bits available, loading input as needed.
func (b *lzxBitReader) ensure(n int) error {
	for b.nbits < n {
		if b.pos+2 > len(b.pending) {
			if b.eof {
				if b.padGiven {
					return fmt.Errorf("compress: lzx truncated bitstream: %w", book.ErrCorrupt)
				}
				b.padGiven = true
				b.nbits += 16
				continue
			}
			if err := b.fill(); err != nil {
				return err
			}
			continue
		}
		w := uint32(b.pending[b.pos]) | uint32(b.pending[b.pos+1])<<8
		b.lastPair[0], b.lastPair[1] = b.pending[b.pos], b.pending[b.pos+1]
		b.hasPair = true
		b.pos += 2
		b.hold |= w << (32 - 16 - b.nbits)
		b.nbits += 16
		b.realBits += 16
	}
	return nil
}

func (b *lzxBitReader) peek(n int) uint32 {
	return b.hold >> (32 - n)
}

func (b *lzxBitReader) consume(n int) {
	b.hold <<= n
	b.nbits -= n
	b.realBits -= min(n, b.realBits)
}

func (b *lzxBitReader) bits(n int) (uint32, error) {
	if err := b.ensure(n); err != nil {
		return 0, err
	}
	v := b.peek(n)
	b.consume(n)
	return v, nil
}

// cleanEnd reports whether the source is exhausted with no real bits left
// buffered: a stream may only end cleanly on a block boundary.
func (b *lzxBitReader) cleanEnd() bool {
	return b.eof && b.realBits == 0 && b.pos >= len(b.pending)
}

// alignToByte drops buffered bits for the uncompressed-block path,
// returning the over-read trailing byte pair to the pending queue.
func (b *lzxBitReader) alignToByte() {
	if b.nbits > 16 && b.hasPair {
		rest := append([]byte(nil), b.pending[b.pos:]...)
		b.pending = append(b.lastPair[:], rest...)
		b.pos = 0
	}
	b.hold, b.nbits, b.realBits = 0, 0, 0
	b.hasPair = false
}

// alignToUnit discards leftover bits up to the next 16-bit boundary,
// pulling real input first so padding (not real header bits) is dropped.
func (b *lzxBitReader) alignToUnit() error {
	if b.nbits > 0 {
		if err := b.ensure(16); err != nil {
			return err
		}
		if r := b.nbits & 15; r != 0 {
			b.consume(r)
		}
	}
	return nil
}

// readRaw copies n bytes straight from the byte stream: pending bytes
// first, then directly from src.
func (b *lzxBitReader) readRaw(dst []byte) error {
	for len(dst) > 0 {
		if b.pos < len(b.pending) {
			n := copy(dst, b.pending[b.pos:])
			b.pos += n
			dst = dst[n:]
			continue
		}
		b.pos, b.pending = 0, b.pending[:0]
		n, err := b.src.Read(dst)
		dst = dst[n:]
		if len(dst) == 0 {
			if err != nil && err != io.EOF {
				return err
			}
			return nil
		}
		if err == io.EOF {
			b.eof = true
			return fmt.Errorf("compress: lzx truncated raw block: %w", book.ErrCorrupt)
		}
		if err != nil {
			return err
		}
		// n == 0 with err == nil: a reader may do this; retry.
	}
	return nil
}

func (b *lzxBitReader) skipRaw(n int) error {
	var tmp [512]byte
	for n > 0 {
		m := min(n, len(tmp))
		if err := b.readRaw(tmp[:m]); err != nil {
			return err
		}
		n -= m
	}
	return nil
}

// ---------------------------------------------------------------------------
// Streaming LZX decoder state machine.
// ---------------------------------------------------------------------------

// lzxDecoder holds the full state of one LZX chunk stream. Fresh R0-R2,
// zeroed Huffman lengths and an empty window match one reference-decode
// call, which is exactly how LIT reset-window chunks are consumed.
type lzxDecoder struct {
	bits       *lzxBitReader
	windowSize int
	window     []byte
	windowPosn int
	framePosn  int
	frame      uint32

	expectOut int64 // <0: unknown length, stop at clean input end
	emitted   int64
	maxOut    int64

	posnSlots int

	mainLen   []uint8
	lengthLen []uint8
	main      *lzxHuffman
	length    *lzxHuffman
	aligned   *lzxHuffman

	r0, r1, r2 uint32

	blockType    int
	blockRemain  int
	needPadSkip  bool // previous block was odd-sized uncompressed
	headerSeen   bool
	intelSize    int32
	intelPos     int32
	intelStarted bool

	staged    []byte // staged, E8-processed frame output
	stagedPos int
	finished  bool
	limitErr  error
}

// NewLZXReader returns a streaming LZX decompressor over in.
//
// windowBits must be in [15,21] and selects the 2^windowBits history window.
// expectOut is the exact decompressed byte count when known (always known
// for LIT reset-window chunks via the reset table); pass a negative value
// to decode until a clean input end on a block boundary instead. maxOut
// bounds the total output (use the format reader's MaxResourceSize); a
// non-positive maxOut resolves to the library default. Streams promising or
// producing more than maxOut fail with book.ErrLimitExceeded.
//
// No input is consumed until the first Read. The returned reader must be
// closed by the caller; Close does not close in.
func NewLZXReader(in io.Reader, windowBits int, expectOut int64, maxOut int64) (io.ReadCloser, error) {
	if in == nil {
		return nil, fmt.Errorf("compress: lzx nil input: %w", book.ErrCorrupt)
	}
	if windowBits < lzxMinWindowBits || windowBits > lzxMaxWindowBits {
		return nil, fmt.Errorf("compress: lzx window bits %d out of range [%d,%d]: %w",
			windowBits, lzxMinWindowBits, lzxMaxWindowBits, book.ErrCorrupt)
	}
	if maxOut <= 0 {
		maxOut = config.DefaultConfig().MaxResourceSize
	}
	if expectOut > maxOut {
		return nil, fmt.Errorf("compress: lzx declares %d bytes (limit %d): %w", expectOut, maxOut, book.ErrLimitExceeded)
	}
	slots := lzxPositionSlots(windowBits)
	d := &lzxDecoder{
		bits:       &lzxBitReader{src: in},
		windowSize: 1 << windowBits,
		expectOut:  expectOut,
		maxOut:     maxOut,
		posnSlots:  slots,
		blockType:  -1,
		r0:         1,
		r1:         1,
		r2:         1,
	}
	d.window = make([]byte, d.windowSize)
	d.mainLen = make([]uint8, lzxNumChars+(slots<<3))
	d.lengthLen = make([]uint8, lzxNumSecondaryLens+1)
	if expectOut == 0 {
		d.finished = true
	}
	return d, nil
}

func (d *lzxDecoder) Close() error {
	d.finished = true
	d.staged, d.stagedPos = nil, 0
	return nil
}

func (d *lzxDecoder) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for {
		if d.limitErr != nil {
			return 0, d.limitErr
		}
		if d.stagedPos < len(d.staged) {
			n := copy(p, d.staged[d.stagedPos:])
			d.stagedPos += n
			return n, nil
		}
		if d.finished {
			return 0, io.EOF
		}
		if err := d.decodeFrame(); err != nil {
			if err == io.EOF {
				d.finished = true
				continue
			}
			return 0, err
		}
	}
}

// frameBudget reports how many output bytes the next frame may hold.
func (d *lzxDecoder) frameBudget() int {
	if d.expectOut >= 0 {
		if left := d.expectOut - d.emitted; left < lzxFrameSize {
			return int(left)
		}
	}
	return lzxFrameSize
}

// decodeFrame decodes one 32KB frame (or the final partial frame) and
// stages its E8-processed bytes for Read.
func (d *lzxDecoder) decodeFrame() error {
	if d.expectOut >= 0 && d.emitted >= d.expectOut {
		return io.EOF
	}
	if !d.headerSeen {
		if err := d.readStreamHeader(); err != nil {
			return err
		}
	}
	frameSize := d.frameBudget()
	if frameSize <= 0 {
		return io.EOF
	}
	frameStart := d.windowPosn
	bytesTodo := frameSize
	for bytesTodo > 0 {
		if d.blockRemain == 0 {
			done, err := d.readBlockHeader()
			if err != nil {
				return err
			}
			if done {
				break
			}
		}
		run := min(d.blockRemain, bytesTodo)
		bytesTodo -= run
		d.blockRemain -= run
		over, err := d.decodeRun(run)
		if err != nil {
			return err
		}
		if over < 0 {
			if -over > d.blockRemain {
				return fmt.Errorf("compress: lzx match overran block end: %w", book.ErrCorrupt)
			}
			d.blockRemain += over // over is negative
		}
	}
	produced := d.windowPosn - frameStart
	if produced == 0 {
		// Unknown-length streams end here; known-length streams that
		// stop short are truncated.
		if d.expectOut >= 0 && d.emitted < d.expectOut {
			return fmt.Errorf("compress: lzx truncated stream: %w", book.ErrCorrupt)
		}
		return io.EOF
	}
	if err := d.bits.alignToUnit(); err != nil {
		return err
	}
	return d.stageFrame(frameStart, produced)
}

// stageFrame copies a decoded frame out of the window, applies the E8
// transform when active, and stages the result for Read. The window keeps
// the untransformed bytes so later matches see the true history.
func (d *lzxDecoder) stageFrame(start, size int) error {
	if d.emitted+int64(size) > d.maxOut {
		d.finished = true
		d.staged, d.stagedPos = nil, 0
		d.limitErr = fmt.Errorf("compress: lzx output exceeds %d bytes: %w", d.maxOut, book.ErrLimitExceeded)
		return d.limitErr
	}
	out := make([]byte, size)
	copy(out, d.window[start:start+size])
	if d.intelStarted && d.intelSize != 0 && d.frame <= 32768 && size > 10 {
		pos := d.intelPos
		size32 := d.intelSize
		end := size - 10
		for i := 0; i < end; {
			if out[i] != 0xE8 {
				i++
				pos++
				continue
			}
			abs := int32(uint32(out[i+1]) | uint32(out[i+2])<<8 | uint32(out[i+3])<<16 | uint32(out[i+4])<<24)
			if abs >= -pos && abs < size32 {
				var rel int32
				if abs >= 0 {
					rel = abs - pos
				} else {
					rel = abs + size32
				}
				out[i+1] = byte(rel)
				out[i+2] = byte(rel >> 8)
				out[i+3] = byte(rel >> 16)
				out[i+4] = byte(rel >> 24)
			}
			i += 5
			pos += 5
		}
		d.intelPos += int32(size)
	} else if d.intelSize != 0 {
		d.intelPos += int32(size)
	}
	d.staged, d.stagedPos = out, 0
	d.emitted += int64(size)
	d.frame++
	if d.windowPosn == d.windowSize {
		d.windowPosn = 0
	}
	d.framePosn += size
	if d.framePosn == d.windowSize {
		d.framePosn = 0
	}
	return nil
}

// readStreamHeader reads the one-bit E8 prelude plus the optional 32-bit
// file size. It runs once per stream.
func (d *lzxDecoder) readStreamHeader() error {
	d.headerSeen = true
	flag, err := d.bits.bits(1)
	if err != nil {
		if d.expectOut < 0 && d.bits.cleanEnd() {
			// Empty stream with unknown length: zero bytes, clean end.
			d.finished = true
			return io.EOF
		}
		return err
	}
	if flag == 0 {
		return nil
	}
	hi, err := d.bits.bits(16)
	if err != nil {
		return err
	}
	lo, err := d.bits.bits(16)
	if err != nil {
		return err
	}
	d.intelSize = int32(hi<<16 | lo)
	return nil
}

// readBlockHeader parses the next block header when the current block is
// drained. It reports done=true when the input ends cleanly on a block
// boundary (only possible with unknown output length).
func (d *lzxDecoder) readBlockHeader() (done bool, err error) {
	if d.needPadSkip {
		d.needPadSkip = false
		if err := d.bits.skipRaw(1); err != nil {
			// A missing pad byte at a clean input end is not corruption.
			if d.expectOut < 0 && d.bits.cleanEnd() {
				return true, nil
			}
			return false, err
		}
	}
	if d.bits.cleanEnd() {
		if d.expectOut >= 0 {
			return false, fmt.Errorf("compress: lzx truncated stream: %w", book.ErrCorrupt)
		}
		return true, nil
	}
	// The pending queue may be drained while src EOF is not yet observed
	// (raw block bodies bypass the bit buffer). Probe once so a clean
	// input end is recognized instead of decoding padding zeros as a
	// bogus header.
	if d.bits.pos >= len(d.bits.pending) && d.bits.realBits == 0 && !d.bits.eof {
		if err := d.bits.fill(); err != nil {
			return false, err
		}
		if d.bits.cleanEnd() {
			if d.expectOut >= 0 {
				return false, fmt.Errorf("compress: lzx truncated stream: %w", book.ErrCorrupt)
			}
			return true, nil
		}
	}
	t, err := d.bits.bits(3)
	if err != nil {
		return false, err
	}
	hi, err := d.bits.bits(16)
	if err != nil {
		return false, err
	}
	lo, err := d.bits.bits(8)
	if err != nil {
		return false, err
	}
	length := int(hi<<8 | lo)
	d.blockType = int(t)
	d.blockRemain = length
	switch d.blockType {
	case lzxBlockAligned:
		al := make([]uint8, lzxAlignedSyms)
		for i := range al {
			v, err := d.bits.bits(3)
			if err != nil {
				return false, err
			}
			al[i] = uint8(v)
		}
		if d.aligned, err = newLzxHuffman(lzxAlignedSyms, lzxAlignedBits, al); err != nil {
			return false, err
		}
		fallthrough
	case lzxBlockVerbatim:
		if err := d.readLens(d.mainLen, 0, 256); err != nil {
			return false, err
		}
		if err := d.readLens(d.mainLen, 256, lzxNumChars+(d.posnSlots<<3)); err != nil {
			return false, err
		}
		if d.main, err = newLzxHuffman(len(d.mainLen), lzxMainBits, d.mainLen); err != nil {
			return false, err
		}
		if d.mainLen[0xE8] != 0 {
			d.intelStarted = true
		}
		if err := d.readLens(d.lengthLen, 0, lzxNumSecondaryLens+1); err != nil {
			return false, err
		}
		if d.length, err = newLzxHuffman(len(d.lengthLen), lzxLengthBits, d.lengthLen); err != nil {
			return false, err
		}
	case lzxBlockUncompressed:
		d.intelStarted = true
		d.bits.alignToByte()
		var hdr [12]byte
		if err := d.bits.readRaw(hdr[:]); err != nil {
			return false, err
		}
		d.r0 = uint32(hdr[0]) | uint32(hdr[1])<<8 | uint32(hdr[2])<<16 | uint32(hdr[3])<<24
		d.r1 = uint32(hdr[4]) | uint32(hdr[5])<<8 | uint32(hdr[6])<<16 | uint32(hdr[7])<<24
		d.r2 = uint32(hdr[8]) | uint32(hdr[9])<<8 | uint32(hdr[10])<<16 | uint32(hdr[11])<<24
		d.needPadSkip = length&1 == 1
	default:
		return false, fmt.Errorf("compress: lzx invalid block type %d: %w", d.blockType, book.ErrCorrupt)
	}
	return false, nil
}

// readLens reads Huffman code lengths for symbols [first,last) using the
// LZX pretree delta encoding. Bounds are enforced: corrupt runs fail with
// book.ErrCorrupt instead of panicking.
func (d *lzxDecoder) readLens(lens []uint8, first, last int) error {
	pre := make([]uint8, lzxPretreeSyms)
	for i := range pre {
		v, err := d.bits.bits(4)
		if err != nil {
			return err
		}
		pre[i] = uint8(v)
	}
	pt, err := newLzxHuffman(lzxPretreeSyms, lzxPretreeBits, pre)
	if err != nil {
		return err
	}
	for x := first; x < last; {
		z, err := d.decodeSym(pt)
		if err != nil {
			return err
		}
		switch z {
		case 17:
			n, err := d.bits.bits(4)
			if err != nil {
				return err
			}
			n += 4
			if x+int(n) > last {
				return fmt.Errorf("compress: lzx length run overflows table: %w", book.ErrCorrupt)
			}
			for ; n > 0; n-- {
				lens[x] = 0
				x++
			}
		case 18:
			n, err := d.bits.bits(5)
			if err != nil {
				return err
			}
			n += 20
			if x+int(n) > last {
				return fmt.Errorf("compress: lzx length run overflows table: %w", book.ErrCorrupt)
			}
			for ; n > 0; n-- {
				lens[x] = 0
				x++
			}
		case 19:
			n, err := d.bits.bits(1)
			if err != nil {
				return err
			}
			n += 4
			y, err := d.decodeSym(pt)
			if err != nil {
				return err
			}
			v := int(lens[x]) - y
			if v < 0 {
				v += 17
			}
			if v > 16 {
				return fmt.Errorf("compress: lzx invalid code length %d: %w", v, book.ErrCorrupt)
			}
			if x+int(n) > last {
				return fmt.Errorf("compress: lzx length run overflows table: %w", book.ErrCorrupt)
			}
			for ; n > 0; n-- {
				lens[x] = uint8(v)
				x++
			}
		default:
			v := int(lens[x]) - z
			if v < 0 {
				v += 17
			}
			if v > 16 {
				return fmt.Errorf("compress: lzx invalid code length %d: %w", v, book.ErrCorrupt)
			}
			lens[x] = uint8(v)
			x++
		}
	}
	return nil
}

func (d *lzxDecoder) decodeSym(h *lzxHuffman) (int, error) {
	if h == nil || h.empty {
		return 0, fmt.Errorf("compress: lzx symbol from empty table: %w", book.ErrCorrupt)
	}
	if err := d.bits.ensure(int(h.bits)); err != nil {
		return 0, err
	}
	if e := h.table[d.bits.peek(int(h.bits))]; e >= 0 {
		d.bits.consume(int(h.lens[e]))
		return int(e), nil
	} else if e == -1 {
		return 0, fmt.Errorf("compress: lzx unset huffman entry: %w", book.ErrCorrupt)
	} else {
		d.bits.consume(int(h.bits))
		node := int(-(e + 2))
		for {
			b, err := d.bits.bits(1)
			if err != nil {
				return 0, err
			}
			if node < 0 || node >= len(h.nodes) {
				return 0, fmt.Errorf("compress: lzx huffman node out of range: %w", book.ErrCorrupt)
			}
			switch e = h.nodes[node][b]; {
			case e >= 0:
				return int(e), nil
			case e == -1:
				return 0, fmt.Errorf("compress: lzx invalid huffman path: %w", book.ErrCorrupt)
			default:
				node = int(-(e + 2))
			}
		}
	}
}

// decodeRun decodes up to run output bytes of the current block. It returns
// the overrun (negative when the final match extends past run while staying
// within the block) so the caller can reconcile the block budget.
func (d *lzxDecoder) decodeRun(run int) (int, error) {
	switch d.blockType {
	case lzxBlockVerbatim:
		return d.decodeMatches(run, false)
	case lzxBlockAligned:
		return d.decodeMatches(run, true)
	case lzxBlockUncompressed:
		if d.windowPosn+run > d.windowSize {
			return 0, fmt.Errorf("compress: lzx raw block overran window: %w", book.ErrCorrupt)
		}
		if err := d.bits.readRaw(d.window[d.windowPosn : d.windowPosn+run]); err != nil {
			return 0, err
		}
		d.windowPosn += run
		return 0, nil
	}
	return 0, fmt.Errorf("compress: lzx invalid block type %d: %w", d.blockType, book.ErrCorrupt)
}

func (d *lzxDecoder) decodeMatches(run int, aligned bool) (int, error) {
	start := d.windowPosn
	for d.windowPosn-start < run {
		el, err := d.decodeSym(d.main)
		if err != nil {
			return 0, err
		}
		if el < lzxNumChars {
			if d.windowPosn >= d.windowSize {
				return 0, fmt.Errorf("compress: lzx literal overran window: %w", book.ErrCorrupt)
			}
			d.window[d.windowPosn] = byte(el)
			d.windowPosn++
			continue
		}
		el -= lzxNumChars
		matchLen := el & lzxNumPrimaryLens
		if matchLen == lzxNumPrimaryLens {
			foot, err := d.decodeSym(d.length)
			if err != nil {
				return 0, err
			}
			matchLen += foot
		}
		matchLen += lzxMinMatch
		slot := uint32(el >> 3)
		var offset uint32
		switch {
		case !aligned && slot < 4:
			switch slot {
			case 0:
				offset = d.r0
			case 1:
				offset = d.r1
				d.r1, d.r0 = d.r0, offset
			case 2:
				offset = d.r2
				d.r2, d.r0 = d.r0, offset
			default: // 3
				offset = 1
				d.r2, d.r1, d.r0 = d.r1, d.r0, 1
			}
		case aligned && slot < 3:
			switch slot {
			case 0:
				offset = d.r0
			case 1:
				offset = d.r1
				d.r1, d.r0 = d.r0, offset
			default: // 2
				offset = d.r2
				d.r2, d.r0 = d.r0, offset
			}
		default:
			if int(slot) >= len(lzxExtraBits) {
				return 0, fmt.Errorf("compress: lzx position slot %d out of range: %w", slot, book.ErrCorrupt)
			}
			extra := lzxExtraBits[slot]
			offset = lzxPositionBase[slot] - 2
			if aligned {
				switch {
				case extra > 3:
					vb, err := d.bits.bits(int(extra - 3))
					if err != nil {
						return 0, err
					}
					offset += vb << 3
					ab, err := d.decodeSym(d.aligned)
					if err != nil {
						return 0, err
					}
					offset += uint32(ab)
				case extra == 3:
					ab, err := d.decodeSym(d.aligned)
					if err != nil {
						return 0, err
					}
					offset += uint32(ab)
				case extra > 0:
					vb, err := d.bits.bits(int(extra))
					if err != nil {
						return 0, err
					}
					offset += vb
				default:
					offset = 1
				}
			} else {
				vb, err := d.bits.bits(int(extra))
				if err != nil {
					return 0, err
				}
				offset += vb
			}
			d.r2, d.r1, d.r0 = d.r1, d.r0, offset
		}
		if offset < 1 || offset > uint32(d.windowSize) {
			return 0, fmt.Errorf("compress: lzx match offset %d out of window: %w", offset, book.ErrCorrupt)
		}
		if d.windowPosn+matchLen > d.windowSize {
			return 0, fmt.Errorf("compress: lzx match ran over window wrap: %w", book.ErrCorrupt)
		}
		// Copy with wrap-around; overlapping copies re-read as they grow.
		dest := d.windowPosn
		if int(offset) > dest {
			j := int(offset) - dest
			src := d.windowSize - j
			for matchLen > 0 && j > 0 {
				d.window[dest] = d.window[src]
				dest++
				src++
				matchLen--
				j--
			}
			src = 0
			for matchLen > 0 {
				d.window[dest] = d.window[src]
				dest++
				src++
				matchLen--
			}
		} else {
			src := dest - int(offset)
			for matchLen > 0 {
				d.window[dest] = d.window[src]
				dest++
				src++
				matchLen--
			}
		}
		d.windowPosn = dest
	}
	return run - (d.windowPosn - start), nil
}
