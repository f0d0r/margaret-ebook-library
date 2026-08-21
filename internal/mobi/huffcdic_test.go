package mobi

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// buildTestHuff builds a minimal HUFF table in which every 8-bit symbol maps
// to a terminal code that resolves to dictionary index 0.
func buildTestHuff() []byte {
	huff := make([]byte, 16+256*4+64*4)
	copy(huff[0:8], "HUFF\x00\x00\x00\x18")
	binary.BigEndian.PutUint32(huff[8:12], 16)
	binary.BigEndian.PutUint32(huff[12:16], 16+256*4)
	for i := range 256 {
		binary.BigEndian.PutUint32(huff[16+i*4:], 0xFF88)
	}
	return huff
}

// buildTestCdic builds a CDIC dictionary with a single terminal entry "Test".
// The entry table at offset 16 holds byte offsets relative to the table start:
// the single block lives at offset 2, i.e. byte 18.
func buildTestCdic() []byte {
	cdic := make([]byte, 0, 24)
	cdic = append(cdic, "CDIC\x00\x00\x00\x10"...)
	cdic = append(cdic, 0, 0, 0, 1, 0, 0, 0, 0) // phrases=1, bits=0
	cdic = append(cdic, 0x00, 0x02)             // table: first block at offset 2
	cdic = append(cdic, 0x80, 0x04)             // blen = 4 | 0x8000 (terminal)
	cdic = append(cdic, []byte("Test")...)
	return cdic
}

func TestHuffCdicUnpack(t *testing.T) {
	h := &HuffCdicReader{}
	if err := h.LoadHuff(buildTestHuff()); err != nil {
		t.Fatalf("LoadHuff: %v", err)
	}
	if err := h.LoadCdic(buildTestCdic()); err != nil {
		t.Fatalf("LoadCdic: %v", err)
	}
	out, err := h.Unpack(bytes.Repeat([]byte{0xFF}, 8), -1)
	if err != nil {
		t.Fatalf("Unpack: %v", err)
	}
	want := bytes.Repeat([]byte("Test"), 8)
	if !bytes.Equal(out, want) {
		t.Errorf("Unpack = %q, want %q", out, want)
	}
}

func TestHuffCdicBadHeaders(t *testing.T) {
	h := &HuffCdicReader{}
	if err := h.LoadHuff([]byte("BADH")); err == nil {
		t.Error("LoadHuff: expected error for bad header")
	}
	if err := h.LoadCdic([]byte("NOPE")); err == nil {
		t.Error("LoadCdic: expected error for bad header")
	}
}

func TestHuffCdicTruncatedTables(t *testing.T) {
	h := &HuffCdicReader{}
	if err := h.LoadHuff([]byte("HUFF\x00\x00\x00\x18\x00\x00\x00\x00")); err == nil {
		t.Error("LoadHuff: expected error for truncated table")
	}
	if err := h.LoadCdic([]byte("CDIC\x00\x00\x00\x10\x00\x00\x00\x01\x00\x00\x00\x00")); err == nil {
		t.Error("LoadCdic: expected error for truncated length table")
	}
}

// buildLongCodeHuff builds a HUFF table in which every code walks to
// codelen=32 (a 32-bit Huffman code) before resolving to dictionary index 0.
func buildLongCodeHuff() []byte {
	huff := make([]byte, 16+256*4+64*4)
	copy(huff[0:8], "HUFF\x00\x00\x00\x18")
	binary.BigEndian.PutUint32(huff[8:12], 16)
	binary.BigEndian.PutUint32(huff[12:16], 16+256*4)
	// dict1: codelen=1, term not set, for every top byte.
	for i := range 256 {
		binary.BigEndian.PutUint32(huff[16+i*4:], 1)
	}
	// dict2: force the mincode walk all the way to codelen=32.
	for i := range 32 {
		binary.BigEndian.PutUint32(huff[16+256*4+2*i*4:], 0xFFFFFFFF)
	}
	// mincode[32] = 0 (stops the walk), maxcode[32] = 0xFFFFFFFF (r = 0).
	binary.BigEndian.PutUint32(huff[16+256*4+62*4:], 0)
	binary.BigEndian.PutUint32(huff[16+256*4+63*4:], 0xFFFFFFFF)
	return huff
}

func TestHuffCdicLongCode(t *testing.T) {
	h := &HuffCdicReader{}
	if err := h.LoadHuff(buildLongCodeHuff()); err != nil {
		t.Fatalf("LoadHuff: %v", err)
	}
	if err := h.LoadCdic(buildTestCdic()); err != nil {
		t.Fatalf("LoadCdic: %v", err)
	}
	out, err := h.Unpack(bytes.Repeat([]byte{0xFF}, 8), -1)
	if err != nil {
		t.Fatalf("Unpack: %v", err)
	}
	want := bytes.Repeat([]byte("Test"), 2)
	if !bytes.Equal(out, want) {
		t.Errorf("Unpack = %q, want %q", out, want)
	}
}
