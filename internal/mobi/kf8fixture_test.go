package mobi

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// This file builds KF8/MOBI8 test files programmatically, mirroring the real
// layout closely enough for the parser to walk it exactly like a real file: a
// PDB record table, a PalmDOC+MOBI header (with EXTH), FDST flow tables and
// multi-record INDX indices (leading header record + TAGX section + one entry
// record per entry + IDXT table).

// recordSpec describes one PDB record. A non-negative size overrides the
// declared record length (used to build records whose header claims more data
// than the record carries, i.e. truncated files); data holds the bytes.
type recordSpec struct {
	data []byte
	size int
}

func (r recordSpec) declaredSize() int {
	if r.size > 0 {
		return r.size
	}
	return len(r.data)
}

func (r recordSpec) emitted() []byte {
	out := make([]byte, r.declaredSize())
	copy(out, r.data)
	return out
}

// assemblePdb writes a PDB container header (name/type/creator/record
// count) plus the record offset table, then the record payloads. Test
// builders share it instead of duplicating the layout.
func assemblePdb(name, dbType, creator string, records [][]byte) []byte {
	header := make([]byte, PDB_HEADER_SIZE)
	copy(header[0:], name)
	copy(header[60:64], dbType)
	copy(header[64:68], creator)
	binary.BigEndian.PutUint16(header[76:78], uint16(len(records)))

	var out bytes.Buffer
	out.Write(header)
	offset := uint32(PDB_HEADER_SIZE + len(records)*8)
	for _, rec := range records {
		entry := make([]byte, 8)
		binary.BigEndian.PutUint32(entry[0:4], offset)
		out.Write(entry)
		offset += uint32(len(rec))
	}
	for _, rec := range records {
		out.Write(rec)
	}
	return out.Bytes()
}

// makePdbBlob assembles a PDB container from the given records. Record 0 is
// expected to be the MOBI header record.
func makePdbBlob(t *testing.T, name string, records []recordSpec) []byte {
	t.Helper()
	if len(records) == 0 {
		t.Fatal("makePdbBlob: no records")
	}

	raw := make([][]byte, 0, len(records))
	for _, rec := range records {
		raw = append(raw, rec.emitted())
	}
	out := assemblePdb(name, "BOOK", "MOBI", raw)
	binary.BigEndian.PutUint32(out[36:40], 2082844800+1000)
	binary.BigEndian.PutUint32(out[40:44], 2082844800+1000+86400)
	binary.BigEndian.PutUint32(out[48:52], 1)
	binary.BigEndian.PutUint32(out[68:72], 1)
	for i := range records {
		copy(out[PDB_HEADER_SIZE+i*8+5:PDB_HEADER_SIZE+i*8+8], []byte{0, 0, byte(i + 1)})
	}
	return out
}

// mobiHeaderOpts describes the PalmDOC+MOBI header record of a book.
type mobiHeaderOpts struct {
	compression      CompressionType
	textLength       uint32
	textRecordCount  uint16
	encryption       EncryptionType
	headerLength     uint32
	mobiType         MobiType
	textEncoding     TextEncodingType
	mobiVersion      uint32
	extraRecordFlags uint32
	exthRecords      map[uint32][]byte
	drmOffset        uint32
	fdstIdx          uint32
	fdstCount        uint32
	divIdx           uint32
	skelIdx          uint32
}

func defaultMobiHeaderOpts() mobiHeaderOpts {
	return mobiHeaderOpts{
		compression:  CompressionNone,
		headerLength: 232,
		mobiType:     MobiTypeBook,
		textEncoding: UTF8,
		mobiVersion:  6,
		drmOffset:    0xFFFFFFFF,
		fdstIdx:      NullIndex,
		divIdx:       NullIndex,
		skelIdx:      NullIndex,
	}
}

// makeMobiHeaderRecord builds the record 0 payload: PalmDOC header, MOBI header
// of headerLength bytes and, when exthRecords is set, an EXTH block directly
// behind it (calibre reads it at headerLength+16).
func makeMobiHeaderRecord(opts mobiHeaderOpts) recordSpec {
	exth := makeExthRecord(opts.exthRecords)
	size := int(opts.headerLength) + PALM_DOC_HEADER_SIZE + len(exth)
	if size < 300 {
		size = 300 // keep room for the KF8 index fields at 0xC0..0x108
	}
	data := make([]byte, size)

	binary.BigEndian.PutUint16(data[0:2], uint16(opts.compression))
	binary.BigEndian.PutUint32(data[4:8], opts.textLength)
	binary.BigEndian.PutUint16(data[8:10], opts.textRecordCount)
	binary.BigEndian.PutUint16(data[10:12], 4096)
	binary.BigEndian.PutUint16(data[12:14], uint16(opts.encryption))
	copy(data[16:20], "MOBI")
	binary.BigEndian.PutUint32(data[20:24], opts.headerLength)
	binary.BigEndian.PutUint32(data[24:28], uint32(opts.mobiType))
	binary.BigEndian.PutUint32(data[28:32], uint32(opts.textEncoding))
	binary.BigEndian.PutUint32(data[36:40], opts.mobiVersion)
	binary.BigEndian.PutUint32(data[80:84], 1) // first non-text record
	if opts.exthRecords != nil {
		binary.BigEndian.PutUint32(data[128:132], 0x40) // EXTH present
	}
	binary.BigEndian.PutUint32(data[168:172], opts.drmOffset)
	binary.BigEndian.PutUint32(data[172:176], 0xFFFFFFFF)
	binary.BigEndian.PutUint32(data[176:180], 0xFFFFFFFF)
	binary.BigEndian.PutUint32(data[180:184], 0xFFFFFFFF)
	if opts.mobiVersion == 8 {
		binary.BigEndian.PutUint32(data[0xC0:0xC4], opts.fdstIdx)
		binary.BigEndian.PutUint32(data[0xC4:0xC8], opts.fdstCount)
		binary.BigEndian.PutUint32(data[0xF8:0xFC], opts.divIdx)
		binary.BigEndian.PutUint32(data[0xFC:0x100], opts.skelIdx)
	}
	binary.BigEndian.PutUint16(data[242:244], uint16(opts.extraRecordFlags))
	if len(exth) > 0 {
		copy(data[int(opts.headerLength)+PALM_DOC_HEADER_SIZE:], exth)
	}
	return recordSpec{data: data}
}

// makeExthRecord serialises EXTH records (type -> value) into an EXTH block.
func makeExthRecord(records map[uint32][]byte) []byte {
	if len(records) == 0 {
		return nil
	}
	types := make([]uint32, 0, len(records))
	for typ := range records {
		types = append(types, typ)
	}
	for i := 1; i < len(types); i++ {
		for j := i; j > 0 && types[j-1] > types[j]; j-- {
			types[j-1], types[j] = types[j], types[j-1]
		}
	}

	body := make([]byte, 0, 64)
	for _, typ := range types {
		val := records[typ]
		rec := make([]byte, 8+len(val))
		binary.BigEndian.PutUint32(rec[0:4], typ)
		binary.BigEndian.PutUint32(rec[4:8], uint32(8+len(val)))
		copy(rec[8:], val)
		body = append(body, rec...)
	}

	out := make([]byte, 12+len(body))
	copy(out[0:4], "EXTH")
	binary.BigEndian.PutUint32(out[4:8], uint32(12+len(body)))
	binary.BigEndian.PutUint32(out[8:12], uint32(len(types)))
	copy(out[12:], body)
	return out
}

// makeWordValue encodes a 4-byte EXTH value (an integer field).
func makeWordValue(v uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return b
}

// makeFDSTRecord builds an FDST (flow table) record.
func makeFDSTRecord(flows [][2]int) []byte {
	data := make([]byte, 12+len(flows)*8)
	copy(data[0:4], "FDST")
	binary.BigEndian.PutUint32(data[4:8], 12)
	binary.BigEndian.PutUint32(data[8:12], uint32(len(flows)))
	for i, flow := range flows {
		binary.BigEndian.PutUint32(data[12+i*8:12+i*8+4], uint32(flow[0]))
		binary.BigEndian.PutUint32(data[12+i*8+4:12+i*8+8], uint32(flow[1]))
	}
	return data
}

// makeMalformedFDSTRecord builds an FDST record whose section count exceeds the
// record, i.e. the kind of corruption that used to panic the parser.
func makeMalformedFDSTRecord() []byte {
	data := make([]byte, 12)
	copy(data[0:4], "FDST")
	binary.BigEndian.PutUint32(data[4:8], 12)
	binary.BigEndian.PutUint32(data[8:12], 999)
	return data
}

// tagxDef mirrors the parts of calibre's TagX tuple the fixtures need: the tag
// id and how many values an entry carries. Every tag gets its own control byte
// holding a single bit, so the fixtures never need multi-bit masks.
type tagxDef struct {
	tag         uint8
	numOfValues uint8
}

// controlByte is the value a fixture entry writes for every tag it carries.
const controlByte = 0x01

// indxTagRef names a tag by its index in the index's tagx list.
type indxTagRef int

// indxEntry is one index entry: an identifier plus the values it carries.
// A tag whose value is a singleton appears once; tag 6 (start, length) appears
// twice, in order.
type indxEntry struct {
	ident  string
	values map[indxTagRef][]int
}

// encVarint encodes a Mobipocket variable-width integer the way decInt reads it
// back (7 bits per byte, high bit set on the final byte).
func encVarint(v int) []byte {
	if v < 0 {
		v = 0
	}
	var groups []byte
	for {
		groups = append([]byte{byte(v & 0x7F)}, groups...)
		v >>= 7
		if v == 0 {
			break
		}
	}
	groups[len(groups)-1] |= 0x80
	return groups
}

// makeTagxSection builds the TAGX section: header plus one tag definition and
// one eof terminator per tag, which is what calibre's get_tag_map expects when
// every tag owns a control byte of its own. The terminator entry consumes the
// control byte group without decoding a tag.
func makeTagxSection(tags []tagxDef) []byte {
	entries := make([]byte, 0, len(tags)*8)
	for _, tg := range tags {
		entries = append(entries, tg.tag, tg.numOfValues, controlByte, 0x00)
		entries = append(entries, 0x00, 0x00, 0x00, 0x01) // eof terminator
	}

	data := make([]byte, 12+len(entries))
	copy(data[0:4], "TAGX")
	binary.BigEndian.PutUint32(data[4:8], uint32(len(data)))  // first entry offset
	binary.BigEndian.PutUint32(data[8:12], uint32(len(tags))) // control byte count
	copy(data[12:], entries)
	return data
}

// makeIndxEntryRecord builds one INDX record holding a single entry: INDX
// header, TAGX section, entry data (ident + control bytes + values), IDXT.
func makeIndxEntryRecord(tags []tagxDef, entry indxEntry) []byte {
	tagx := makeTagxSection(tags)

	ident := append([]byte{byte(len(entry.ident))}, []byte(entry.ident)...)
	body := make([]byte, len(tags)) // one control byte per tag
	values := make([]byte, 0, 16)
	for i := range tags {
		for _, v := range entry.values[indxTagRef(i)] {
			body[i] = controlByte
			values = append(values, encVarint(v)...)
		}
	}
	entryData := append(append(append([]byte{}, ident...), body...), values...)

	headerLen := 4 + 45*4
	tagxOff := headerLen
	entryOff := tagxOff + len(tagx)
	idxtOff := entryOff + len(entryData)

	data := make([]byte, idxtOff+4+2+2)
	copy(data[0:4], "INDX")
	binary.BigEndian.PutUint32(data[4:8], uint32(len(data))) // len
	binary.BigEndian.PutUint32(data[20:24], uint32(idxtOff)) // start (IDXT offset)
	binary.BigEndian.PutUint32(data[24:28], 1)               // count
	binary.BigEndian.PutUint32(data[28:32], uint32(UTF8))    // code
	binary.BigEndian.PutUint32(data[180:184], uint32(tagxOff))

	copy(data[tagxOff:], tagx)
	copy(data[entryOff:], entryData)
	copy(data[idxtOff:], "IDXT")
	binary.BigEndian.PutUint16(data[idxtOff+4:idxtOff+6], uint16(entryOff))
	return data
}

// makeIndxHeaderRecord builds the leading INDX record: it carries the TAGX
// section the entry records are parsed with and the number of entry records
// that follow, like the real multi-record INDX layout.
func makeIndxHeaderRecord(tags []tagxDef, entryCount int) []byte {
	tagx := makeTagxSection(tags)
	headerLen := 4 + 45*4
	tagxOff := headerLen
	idxtOff := tagxOff + len(tagx)

	data := make([]byte, idxtOff+4+2*entryCount)
	copy(data[0:4], "INDX")
	binary.BigEndian.PutUint32(data[4:8], uint32(len(data)))
	binary.BigEndian.PutUint32(data[20:24], uint32(idxtOff))    // start
	binary.BigEndian.PutUint32(data[24:28], uint32(entryCount)) // count
	binary.BigEndian.PutUint32(data[28:32], uint32(UTF8))       // code
	binary.BigEndian.PutUint32(data[180:184], uint32(tagxOff))

	copy(data[tagxOff:], tagx)
	copy(data[idxtOff:], "IDXT")
	return data
}

// makeIndxSection builds a complete INDX index: the leading header record
// followed by one record per entry, ready to be appended to the PDB records.
func makeIndxSection(tags []tagxDef, entries []indxEntry) [][]byte {
	out := [][]byte{makeIndxHeaderRecord(tags, len(entries))}
	for _, e := range entries {
		out = append(out, makeIndxEntryRecord(tags, e))
	}
	return out
}

// makeCNCXRecord encodes strings as an uncompiled NCX (CNCX) record: each entry
// is a variable-width length followed by the string bytes. The absolute offset
// of an entry in record n is n*0x10000 + its position within the record.
func makeCNCXRecord(lines ...string) []byte {
	out := make([]byte, 0, 64)
	for _, s := range lines {
		out = append(out, encVarint(len(s))...)
		out = append(out, s...)
	}
	return out
}

// binaryPutUint32 writes a big-endian uint32 at off, panicking when off is out
// of range (fixture code only).
func binaryPutUint32(b []byte, off int, v uint32) {
	binary.BigEndian.PutUint32(b[off:off+4], v)
}
