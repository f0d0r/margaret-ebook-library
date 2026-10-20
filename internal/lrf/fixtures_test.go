package lrf

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"testing"
	"unicode/utf16"
)

// testObject describes one object of a synthetic LRF file.
type testObject struct {
	id   uint32
	typ  uint16
	body []byte
}

// encodeTestObject encodes an object start tag, its body and an object end
// tag.
func encodeTestObject(obj testObject) []byte {
	var buf []byte
	buf = binary.LittleEndian.AppendUint16(buf, 0xF500)
	buf = binary.LittleEndian.AppendUint32(buf, obj.id)
	buf = binary.LittleEndian.AppendUint16(buf, obj.typ)
	buf = append(buf, obj.body...)
	buf = binary.LittleEndian.AppendUint16(buf, tagObjectEnd)
	return buf
}

// buildLrfBook builds a valid LRF container whose object table holds the
// given objects, with the given root object id.
func buildLrfBook(t *testing.T, root uint32, objects ...testObject) []byte {
	t.Helper()
	return appendObjectsAndTable(t, buildLrf(t, 1000, []byte("<Info/>")), root, objects...)
}

// appendObjectsAndTable appends the encoded objects and the object table to
// an LRF container image and patches the header fields.
func appendObjectsAndTable(t *testing.T, data []byte, root uint32, objects ...testObject) []byte {
	t.Helper()

	type row struct {
		id        uint32
		off, size uint32
	}
	var rows []row
	for _, obj := range objects {
		rows = append(rows, row{obj.id, uint32(len(data)), 0})
		data = append(data, encodeTestObject(obj)...)
		rows[len(rows)-1].size = uint32(len(data)) - rows[len(rows)-1].off
	}
	tableOffset := len(data)
	for _, r := range rows {
		data = binary.LittleEndian.AppendUint32(data, r.id)
		data = binary.LittleEndian.AppendUint32(data, r.off)
		data = binary.LittleEndian.AppendUint32(data, r.size)
		data = binary.LittleEndian.AppendUint32(data, 0)
	}
	binary.LittleEndian.PutUint64(data[offNumberOfObjects:], uint64(len(rows)))
	binary.LittleEndian.PutUint64(data[offObjectTableOffset:], uint64(tableOffset))
	binary.LittleEndian.PutUint32(data[offRootObjectID:], root)
	return data
}

// tTag encodes a tag with the given full id and payload bytes.
func tTag(tag uint16, payload ...byte) []byte {
	return append(binary.LittleEndian.AppendUint16(nil, tag), payload...)
}

// tDword encodes a tag whose payload is one u32.
func tDword(tag uint16, value uint32) []byte {
	return tTag(tag, byte(value), byte(value>>8), byte(value>>16), byte(value>>24))
}

// tTypeOne encodes a type_one tag: a u16 count followed by u32 values.
func tTypeOne(tag uint16, values ...uint32) []byte {
	buf := tTag(tag, byte(len(values)), byte(len(values)>>8))
	for _, v := range values {
		buf = binary.LittleEndian.AppendUint32(buf, v)
	}
	return buf
}

// tString encodes a string tag holding UTF-16LE text.
func tString(tag uint16, s string) []byte {
	units := utf16.Encode([]rune(s))
	buf := tTag(tag, byte(len(units)*2), byte(len(units)*2>>8))
	for _, u := range units {
		buf = binary.LittleEndian.AppendUint16(buf, u)
	}
	return buf
}

// tStream encodes the stream tags of an object: flags, size, payload and end
// tag.
func tStream(flags uint16, payload []byte) []byte {
	buf := tTag(tagStreamFlags, byte(flags), byte(flags>>8))
	buf = append(buf, tDword(tagStreamSize, uint32(len(payload)))...)
	buf = append(buf, tTag(tagStreamStart)...)
	buf = append(buf, payload...)
	buf = append(buf, tTag(tagStreamEnd)...)
	return buf
}

// tPutObj builds a Canvas PutObj tag: position plus the referenced object id.
func tPutObj(x, y uint16, ref uint32) []byte {
	buf := binary.LittleEndian.AppendUint16(nil, x)
	buf = binary.LittleEndian.AppendUint16(buf, y)
	buf = binary.LittleEndian.AppendUint32(buf, ref)
	return tTag(tagPutObj, buf...)
}

// utf16LEBytes encodes a string as raw UTF-16LE bytes without a BOM.
func utf16LEBytes(s string) []byte {
	var buf []byte
	for _, u := range utf16.Encode([]rune(s)) {
		buf = binary.LittleEndian.AppendUint16(buf, u)
	}
	return buf
}

// zlibStreamPayload compresses data with a u32 uncompressed-size prefix, as
// LRF stream writers do.
func zlibStreamPayload(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	buf.Write(binary.LittleEndian.AppendUint32(nil, uint32(len(data))))
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// scrambleTestBytes applies the LRF scrambling transform to payload, capping
// the region at max when max > 0.
func scrambleTestBytes(payload []byte, xorKey uint16, max int) []byte {
	key := byte(xorKey & 0xFF)
	if key == 0 || key > 0xF0 {
		return payload
	}
	key = byte(len(payload)%int(key)) + 0xF
	n := len(payload)
	if max > 0 && n > max {
		n = max
	}
	out := bytes.Clone(payload)
	for i := 0; i < n; i++ {
		out[i] ^= key
	}
	return out
}
