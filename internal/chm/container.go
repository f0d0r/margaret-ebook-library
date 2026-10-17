package chm

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math/bits"
	"sync"

	"golang.org/x/net/html"

	"github.com/f0d0r/margaret-ebook-library/book"
	compress "github.com/f0d0r/margaret-ebook-library/internal/compress"
	"github.com/f0d0r/margaret-ebook-library/internal/util"
)

// Container layout constants. Offsets are into the ITSF/ITSP headers;
// multi-byte integers are little-endian. See chm_lib.c: chmItsfHeader,
// chmItspHeader, chmPmglHeader, chmPmgiHeader.
const (
	itsfMagic = "ITSF"

	itsfPreamble = 12

	itsfV2Len = 0x58
	itsfV3Len = 0x60

	itspLen = 0x54

	pmglLen = 0x14
	pmgiLen = 0x08

	lzxcResetTableLen = 0x28
	lzxcControlMinLen = 0x18

	// turnCacheMax bounds decoded turn retention; maxCachedTurn skips
	// oversized turns so one huge interval cannot crowd out the small ones.
	turnCacheMax  = 8
	maxCachedTurn = 4 << 20

	maxPathLen = 512
	maxDirWalk = 1 << 20
)

// Control entry paths needed for LZX decompression. See chm_lib.c:
// _CHMU_RESET_TABLE, _CHMU_LZXC_CONTROLDATA, _CHMU_CONTENT.
const (
	resetTablePath = "::DataSpace/Storage/MSCompressed/Transform/{7FC28940-9D31-11D0-9B27-00A0C91E9C7C}/InstanceData/ResetTable"
	controlPath    = "::DataSpace/Storage/MSCompressed/ControlData"
	contentPath    = "::DataSpace/Storage/MSCompressed/Content"
)

// entry maps an internal path to its payload slice.
type entry struct {
	space  uint64
	start  uint64
	length uint64
}

// compParams holds the parsed LZX control structures.
type compParams struct {
	blockLen      uint64
	blockCount    uint64
	tableOff      uint64
	rtStart       uint64
	cnStart       uint64
	uncompLen     uint64
	compLen       uint64
	blocksPerTurn uint64
	windowBits    int
}

// container is a parsed CHM header/directory over a Blob. Payloads assemble
// lazily through getFile; only metadata-sized pieces materialize during Read.
// Decoded LZX turns, the parsed #SYSTEM and the default topic document are
// memoized under mu so repeated and concurrent reads share them.
type container struct {
	blob    book.Blob
	size    int64
	maxSize int64

	version  string
	dirOff   int64
	dirLen   int64
	dataOff  int64
	blkLen   int64
	indexRt  int64
	indexHd  int64
	comp     *compParams
	compErr  error
	compDone bool

	mu        sync.Mutex
	sysDone   bool
	sysCache  *system
	sysErr    error
	topicDone bool
	topicRaw  []byte
	topicDoc  *html.Node
	topicErr  error
	turns     map[uint64][]byte
	turnOrder []uint64
}

// openContainer parses the ITSF and ITSP headers. Decompression parameters
// load lazily on the first compressed entry read.
func openContainer(b book.Blob, maxSize int64) (*container, error) {
	size, err := b.Size()
	if err != nil {
		return nil, fmt.Errorf("chm: failed to stat: %w", err)
	}
	if size < itsfPreamble {
		return nil, fmt.Errorf("chm: file too small: %w", book.ErrCorrupt)
	}

	head := make([]byte, itsfPreamble)
	if err := util.ReadAtFull(b, head, 0); err != nil {
		return nil, fmt.Errorf("chm: failed to read header: %w", book.ErrCorrupt)
	}
	if string(head[:4]) != itsfMagic {
		return nil, fmt.Errorf("chm: bad magic: %w", book.ErrCorrupt)
	}
	ver := binary.LittleEndian.Uint32(head[4:])
	headerLen := int64(binary.LittleEndian.Uint32(head[8:]))
	var wantLen int64
	switch ver {
	case 2:
		wantLen = itsfV2Len
	case 3:
		wantLen = itsfV3Len
	default:
		return nil, fmt.Errorf("chm: unsupported version %d: %w", ver, book.ErrCorrupt)
	}
	if headerLen < wantLen || headerLen > size {
		return nil, fmt.Errorf("chm: bad header length %d: %w", headerLen, book.ErrCorrupt)
	}

	full := make([]byte, headerLen)
	if err := util.ReadAtFull(b, full, 0); err != nil {
		return nil, fmt.Errorf("chm: failed to read header: %w", book.ErrCorrupt)
	}
	dirOff := int64(binary.LittleEndian.Uint64(full[72:]))
	dirLen := int64(binary.LittleEndian.Uint64(full[80:]))
	var dataOff int64
	if ver == 3 {
		dataOff = int64(binary.LittleEndian.Uint64(full[88:]))
	} else {
		dataOff = dirOff + dirLen
	}
	if dirOff < 0 || dirLen < itspLen || dirOff+dirLen > size || dataOff < 0 || dataOff > size {
		return nil, fmt.Errorf("chm: header offsets out of bounds: %w", book.ErrCorrupt)
	}

	ih := make([]byte, itspLen)
	if err := util.ReadAtFull(b, ih, dirOff); err != nil {
		return nil, fmt.Errorf("chm: failed to read directory header: %w", book.ErrCorrupt)
	}
	if string(ih[:4]) != "ITSP" {
		return nil, fmt.Errorf("chm: bad directory magic: %w", book.ErrCorrupt)
	}
	if v := binary.LittleEndian.Uint32(ih[4:]); v != 1 {
		return nil, fmt.Errorf("chm: unsupported directory version %d: %w", v, book.ErrCorrupt)
	}
	if l := int64(binary.LittleEndian.Uint32(ih[8:])); l != itspLen {
		return nil, fmt.Errorf("chm: bad directory length %d: %w", l, book.ErrCorrupt)
	}
	blkLen := int64(binary.LittleEndian.Uint32(ih[16:]))
	indexRt := int64(int32LE(ih[28:]))
	indexHd := int64(int32LE(ih[32:]))
	if blkLen < pmglLen || dirLen-itspLen < blkLen {
		return nil, fmt.Errorf("chm: bad directory block length %d: %w", blkLen, book.ErrCorrupt)
	}
	if indexRt < -1 || indexHd < 0 {
		return nil, fmt.Errorf("chm: bad directory roots: %w", book.ErrCorrupt)
	}
	if indexRt == -1 {
		indexRt = indexHd
	}

	return &container{
		blob:    b,
		size:    size,
		maxSize: maxSize,
		version: fmt.Sprint(ver),
		dirOff:  dirOff + itspLen,
		dirLen:  dirLen - itspLen,
		dataOff: dataOff,
		blkLen:  blkLen,
		indexRt: indexRt,
		indexHd: indexHd,
		turns:   map[uint64][]byte{},
	}, nil
}

// getFile returns the bytes of an internal entry, decompressing LZX sections
// on demand.
func (c *container) getFile(name string) ([]byte, error) {
	e, err := c.resolve(name)
	if err != nil {
		return nil, err
	}
	return c.readEntry(name, e)
}

// readBounded reads size bytes at offset, enforcing maxSize.
func (c *container) readBounded(offset, size int64, what string) ([]byte, error) {
	if size > c.maxSize {
		return nil, book.LimitError(what, size, c.maxSize)
	}
	data := make([]byte, size)
	if err := util.ReadAtFull(c.blob, data, offset); err != nil {
		return nil, fmt.Errorf("chm: short read: %w", book.ErrCorrupt)
	}
	return data, nil
}

func (c *container) readEntry(name string, e entry) ([]byte, error) {
	if e.length > uint64(c.maxSize) {
		return nil, book.LimitError("entry "+name, int64(min(e.length, 1<<62)), c.maxSize)
	}
	switch e.space {
	case 0:
		off := c.dataOff + int64(e.start)
		if int64(e.start) < 0 || off+int64(e.length) > c.size {
			return nil, fmt.Errorf("chm: entry %q out of bounds: %w", name, book.ErrCorrupt)
		}
		return c.readBounded(off, int64(e.length), "entry "+name)
	case 1:
		return c.readCompressed(name, e)
	default:
		return nil, fmt.Errorf("chm: entry %q in unknown section %d: %w", name, e.space, book.ErrCorrupt)
	}
}

// resolve walks the PMGI index from the root to the PMGL leaf holding name.
// Comparison is ASCII case-insensitive, mirroring chmlib strcasecmp.
func (c *container) resolve(name string) (entry, error) {
	page := c.indexRt
	pageBuf := make([]byte, c.blkLen)
	for i := 0; i < maxDirWalk; i++ {
		off := c.dirOff + page*c.blkLen
		if page < 0 || off+c.blkLen > c.dirOff+c.dirLen {
			return entry{}, fmt.Errorf("chm: directory page out of bounds: %w", book.ErrCorrupt)
		}
		if err := util.ReadAtFull(c.blob, pageBuf, off); err != nil {
			return entry{}, fmt.Errorf("chm: failed to read directory page: %w", book.ErrCorrupt)
		}
		switch string(pageBuf[:4]) {
		case "PMGL":
			e, err := scanPMGL(pageBuf, name)
			if err != nil {
				return entry{}, err
			}
			return checkCoords(name, e, c.size)
		case "PMGI":
			next, err := scanPMGI(pageBuf, name)
			if err != nil {
				return entry{}, err
			}
			if next < 0 {
				return entry{}, fmt.Errorf("chm: missing entry %q: %w", name, book.ErrCorrupt)
			}
			page = next
		default:
			return entry{}, fmt.Errorf("chm: bad directory page magic: %w", book.ErrCorrupt)
		}
	}
	return entry{}, fmt.Errorf("chm: directory walk did not terminate: %w", book.ErrCorrupt)
}

// resolveRaw walks the PMGI index like resolve but skips coordinate checks.
// Only the LZX control entries use it: their stored lengths are unreliable
// (the Content length may exceed the file), and each read below is bounded
// explicitly.
func (c *container) resolveRaw(name string) (entry, error) {
	page := c.indexRt
	pageBuf := make([]byte, c.blkLen)
	for i := 0; i < maxDirWalk; i++ {
		off := c.dirOff + page*c.blkLen
		if page < 0 || off+c.blkLen > c.dirOff+c.dirLen {
			return entry{}, fmt.Errorf("chm: directory page out of bounds: %w", book.ErrCorrupt)
		}
		if err := util.ReadAtFull(c.blob, pageBuf, off); err != nil {
			return entry{}, fmt.Errorf("chm: failed to read directory page: %w", book.ErrCorrupt)
		}
		switch string(pageBuf[:4]) {
		case "PMGL":
			return scanPMGL(pageBuf, name)
		case "PMGI":
			next, err := scanPMGI(pageBuf, name)
			if err != nil {
				return entry{}, err
			}
			if next < 0 {
				return entry{}, fmt.Errorf("chm: missing entry %q: %w", name, book.ErrCorrupt)
			}
			page = next
		default:
			return entry{}, fmt.Errorf("chm: bad directory page magic: %w", book.ErrCorrupt)
		}
	}
	return entry{}, fmt.Errorf("chm: directory walk did not terminate: %w", book.ErrCorrupt)
}

// checkCoords rejects absurd entry coordinates before any arithmetic on
// them, so wrapped sums can never pass the later bounds checks. Compressed
// entries address the uncompressed content stream, which may exceed the
// file size, so only section validity is checked for them here; stream
// bounds are enforced in readCompressed.
func checkCoords(name string, e entry, size int64) (entry, error) {
	if e.space > 1 {
		return entry{}, fmt.Errorf("chm: entry %q in unknown section: %w", name, book.ErrCorrupt)
	}
	if e.space == 0 && (e.start > uint64(size) || e.length > uint64(size)) {
		return entry{}, fmt.Errorf("chm: entry %q out of bounds: %w", name, book.ErrCorrupt)
	}
	return e, nil
}

// scanPMGL linearly scans one leaf block for name. See chm_lib.c:
// _chm_find_in_PMGL, _chm_parse_PMGL_entry.
func scanPMGL(block []byte, name string) (entry, error) {
	entries, err := parsePMGLAll(block)
	if err != nil {
		return entry{}, err
	}
	for _, e := range entries {
		if foldEqual(e.name, name) {
			return e.ent, nil
		}
	}
	return entry{}, fmt.Errorf("chm: missing entry %q: %w", name, book.ErrCorrupt)
}

// parsePMGLAll parses every entry of one leaf block.
func parsePMGLAll(block []byte) ([]namedEntry, error) {
	if len(block) < pmglLen || string(block[:4]) != "PMGL" {
		return nil, fmt.Errorf("chm: bad leaf header: %w", book.ErrCorrupt)
	}
	free := int64(binary.LittleEndian.Uint32(block[4:]))
	end := int64(len(block)) - free
	if end < pmglLen || end > int64(len(block)) {
		return nil, fmt.Errorf("chm: bad leaf free space: %w", book.ErrCorrupt)
	}
	var out []namedEntry
	pos := int64(pmglLen)
	for pos < end {
		keyLen, next, err := cword(block, pos, end)
		if err != nil {
			return nil, err
		}
		pos = next
		if keyLen > maxPathLen || pos+int64(keyLen) > end {
			return nil, fmt.Errorf("chm: bad leaf entry name: %w", book.ErrCorrupt)
		}
		key := string(block[pos : pos+int64(keyLen)])
		pos += int64(keyLen)
		var vals [3]uint64
		for i := range vals {
			v, next, err := cword(block, pos, end)
			if err != nil {
				return nil, err
			}
			vals[i] = v
			pos = next
		}
		out = append(out, namedEntry{name: key, ent: entry{space: vals[0], start: vals[1], length: vals[2]}})
	}
	return out, nil
}

// scanPMGI returns the child page whose key range holds name. See chm_lib.c:
// _chm_find_in_PMGI.
func scanPMGI(block []byte, name string) (int64, error) {
	if len(block) < pmgiLen || string(block[:4]) != "PMGI" {
		return -1, fmt.Errorf("chm: bad index header: %w", book.ErrCorrupt)
	}
	free := int64(binary.LittleEndian.Uint32(block[4:]))
	end := int64(len(block)) - free
	pos := int64(pmgiLen)
	page := int64(-1)
	for pos < end {
		keyLen, next, err := cword(block, pos, end)
		if err != nil {
			return -1, err
		}
		pos = next
		if keyLen > maxPathLen || pos+int64(keyLen) > end {
			return -1, fmt.Errorf("chm: bad index entry name: %w", book.ErrCorrupt)
		}
		key := string(block[pos : pos+int64(keyLen)])
		pos += int64(keyLen)
		if foldCompare(key, name) > 0 {
			return page, nil
		}
		p, next, err := cword(block, pos, end)
		if err != nil {
			return -1, err
		}
		pos = next
		page = int64(p)
	}
	return page, nil
}

// cword decodes a big-endian 7-bit group varint bounded by end.
func cword(buf []byte, pos, end int64) (uint64, int64, error) {
	var val uint64
	for i := 0; i < 10; i++ {
		if pos >= end || pos >= int64(len(buf)) {
			return 0, 0, fmt.Errorf("chm: truncated varint: %w", book.ErrCorrupt)
		}
		b := buf[pos]
		pos++
		val = (val << 7) | uint64(b&0x7F)
		if b&0x80 == 0 {
			return val, pos, nil
		}
	}
	return 0, 0, fmt.Errorf("chm: oversized varint: %w", book.ErrCorrupt)
}

// foldEqual reports ASCII case-insensitive equality.
func foldEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return foldCompare(a, b) == 0
}

// foldCompare compares ASCII case-insensitively.
func foldCompare(a, b string) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		ca, cb := lowerASCII(a[i]), lowerASCII(b[i])
		if ca != cb {
			if ca < cb {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	}
	return 0
}

func lowerASCII(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

// ensureComp loads the LZX control structures on first compressed read.
func (c *container) ensureComp() error {
	c.mu.Lock()
	if c.compDone {
		err := c.compErr
		c.mu.Unlock()
		return err
	}
	c.mu.Unlock()
	p, err := c.loadComp()
	c.mu.Lock()
	if !c.compDone {
		c.comp, c.compErr, c.compDone = p, err, true
	}
	err = c.compErr
	c.mu.Unlock()
	return err
}

func (c *container) loadComp() (*compParams, error) {
	rt, err := c.resolveRaw(resetTablePath)
	if err != nil {
		return nil, err
	}
	ctl, err := c.resolveRaw(controlPath)
	if err != nil {
		return nil, err
	}
	cn, err := c.resolveRaw(contentPath)
	if err != nil {
		return nil, err
	}
	if rt.space != 0 || ctl.space != 0 || cn.space != 0 {
		return nil, fmt.Errorf("chm: compressed control entries: %w", book.ErrCorrupt)
	}
	raw, err := c.readRawAt(resetTablePath, rt.start, lzxcResetTableLen)
	if err != nil {
		return nil, err
	}
	if v := binary.LittleEndian.Uint32(raw[0:]); v != 2 {
		return nil, fmt.Errorf("chm: unsupported reset table version %d: %w", v, book.ErrCorrupt)
	}
	p := &compParams{
		blockCount: uint64(binary.LittleEndian.Uint32(raw[4:])),
		tableOff:   uint64(binary.LittleEndian.Uint32(raw[12:])),
		rtStart:    rt.start,
		cnStart:    cn.start,
		uncompLen:  binary.LittleEndian.Uint64(raw[16:]),
		compLen:    binary.LittleEndian.Uint64(raw[24:]),
	}
	p.blockLen = binary.LittleEndian.Uint64(raw[32:])
	if p.blockCount == 0 || p.blockLen == 0 {
		return nil, fmt.Errorf("chm: bad reset table counts: %w", book.ErrCorrupt)
	}

	if ctl.length < lzxcControlMinLen || ctl.length > uint64(c.maxSize) {
		return nil, fmt.Errorf("chm: bad control length %d: %w", ctl.length, book.ErrCorrupt)
	}
	cb, err := c.readRawAt(controlPath, ctl.start, ctl.length)
	if err != nil {
		return nil, err
	}
	if string(cb[4:8]) != "LZXC" {
		return nil, fmt.Errorf("chm: bad control tag: %w", book.ErrCorrupt)
	}
	ver := binary.LittleEndian.Uint32(cb[8:])
	resetInterval := uint64(binary.LittleEndian.Uint32(cb[12:]))
	windowSize := uint64(binary.LittleEndian.Uint32(cb[16:]))
	windowsPerReset := uint64(binary.LittleEndian.Uint32(cb[20:]))
	if ver == 2 {
		resetInterval *= 0x8000
		windowSize *= 0x8000
	}
	if windowSize == 0 || windowSize == 1 || resetInterval == 0 ||
		resetInterval%(windowSize/2) != 0 {
		return nil, fmt.Errorf("chm: bad control window: %w", book.ErrCorrupt)
	}
	p.blocksPerTurn = resetInterval / (windowSize / 2) * windowsPerReset
	if p.blocksPerTurn == 0 {
		return nil, fmt.Errorf("chm: bad control window: %w", book.ErrCorrupt)
	}
	if bits.OnesCount64(windowSize) != 1 {
		return nil, fmt.Errorf("chm: bad control window: %w", book.ErrCorrupt)
	}
	p.windowBits = bits.Len64(windowSize) - 1

	return p, nil
}

// readRawAt reads length bytes at a data-area offset with explicit bounds,
// for control structures whose stored lengths are unreliable.
func (c *container) readRawAt(what string, start, length uint64) ([]byte, error) {
	if length == 0 || length > uint64(c.maxSize) || start > uint64(c.size) {
		return nil, fmt.Errorf("chm: %s out of bounds: %w", what, book.ErrCorrupt)
	}
	off := c.dataOff + int64(start)
	if off < 0 || off+int64(length) > c.size {
		return nil, fmt.Errorf("chm: %s out of bounds: %w", what, book.ErrCorrupt)
	}
	buf := make([]byte, length)
	if err := util.ReadAtFull(c.blob, buf, off); err != nil {
		return nil, fmt.Errorf("chm: failed to read %s: %w", what, book.ErrCorrupt)
	}
	return buf, nil
}

// readEntryHead returns at most limit bytes from the start of an entry,
// decompressing only the turns it needs. Used for charset sniffing so
// scan-time Read does not materialize whole entries.
func (c *container) readEntryHead(name string, e entry, limit uint64) ([]byte, error) {
	if e.length == 0 {
		return nil, nil
	}
	end := min(limit, e.length)
	switch e.space {
	case 0:
		off := c.dataOff + int64(e.start)
		if int64(e.start) < 0 || off+int64(end) > c.size {
			return nil, fmt.Errorf("chm: entry %q out of bounds: %w", name, book.ErrCorrupt)
		}
		return c.readBounded(off, int64(end), "entry "+name)
	case 1:
		return c.readEntryRange(name, e, e.start, e.start+end)
	default:
		return nil, fmt.Errorf("chm: entry %q in unknown section %d: %w", name, e.space, book.ErrCorrupt)
	}
}

// readCompressed decodes the reset intervals covering the entry slice,
// like chmlib retrieve_object looping over decompress_region calls.
func (c *container) readCompressed(name string, e entry) ([]byte, error) {
	return c.readEntryRange(name, e, e.start, e.start+e.length)
}

// readEntryRange decodes the entry's [start, end) slice of uncompressed
// content, decompressing only the turns covering that slice.
func (c *container) readEntryRange(name string, e entry, start, end uint64) ([]byte, error) {
	if err := c.ensureComp(); err != nil {
		return nil, err
	}
	p := c.comp
	if start < e.start || end < start || end > e.start+e.length {
		return nil, fmt.Errorf("chm: entry %q range out of bounds: %w", name, book.ErrCorrupt)
	}
	if e.start > p.uncompLen || end > p.uncompLen {
		return nil, fmt.Errorf("chm: entry %q out of content bounds: %w", name, book.ErrCorrupt)
	}
	turnLen := p.blocksPerTurn * p.blockLen
	out := make([]byte, 0, end-start)
	for off := start; off < end; {
		turnStart := off / turnLen * turnLen
		needEnd := min(turnStart+turnLen, end)
		plain, err := c.decodeTurn(turnStart, needEnd)
		if err != nil {
			return nil, err
		}
		from := off - turnStart
		if from >= uint64(len(plain)) {
			return nil, fmt.Errorf("chm: entry %q outside interval: %w", name, book.ErrCorrupt)
		}
		n := min(uint64(len(plain))-from, end-off)
		if n == 0 {
			return nil, fmt.Errorf("chm: entry %q made no progress: %w", name, book.ErrCorrupt)
		}
		out = append(out, plain[from:from+n]...)
		off += n
	}
	return out, nil
}

// decodeTurn decompresses the turn prefix the entry needs: from turnStart
// up to needEnd. Bounding the range by the last touched block (instead of
// the whole turn) tolerates overstated table tails, like chmlib block reads.
// Decoded prefixes are memoized under mu (bounded, longest-kept) so repeated
// and concurrent reads of one interval share the work.
func (c *container) decodeTurn(turnStart, needEnd uint64) ([]byte, error) {
	p := c.comp
	turnLen := p.blocksPerTurn * p.blockLen
	turnEnd := min(turnStart+turnLen, p.uncompLen)
	if needEnd <= turnStart || needEnd > turnEnd {
		return nil, fmt.Errorf("chm: bad turn range: %w", book.ErrCorrupt)
	}

	c.mu.Lock()
	if prefix, ok := c.turns[turnStart]; ok && turnStart+uint64(len(prefix)) >= needEnd {
		out := prefix[:needEnd-turnStart]
		c.mu.Unlock()
		return out, nil
	}
	c.mu.Unlock()

	b0 := turnStart / p.blockLen
	b1 := min((needEnd-1)/p.blockLen+1, p.blockCount)
	compStart, err := c.resetEntry(b0)
	if err != nil {
		return nil, err
	}
	var compEnd uint64
	if b1 < p.blockCount {
		compEnd, err = c.resetEntry(b1)
		if err != nil {
			return nil, err
		}
	} else {
		compEnd = p.compLen
	}
	if compEnd < compStart {
		return nil, fmt.Errorf("chm: bad reset table range: %w", book.ErrCorrupt)
	}
	off := c.dataOff + int64(p.cnStart) + int64(compStart)
	clen := int64(compEnd - compStart)
	if clen > c.maxSize {
		return nil, book.LimitError("compressed interval", clen, c.maxSize)
	}
	if off < 0 || off+clen > c.size {
		return nil, fmt.Errorf("chm: compressed data out of bounds: %w", book.ErrCorrupt)
	}
	comp := make([]byte, clen)
	if err := util.ReadAtFull(c.blob, comp, off); err != nil {
		return nil, fmt.Errorf("chm: short read: %w", book.ErrCorrupt)
	}
	rc, err := compress.NewLZXReader(bytes.NewReader(comp), p.windowBits, int64(needEnd-turnStart), c.maxSize)
	if err != nil {
		return nil, err
	}
	plain, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		return nil, err
	}
	if uint64(len(plain)) != needEnd-turnStart {
		return nil, fmt.Errorf("chm: LZX interval gave %d want %d: %w", len(plain), needEnd-turnStart, book.ErrCorrupt)
	}
	c.mu.Lock()
	if cur, ok := c.turns[turnStart]; ok && uint64(len(cur)) >= needEnd-turnStart {
		plain = cur[:needEnd-turnStart]
	} else {
		c.cacheTurn(turnStart, plain)
	}
	c.mu.Unlock()
	return plain, nil
}

// cacheTurn retains a decoded turn prefix under mu (bounded FIFO,
// longest-kept). Oversized turns are skipped entirely so turnOrder never
// holds phantom keys missing from the map.
func (c *container) cacheTurn(turnStart uint64, plain []byte) {
	if uint64(len(plain)) > maxCachedTurn {
		return
	}
	if c.turns == nil {
		c.turns = map[uint64][]byte{}
	}
	if prev, ok := c.turns[turnStart]; ok {
		if len(plain) > len(prev) {
			c.turns[turnStart] = plain
		}
		return
	}
	if len(c.turnOrder) >= turnCacheMax {
		delete(c.turns, c.turnOrder[0])
		c.turnOrder = c.turnOrder[1:]
	}
	c.turnOrder = append(c.turnOrder, turnStart)
	c.turns[turnStart] = plain
}

// resetEntry reads one compressed offset from the reset table.
func (c *container) resetEntry(block uint64) (uint64, error) {
	p := c.comp
	off := c.dataOff + int64(p.rtStart) + int64(p.tableOff) + int64(block*8)
	var buf [8]byte
	if off < 0 || off+8 > c.size {
		return 0, fmt.Errorf("chm: reset table out of bounds: %w", book.ErrCorrupt)
	}
	if err := util.ReadAtFull(c.blob, buf[:], off); err != nil {
		return 0, fmt.Errorf("chm: failed to read reset table: %w", book.ErrCorrupt)
	}
	return binary.LittleEndian.Uint64(buf[:]), nil
}

// namedEntry pairs an internal path with its payload coordinates.
type namedEntry struct {
	name string
	ent  entry
}

// enumerate lists every directory entry following the PMGL leaf chain from
// the index head, like chm_enumerate. Orphaned pages outside the chain are
// ignored so the manifest agrees with PMGI resolution.
func (c *container) enumerate() ([]namedEntry, error) {
	var out []namedEntry
	pageBuf := make([]byte, c.blkLen)
	for page, i := c.indexHd, 0; page != -1; i++ {
		if i > maxDirWalk {
			return nil, fmt.Errorf("chm: directory walk did not terminate: %w", book.ErrCorrupt)
		}
		off := c.dirOff + page*c.blkLen
		if page < 0 || off+c.blkLen > c.dirOff+c.dirLen {
			return nil, fmt.Errorf("chm: directory page out of bounds: %w", book.ErrCorrupt)
		}
		if err := util.ReadAtFull(c.blob, pageBuf, off); err != nil {
			return nil, fmt.Errorf("chm: failed to read directory page: %w", book.ErrCorrupt)
		}
		if string(pageBuf[:4]) != "PMGL" {
			return nil, fmt.Errorf("chm: bad leaf header: %w", book.ErrCorrupt)
		}
		entries, err := parsePMGLAll(pageBuf)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			out = append(out, namedEntry{name: e.name, ent: e.ent})
		}
		page = int64(int32LE(pageBuf[16:]))
	}
	return out, nil
}

// int32LE decodes a signed little-endian 32-bit integer.
func int32LE(raw []byte) int32 {
	return int32(binary.LittleEndian.Uint32(raw))
}
