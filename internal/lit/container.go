package lit

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/f0d0r/margaret-ebook-library/book"
	compress "github.com/f0d0r/margaret-ebook-library/internal/compress"
)

// Container layout constants. Offsets are into the primary header;
// multi-byte integers are little-endian (in contrast to MOBI).
const (
	litVersion = 1

	priVersion   = 8
	priHdrLen    = 12
	priNumPieces = 16
	priSecHdrLen = 20
	priSize      = 40

	pieceSize = 16

	// maxHeaderBytes bounds header metadata reads against corrupt
	// length claims.
	maxHeaderBytes = 64 << 20
)

// Transform GUIDs (little-endian encoding, uppercase text form).
const (
	desEncryptGUID  = "{67F6E4A2-60BF-11D3-8540-00C04F58C3CF}"
	lzxCompressGUID = "{0A9007C6-4076-11D3-8789-0000F8105754}"
)

// metaEntry is the internal entry holding the binary OPF document.
const metaEntry = "/meta"

// nameListEntry lists the storage section names.
const nameListEntry = "::DataSpace/NameList"

// storagePrefix scopes per-section transform, content and control entries.
const storagePrefix = "::DataSpace/Storage/"

// directoryEntry maps an internal path to a section slice.
type directoryEntry struct {
	name    string
	section int64
	offset  int64
	size    int64
}

// container is a parsed LIT header/directory over a Blob. Section payloads
// assemble lazily through getFile; only metadata-sized pieces materialize
// during Read.
type container struct {
	blob          book.Blob
	fileSize      int64
	contentOffset int64
	maxSize       int64
	entries       map[string]directoryEntry
	sectionNames  []string
	sectionCache  [][][]byte // per-section assembled payload, nil until loaded
	manifest      map[string]manifestItem

	entryChunkLen uint32
	countChunkLen uint32
	entryUnknown  uint32
	countUnknown  uint32
}

// openContainer parses the primary/secondary headers, the piece table, the
// directory and the section name list. DRM-protected books fail with
// book.ErrDRM since no keys are available.
func openContainer(b book.Blob, maxSize int64) (*container, error) {
	size, err := b.Size()
	if err != nil {
		return nil, fmt.Errorf("lit: failed to stat: %w", err)
	}
	if size < minLitSize {
		return nil, fmt.Errorf("lit: file too small: %w", book.ErrCorrupt)
	}
	c := &container{blob: b, fileSize: size, maxSize: maxSize, entries: map[string]directoryEntry{}}

	head := make([]byte, priSize)
	if err := readAtFull(b, head, 0); err != nil {
		return nil, fmt.Errorf("lit: failed to read header: %w", book.ErrCorrupt)
	}
	if string(head[:8]) != litMagic {
		return nil, fmt.Errorf("lit: bad magic: %w", book.ErrCorrupt)
	}
	if v := binary.LittleEndian.Uint32(head[priVersion:]); v != litVersion {
		return nil, fmt.Errorf("lit: unsupported version %d: %w", v, book.ErrCorrupt)
	}
	hdrLen := int64(int32LE(head[priHdrLen:]))
	numPieces := int64(int32LE(head[priNumPieces:]))
	secHdrLen := int64(int32LE(head[priSecHdrLen:]))
	if hdrLen < 0 || numPieces < 0 || secHdrLen < 0 {
		return nil, fmt.Errorf("lit: negative header length: %w", book.ErrCorrupt)
	}
	if numPieces > 1<<20 || secHdrLen > maxHeaderBytes {
		return nil, fmt.Errorf("lit: absurd header length: %w", book.ErrCorrupt)
	}
	totalHdr := hdrLen + numPieces*pieceSize + secHdrLen
	if totalHdr > size {
		return nil, fmt.Errorf("lit: header exceeds file: %w", book.ErrCorrupt)
	}
	if numPieces < 2 {
		return nil, fmt.Errorf("lit: no directory piece: %w", book.ErrCorrupt)
	}

	if err := c.readSecondaryHeader(hdrLen+numPieces*pieceSize, secHdrLen); err != nil {
		return nil, err
	}
	if err := c.readPieces(hdrLen, numPieces); err != nil {
		return nil, err
	}
	if isDRMProtected(c.entries) {
		return nil, fmt.Errorf("lit: DRM protected: %w", book.ErrDRM)
	}
	if err := c.readSectionNames(); err != nil {
		return nil, err
	}
	raw, err := c.getFile("/manifest")
	if err != nil {
		return nil, fmt.Errorf("lit: no manifest: %w", book.ErrCorrupt)
	}
	if c.manifest, err = readManifest(raw); err != nil {
		return nil, err
	}
	return c, nil
}

// isDRMProtected reports DRM when a licenses entry exists. Only level 5
// ("owner exclusive", passport-locked) is unreadable: sealed/inscribed
// books decrypt transparently from content-derived keys, and neither the
// metadata nor the LZX-only content sections need keys at all.
func isDRMProtected(entries map[string]directoryEntry) bool {
	_, ok := entries["/DRMStorage/Licenses/EUL"]
	return ok
}

// readSecondaryHeader parses CAOL/ITSF blocks; the ITSF block yields the
// content offset. Unknown blocks are skipped via their declared length.
func (c *container) readSecondaryHeader(offset, length int64) error {
	if length == 0 {
		return fmt.Errorf("lit: empty secondary header: %w", book.ErrCorrupt)
	}
	raw := make([]byte, length)
	if err := readAtFull(c.blob, raw, offset); err != nil {
		return fmt.Errorf("lit: failed to read secondary header: %w", book.ErrCorrupt)
	}
	at := int64(int32LE(raw[4:]))
	found := false
	for at < length {
		if at+12 > length {
			return fmt.Errorf("lit: truncated secondary block: %w", book.ErrCorrupt)
		}
		tag := string(raw[at : at+4])
		ver := binary.LittleEndian.Uint32(raw[at+4:])
		blockLen := int64(int32LE(raw[at+8:]))
		switch tag {
		case "CAOL":
			if ver != 2 {
				return fmt.Errorf("lit: unknown CAOL version %d: %w", ver, book.ErrCorrupt)
			}
			if at+48 > length {
				return fmt.Errorf("lit: truncated CAOL block: %w", book.ErrCorrupt)
			}
			c.entryChunkLen = binary.LittleEndian.Uint32(raw[at+20:])
			c.countChunkLen = binary.LittleEndian.Uint32(raw[at+24:])
			c.entryUnknown = binary.LittleEndian.Uint32(raw[at+28:])
			c.countUnknown = binary.LittleEndian.Uint32(raw[at+32:])
			at += 48
		case "ITSF":
			if ver != 4 {
				return fmt.Errorf("lit: unknown ITSF version %d: %w", ver, book.ErrCorrupt)
			}
			if at+32 > length {
				return fmt.Errorf("lit: truncated ITSF block: %w", book.ErrCorrupt)
			}
			if binary.LittleEndian.Uint32(raw[at+20:]) != 0 {
				return fmt.Errorf("lit: 64-bit content offset: %w", book.ErrCorrupt)
			}
			c.contentOffset = int64(binary.LittleEndian.Uint32(raw[at+16:]))
			found = true
			if blockLen <= 0 {
				return fmt.Errorf("lit: bad ITSF length: %w", book.ErrCorrupt)
			}
			at += blockLen
		default:
			if blockLen <= 0 || at+blockLen > length {
				return fmt.Errorf("lit: bad secondary block length: %w", book.ErrCorrupt)
			}
			at += blockLen
		}
	}
	if at != length {
		return fmt.Errorf("lit: secondary header length mismatch: %w", book.ErrCorrupt)
	}
	if !found {
		return fmt.Errorf("lit: no content offset: %w", book.ErrCorrupt)
	}
	return nil
}

// readPieces walks the piece table; piece 1 holds the directory.
func (c *container) readPieces(hdrLen, numPieces int64) error {
	table := make([]byte, numPieces*pieceSize)
	if err := readAtFull(c.blob, table, hdrLen); err != nil {
		return fmt.Errorf("lit: failed to read piece table: %w", book.ErrCorrupt)
	}
	for i := int64(0); i < numPieces; i++ {
		piece := table[i*pieceSize : (i+1)*pieceSize]
		if binary.LittleEndian.Uint32(piece[4:]) != 0 || binary.LittleEndian.Uint32(piece[12:]) != 0 {
			return fmt.Errorf("lit: piece %d has 64-bit value: %w", i, book.ErrCorrupt)
		}
		offset := int64(binary.LittleEndian.Uint32(piece[0:]))
		size := int64(int32LE(piece[8:]))
		if offset < 0 || size < 0 || size > c.fileSize-offset {
			return fmt.Errorf("lit: piece %d out of bounds: %w", i, book.ErrCorrupt)
		}
		switch i {
		case 0:
			// File size piece; the Blob size is authoritative.
		case 1:
			data, err := c.readBounded(offset, size)
			if err != nil {
				return err
			}
			if len(data) < 16 {
				return fmt.Errorf("lit: directory piece too short: %w", book.ErrCorrupt)
			}
			if binary.LittleEndian.Uint32(data[8:]) != c.entryChunkLen ||
				binary.LittleEndian.Uint32(data[12:]) != c.entryUnknown {
				return fmt.Errorf("lit: directory piece mismatches secondary header: %w", book.ErrCorrupt)
			}
			if err := readDirectory(data, c.entries); err != nil {
				return err
			}
		case 2:
			data, err := c.readBounded(offset, size)
			if err != nil {
				return err
			}
			if len(data) < 16 {
				return fmt.Errorf("lit: count piece too short: %w", book.ErrCorrupt)
			}
			if binary.LittleEndian.Uint32(data[8:]) != c.countChunkLen ||
				binary.LittleEndian.Uint32(data[12:]) != c.countUnknown {
				return fmt.Errorf("lit: count piece mismatches secondary header: %w", book.ErrCorrupt)
			}
		}
	}
	return nil
}

// readBounded reads size bytes at offset, enforcing maxSize.
func (c *container) readBounded(offset, size int64) ([]byte, error) {
	if size > c.maxSize {
		return nil, fmt.Errorf("lit: %d bytes exceeds limit %d: %w", size, c.maxSize, book.ErrLimitExceeded)
	}
	data := make([]byte, size)
	if err := readAtFull(c.blob, data, offset); err != nil {
		return nil, fmt.Errorf("lit: short read: %w", book.ErrCorrupt)
	}
	return data, nil
}

// readDirectory parses the IFCM/AOLL directory into entries.
func readDirectory(piece []byte, entries map[string]directoryEntry) error {
	if len(piece) < 32 || string(piece[:4]) != "IFCM" {
		return fmt.Errorf("lit: directory is not IFCM: %w", book.ErrCorrupt)
	}
	chunkSize := int64(int32LE(piece[8:12]))
	numChunks := int64(int32LE(piece[24:28]))
	if chunkSize <= 0 || numChunks < 0 || 32+numChunks*chunkSize != int64(len(piece)) {
		return fmt.Errorf("lit: bad IFCM length: %w", book.ErrCorrupt)
	}
	for i := int64(0); i < numChunks; i++ {
		chunk := piece[32+i*chunkSize : 32+(i+1)*chunkSize]
		if string(chunk[:4]) != "AOLL" {
			continue
		}
		rest := chunk[4:]
		if len(rest) < 46 {
			return fmt.Errorf("lit: truncated AOLL chunk: %w", book.ErrCorrupt)
		}
		remaining := int(int32LE(rest[:4]))
		if remaining < 0 || int64(remaining) >= chunkSize {
			return fmt.Errorf("lit: bad AOLL length: %w", book.ErrCorrupt)
		}
		remaining = int(chunkSize) - (remaining + 48)
		rest = rest[4:]
		count := int(binary.LittleEndian.Uint16(rest[len(rest)-2:]))
		if count == 0 {
			count = (1 << 16) - 1
		}
		rest = rest[40:]
		for j := 0; j < count && remaining > 0; j++ {
			var nameLen, section, offset, size int64
			var err error
			if nameLen, rest, remaining, err = encInt(rest, remaining); err != nil {
				return err
			}
			if nameLen < 0 || nameLen > int64(remaining-3) {
				return fmt.Errorf("lit: directory entry overruns chunk: %w", book.ErrCorrupt)
			}
			name := string(rest[:nameLen])
			if !utf8.ValidString(name) {
				break
			}
			rest = rest[nameLen:]
			remaining -= int(nameLen)
			if section, rest, remaining, err = encInt(rest, remaining); err != nil {
				return err
			}
			if offset, rest, remaining, err = encInt(rest, remaining); err != nil {
				return err
			}
			if size, rest, remaining, err = encInt(rest, remaining); err != nil {
				return err
			}
			if section < 0 || offset < 0 || size < 0 {
				return fmt.Errorf("lit: negative directory entry: %w", book.ErrCorrupt)
			}
			entries[name] = directoryEntry{name: name, section: section, offset: offset, size: size}
		}
	}
	return nil
}

// encInt decodes a big-endian 7-bit group varint bounded by budget.
func encInt(data []byte, budget int) (int64, []byte, int, error) {
	var val int64
	pos := 0
	for budget > 0 {
		if pos >= len(data) {
			return 0, nil, 0, fmt.Errorf("lit: truncated varint: %w", book.ErrCorrupt)
		}
		if pos >= 10 {
			return 0, nil, 0, fmt.Errorf("lit: oversized varint: %w", book.ErrCorrupt)
		}
		b := data[pos]
		pos++
		budget--
		val = (val << 7) | int64(b&0x7F)
		if b&0x80 == 0 {
			return val, data[pos:], budget, nil
		}
	}
	return 0, nil, 0, fmt.Errorf("lit: unterminated varint: %w", book.ErrCorrupt)
}

// readSectionNames parses the ::DataSpace/NameList section.
func (c *container) readSectionNames() error {
	raw, err := c.getFile(nameListEntry)
	if err != nil {
		return err
	}
	if len(raw) < 4 {
		return fmt.Errorf("lit: invalid namelist: %w", book.ErrCorrupt)
	}
	count := int(binary.LittleEndian.Uint16(raw[2:4]))
	c.sectionNames = make([]string, count)
	c.sectionCache = make([][][]byte, count)
	pos := 4
	for i := range c.sectionNames {
		if pos+2 > len(raw) {
			return fmt.Errorf("lit: truncated namelist: %w", book.ErrCorrupt)
		}
		n := int(binary.LittleEndian.Uint16(raw[pos:]))
		pos += 2
		size := n*2 + 2
		if pos+size > len(raw) {
			return fmt.Errorf("lit: truncated section name: %w", book.ErrCorrupt)
		}
		name := decodeUTF16LE(raw[pos : pos+size])
		pos += size
		c.sectionNames[i] = name
	}
	return nil
}

// getFile returns the bytes of an internal entry, assembling transformed
// sections on demand.
func (c *container) getFile(name string) ([]byte, error) {
	entry, ok := c.entries[name]
	if !ok {
		return nil, fmt.Errorf("lit: missing entry %q: %w", name, book.ErrCorrupt)
	}
	if entry.section == 0 {
		if entry.size > c.maxSize {
			return nil, fmt.Errorf("lit: entry %q %d bytes exceeds limit %d: %w", name, entry.size, c.maxSize, book.ErrLimitExceeded)
		}
		off := c.contentOffset + entry.offset
		if entry.offset < 0 || entry.size > c.fileSize-off {
			return nil, fmt.Errorf("lit: entry %q out of bounds: %w", name, book.ErrCorrupt)
		}
		return c.readBounded(off, entry.size)
	}
	section, err := c.sectionData(entry.section)
	if err != nil {
		return nil, err
	}
	if entry.offset < 0 || entry.size < 0 || entry.offset+entry.size > int64(len(section)) {
		return nil, fmt.Errorf("lit: entry %q out of section bounds: %w", name, book.ErrCorrupt)
	}
	return section[entry.offset : entry.offset+entry.size], nil
}

// sectionData assembles a storage section through its transform list.
func (c *container) sectionData(section int64) ([]byte, error) {
	if section < 0 || section >= int64(len(c.sectionNames)) {
		return nil, fmt.Errorf("lit: bad section %d: %w", section, book.ErrCorrupt)
	}
	if cached := c.sectionCache[section]; cached != nil {
		return cached[0], nil
	}
	base := storagePrefix + c.sectionNames[section]
	transform, err := c.getFile(base + "/Transform/List")
	if err != nil {
		return nil, err
	}
	content, err := c.getFile(base + "/Content")
	if err != nil {
		return nil, err
	}
	control, err := c.getFile(base + "/ControlData")
	if err != nil {
		return nil, err
	}
	for len(transform) >= 16 {
		if len(control) < 4 {
			return nil, fmt.Errorf("lit: truncated control data: %w", book.ErrCorrupt)
		}
		csize := int64(int32LE(control)) + 1
		csize *= 4
		if csize <= 0 || csize > int64(len(control)) {
			return nil, fmt.Errorf("lit: control data too short: %w", book.ErrCorrupt)
		}
		switch guid := formatGUID(transform[:16]); guid {
		case desEncryptGUID:
			return nil, fmt.Errorf("LIT content is sealed; decryption is out of scope: %w", book.ErrDRM)
		case lzxCompressGUID:
			reset, err := c.getFile(base + "/Transform/" + lzxCompressGUID + "/InstanceData/ResetTable")
			if err != nil {
				return nil, err
			}
			content, err = decompressSection(content, control[:csize], reset, c.maxSize)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("lit: unrecognized transform %s: %w", guid, book.ErrUnsupportedFormat)
		}
		control = control[csize:]
		transform = transform[16:]
	}
	if int64(len(content)) > c.maxSize {
		return nil, fmt.Errorf("lit: section %q %d bytes exceeds limit %d: %w", c.sectionNames[int(section)], len(content), c.maxSize, book.ErrLimitExceeded)
	}
	c.sectionCache[section] = [][]byte{content}
	return content, nil
}

// decompressSection runs one LZX reset-table pass over control-sized chunks.
func decompressSection(content, control, reset []byte, maxSize int64) ([]byte, error) {
	if len(control) < 32 || string(control[4:8]) != "LZXC" {
		return nil, fmt.Errorf("lit: invalid LZXC control tag: %w", book.ErrCorrupt)
	}
	windowBits := 14
	for u := binary.LittleEndian.Uint32(control[12:]); u > 0; u >>= 1 {
		windowBits++
	}
	if windowBits < 15 || windowBits > 21 {
		return nil, fmt.Errorf("lit: invalid LZX window %d: %w", windowBits, book.ErrCorrupt)
	}
	if len(reset) < 40 {
		return nil, fmt.Errorf("lit: reset table too short: %w", book.ErrCorrupt)
	}
	if binary.LittleEndian.Uint32(reset[20:]) != 0 {
		return nil, fmt.Errorf("lit: 64-bit reset length: %w", book.ErrCorrupt)
	}
	outLen := int64(int32LE(reset[16:]))
	if outLen < 0 {
		return nil, fmt.Errorf("lit: negative reset length: %w", book.ErrCorrupt)
	}
	windowBytes := int64(1) << uint(windowBits)
	ofsEntry := int64(int32LE(reset[12:])) + 8
	if ofsEntry < 0 {
		return nil, fmt.Errorf("lit: bad reset entry offset: %w", book.ErrCorrupt)
	}
	accum := int64(int32LE(reset[32:]))
	var out [][]byte
	remaining := outLen
	base := int64(0)
	for ofsEntry < int64(len(reset)) {
		if accum >= windowBytes {
			accum = 0
			if ofsEntry+8 > int64(len(reset)) {
				return nil, fmt.Errorf("lit: truncated reset entry: %w", book.ErrCorrupt)
			}
			size := int64(int32LE(reset[ofsEntry:]))
			if binary.LittleEndian.Uint32(reset[ofsEntry+4:]) != 0 {
				return nil, fmt.Errorf("lit: 64-bit reset entry: %w", book.ErrCorrupt)
			}
			if size < 0 || size > int64(len(content)) {
				return nil, fmt.Errorf("lit: reset entry out of bounds: %w", book.ErrCorrupt)
			}
			if size < base {
				return nil, fmt.Errorf("lit: reset entry goes backwards: %w", book.ErrCorrupt)
			}
			if remaining >= windowBytes {
				chunk, err := decompressChunk(content[base:size], windowBits, windowBytes, maxSize)
				if err != nil {
					return nil, fmt.Errorf("lit: LZX chunk [%d:%d]: %w", base, size, err)
				}
				out = append(out, chunk)
				remaining -= windowBytes
				base = size
			}
		}
		accum += int64(int32LE(reset[32:]))
		ofsEntry += 8
	}
	if remaining > 0 {
		if base > int64(len(content)) {
			return nil, fmt.Errorf("lit: reset base out of bounds: %w", book.ErrCorrupt)
		}
		chunk, err := decompressChunk(content[base:], windowBits, remaining, maxSize)
		if err != nil {
			return nil, err
		}
		out = append(out, chunk)
	}
	return bytes.Join(out, nil), nil
}

// decompressChunk decodes one reset-window chunk streaming into memory.
func decompressChunk(data []byte, windowBits int, outLen, maxSize int64) ([]byte, error) {
	rc, err := compress.NewLZXReader(bytes.NewReader(data), windowBits, outLen, maxSize)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	decoded, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	if int64(len(decoded)) != outLen {
		return nil, fmt.Errorf("lit: LZX chunk gave %d want %d: %w", len(decoded), outLen, book.ErrCorrupt)
	}
	return decoded, nil
}

// getAtoms reads custom tag/attribute names for a content entry. Missing
// atom entries mean no custom tags; damaged tables degrade to the partial
// prefix, mirroring reference tolerance.
func (c *container) getAtoms(internal string) *unbinAtoms {
	atoms := &unbinAtoms{}
	data, err := c.getFile("/data/" + internal + "/atom")
	if err != nil {
		return atoms
	}
	pos := 0
	u32 := func() (uint32, bool) {
		if pos+4 > len(data) {
			return 0, false
		}
		v := binary.LittleEndian.Uint32(data[pos:])
		pos += 4
		return v, true
	}
	n, ok := u32()
	if !ok {
		return atoms
	}
	tags := map[int]string{}
	for i := 1; i <= int(n); i++ {
		if pos >= len(data) {
			break
		}
		size := int(data[pos])
		pos++
		if size == 0 || pos+size > len(data) {
			break
		}
		tags[i] = string(data[pos : pos+size])
		pos += size
	}
	atoms.tags = tags
	n, ok = u32()
	if !ok {
		return atoms
	}
	attrs := map[int]string{}
	for i := 1; i <= int(n); i++ {
		size, ok := u32()
		if !ok || size == 0 {
			break
		}
		if uint64(pos)+uint64(size) > uint64(len(data)) {
			break
		}
		attrs[i] = string(data[pos : pos+int(size)])
		pos += int(size)
	}
	atoms.attrs = attrs
	return atoms
}

// formatGUID renders a little-endian GUID in registry form.
func formatGUID(raw []byte) string {
	d1 := binary.LittleEndian.Uint32(raw[0:])
	w1 := binary.LittleEndian.Uint16(raw[4:])
	w2 := binary.LittleEndian.Uint16(raw[6:])
	return fmt.Sprintf("{%08X-%04X-%04X-%02X%02X-%02X%02X%02X%02X%02X%02X}",
		d1, w1, w2, raw[8], raw[9], raw[10], raw[11], raw[12], raw[13], raw[14], raw[15])
}

// decodeUTF16LE decodes NUL-trimmed UTF-16LE text.
func decodeUTF16LE(raw []byte) string {
	u16 := make([]uint16, 0, len(raw)/2)
	for i := 0; i+1 < len(raw); i += 2 {
		u16 = append(u16, binary.LittleEndian.Uint16(raw[i:]))
	}
	for len(u16) > 0 && u16[len(u16)-1] == 0 {
		u16 = u16[:len(u16)-1]
	}
	runes := make([]rune, 0, len(u16))
	for i := 0; i < len(u16); {
		c := u16[i]
		if c >= 0xD800 && c <= 0xDBFF && i+1 < len(u16) {
			lo := u16[i+1]
			if lo >= 0xDC00 && lo <= 0xDFFF {
				runes = append(runes, rune(0x10000+uint32(c-0xD800)<<10+uint32(lo-0xDC00)))
				i += 2
				continue
			}
		}
		runes = append(runes, rune(c))
		i++
	}
	return string(runes)
}

// int32LE decodes a signed little-endian 32-bit integer.
func int32LE(raw []byte) int32 {
	return int32(binary.LittleEndian.Uint32(raw))
}

// readAtFull reads exactly len(buf) bytes at offset or fails.
func readAtFull(r io.ReaderAt, buf []byte, offset int64) error {
	n, err := r.ReadAt(buf, offset)
	if err != nil && err != io.EOF {
		return err
	}
	if n != len(buf) {
		return io.ErrUnexpectedEOF
	}
	return nil
}
