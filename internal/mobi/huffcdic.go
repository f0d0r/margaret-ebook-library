package mobi

import (
	"encoding/binary"
	"fmt"

	"github.com/f0d0r/margaret-ebook-library/pkg/errs"
)

// huffDict holds the primary lookup entry for the top byte of a code.
type huffDict struct {
	codelen uint32
	term    bool
	maxcode uint64
}

// huffEntry is a single CDIC dictionary entry. When flag is set, slice holds
// literal text; otherwise slice is a compressed block to unpack recursively.
type huffEntry struct {
	slice []byte
	flag  bool
}

// HuffCdicReader decompresses HUFF/CDIC (Mobipocket Huffman) compressed
// records. It is a faithful port of the reference mobi_uncompress
// HuffcdicReader. Call LoadHuff with the HUFF table, LoadCdic (possibly
// multiple times) with the CDIC dictionaries, then Unpack on each record.
type HuffCdicReader struct {
	dict1      [256]huffDict
	mincode    [33]uint64
	maxcode    [33]uint64
	dictionary []huffEntry
}

// LoadHuff parses the HUFF table record.
func (h *HuffCdicReader) LoadHuff(huff []byte) error {
	if len(huff) < 8 || string(huff[0:8]) != "HUFF\x00\x00\x00\x18" {
		return fmt.Errorf("invalid huff header")
	}
	if len(huff) < 16 {
		return fmt.Errorf("invalid huff header: truncated offsets")
	}
	off1 := int(binary.BigEndian.Uint32(huff[8:12]))
	off2 := int(binary.BigEndian.Uint32(huff[12:16]))

	if off1+256*4 > len(huff) {
		return fmt.Errorf("invalid huff table: dict1 out of range")
	}
	for i := range 256 {
		v := binary.BigEndian.Uint32(huff[off1+i*4:])
		codelen := uint32(v & 0x1f)
		term := v&0x80 != 0
		maxcode := uint64(v>>8) + 1
		maxcode = (maxcode << (32 - codelen)) - 1
		h.dict1[i] = huffDict{codelen: codelen, term: term, maxcode: maxcode}
	}

	if off2+64*4 > len(huff) {
		return fmt.Errorf("invalid huff table: dict2 out of range")
	}
	for i := range 32 {
		codelen := uint32(i + 1)
		shift := 32 - codelen
		minval := uint64(binary.BigEndian.Uint32(huff[off2+2*i*4:]))
		maxval := uint64(binary.BigEndian.Uint32(huff[off2+(2*i+1)*4:]))
		h.mincode[codelen] = minval << shift
		h.maxcode[codelen] = (maxval+1)<<shift - 1
	}
	h.mincode[0] = 0
	h.maxcode[0] = 0xFFFFFFFF

	h.dictionary = nil
	return nil
}

// LoadCdic appends the entries of a CDIC dictionary record.
func (h *HuffCdicReader) LoadCdic(cdic []byte) error {
	if len(cdic) < 16 || string(cdic[0:8]) != "CDIC\x00\x00\x00\x10" {
		return fmt.Errorf("invalid cdic header")
	}
	phrases := int64(binary.BigEndian.Uint32(cdic[8:12]))
	bits := binary.BigEndian.Uint32(cdic[12:16])

	n := min(int64(1)<<bits, phrases-int64(len(h.dictionary)))
	if n < 0 {
		return fmt.Errorf("invalid cdic: too many dictionary entries")
	}
	if len(cdic) < 16+int(n)*2 {
		return fmt.Errorf("invalid cdic: truncated length table")
	}
	for i := range n {
		// Each entry in the table at offset 16 is a byte offset relative to
		// the start of the table, not a length. At that offset a 16-bit
		// length (with the high bit as a literal flag) precedes the slice.
		off := int(binary.BigEndian.Uint16(cdic[16+i*2:]))
		if 16+off+2 > len(cdic) {
			return fmt.Errorf("invalid cdic: entry %d out of range", i)
		}
		blen := binary.BigEndian.Uint16(cdic[16+off:])
		start := 18 + off
		end := start + int(blen&0x7fff)
		if end > len(cdic) {
			return fmt.Errorf("invalid cdic: entry %d out of range", i)
		}
		h.dictionary = append(h.dictionary, huffEntry{
			slice: cdic[start:end],
			flag:  blen&0x8000 != 0,
		})
	}
	return nil
}

// Unpack decompresses a single HUFF/CDIC compressed record.
// remain is the remaining budget for decompressed bytes. If remain >=0,
// decompression fails with errs.ErrLimitExceeded when the output would exceed
// the remaining budget. Pass remain <0 for unlimited.
func (h *HuffCdicReader) Unpack(data []byte, remain int64) ([]byte, error) {
	return h.unpack(data, remain, 0)
}

const maxHuffDepth = 32

func (h *HuffCdicReader) unpack(data []byte, remain int64, depth int) ([]byte, error) {
	if depth > maxHuffDepth {
		return nil, fmt.Errorf("huffcdic: recursion depth exceeded")
	}

	bitsleft := len(data) * 8
	// Pad into a fresh buffer: calibre does data += b'\0'*8 which allocates a
	// new immutable bytes object. Growing the input in place with append would
	// clobber the shared backing array of the CDIC record and corrupt other
	// dictionary slices that alias it.
	buf := make([]byte, len(data)+8)
	copy(buf, data)
	data = buf
	pos := 0
	x := binary.BigEndian.Uint64(data[pos:])
	n := 32
	capHint := len(data) * 2
	if remain >= 0 && int64(capHint) > remain {
		capHint = int(remain)
		capHint = max(capHint, 0)
	}
	out := make([]byte, 0, capHint)

	for {
		if n <= 0 {
			pos += 4
			if pos+8 > len(data) {
				return nil, fmt.Errorf("huffcdic: bitstream exhausted")
			}
			x = binary.BigEndian.Uint64(data[pos:])
			n += 32
		}
		code := uint32(x >> n)

		e := h.dict1[code>>24]
		codelen := int(e.codelen)
		term := e.term
		maxcode := e.maxcode
		if !term {
			for uint64(code) < h.mincode[codelen] {
				if codelen >= 32 {
					return nil, fmt.Errorf("huffcdic: code length overflow")
				}
				codelen++
			}
			maxcode = h.maxcode[codelen]
		}

		n -= codelen
		bitsleft -= codelen
		if bitsleft < 0 {
			break
		}
		if codelen == 0 {
			return nil, fmt.Errorf("huffcdic: zero code length")
		}

		r := int((maxcode - uint64(code)) >> uint(32-codelen))
		if r < 0 || r >= len(h.dictionary) {
			return nil, fmt.Errorf("huffcdic: dictionary index %d out of range", r)
		}
		entry := h.dictionary[r]
		if !entry.flag {
			h.dictionary[r] = huffEntry{}
			remForRec := remain
			if remain >= 0 {
				remForRec = max(remain-int64(len(out)), 0)
			}
			slice, err := h.unpack(entry.slice, remForRec, depth+1)
			if err != nil {
				return nil, err
			}
			entry = huffEntry{slice: slice, flag: true}
			h.dictionary[r] = entry
		}
		if remain >= 0 && int64(len(out)+len(entry.slice)) > remain {
			return nil, errs.ErrLimitExceeded
		}
		out = append(out, entry.slice...)
	}
	return out, nil
}
