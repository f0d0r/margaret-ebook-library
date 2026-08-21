package mobi

import (
	"bytes"
	"fmt"

	"github.com/f0d0r/margaret-ebook-library/pkg/errs"
)

// extractText returns the raw HTML markup of a MOBI6 (KF7) book, obtained by
// concatenating and decompressing its text records. The algorithm mirrors
// calibre's mobi6.py extract_text: trailing-data entries are stripped from
// each record, every record is decompressed according to the book's
// compression type, the pieces are concatenated, a trailing '#' is removed,
// NUL bytes are dropped, and for cp1252-encoded books the record separator
// (\x1e) and start-of-text (\x02) control bytes are removed.
func extractText(pdbDb *PdbDb, mobi *Mobi, maxSize int64) ([]byte, error) {
	return extractTextWithOffset(pdbDb, mobi, 1, maxSize)
}

// extractTextWithOffset is the offset-aware variant used for KF8.
// For MOBI6, offset is 1 (calibre default). For joint MOBI6/KF8, KF8 text starts at kf8_boundary+2.
// It mirrors calibre's MobiReader.extract_text(offset=...).
func extractTextWithOffset(pdbDb *PdbDb, mobi *Mobi, offset int, maxSize int64) ([]byte, error) {
	var huff *HuffCdicReader
	switch mobi.Compression {
	case CompressionNone, 0:
		// identity unpacker
	case CompressionPalmDOC:
		// handled per record
	case CompressionHUFF:
		var err error
		huff, err = newHuffReader(pdbDb, mobi)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported compression type %d", mobi.Compression)
	}

	first := int(mobi.FirstTextRecord)
	if first <= 0 {
		first = offset
	} else {
		// For KF8, FirstTextRecord is relative to KF8 start? calibre uses offset directly, ignoring FirstTextRecord for KF8?
		// We follow calibre: text_sections = [text_section(i) for i in range(offset, min(records+offset, len))]
		// So if offset !=1, we use offset as base, not FirstTextRecord.
		if offset != 1 {
			first = offset
		}
	}
	first = min(first, len(pdbDb.PdbRecords))
	end := first + int(mobi.TextRecordCount)
	end = min(end, len(pdbDb.PdbRecords))

	var out []byte
	for i := first; i < end; i++ {
		data, err := pdbDb.PdbRecords[i].Data()
		if err != nil {
			return nil, fmt.Errorf("failed to read text record %d: %w", i, err)
		}
		section := data
		if trail := sizeofTrailingEntries(data, mobi.ExtraRecordFlags); trail > 0 {
			if trail >= len(data) {
				section = nil
			} else {
				section = data[:len(data)-trail]
			}
		}

		var dec []byte
		switch mobi.Compression {
		case CompressionPalmDOC:
			remain := int64(-1)
			if maxSize > 0 {
				remain = max(maxSize-int64(len(out)), 0)
			}
			dec, err = DecompressPalmDoc(section, remain)
			if err != nil {
				return nil, fmt.Errorf("failed to decompress text record %d: %w", i, err)
			}
		case CompressionHUFF:
			remain := int64(-1)
			if maxSize > 0 {
				remain = max(maxSize-int64(len(out)), 0)
			}
			dec, err = huff.Unpack(section, remain)
			if err != nil {
				return nil, fmt.Errorf("failed to decompress text record %d: %w", i, err)
			}
		default:
			dec = section
		}

		if maxSize > 0 && int64(len(out)+len(dec)) > maxSize {
			return nil, errs.ErrLimitExceeded
		}
		out = append(out, dec...)
	}

	if len(out) > 0 && out[len(out)-1] == '#' {
		out = out[:len(out)-1]
	}
	out = bytes.ReplaceAll(out, []byte{0}, nil)
	if mobi.TextEncoding == CP1252 {
		out = bytes.ReplaceAll(out, []byte{0x1e}, nil)
		out = bytes.ReplaceAll(out, []byte{0x02}, nil)
	}
	return out, nil
}

// newHuffReader builds a HuffCdicReader from the book's HUFF table record and
// its CDIC dictionary records, mirroring calibre's HuffReader.
func newHuffReader(pdbDb *PdbDb, mobi *Mobi) (*HuffCdicReader, error) {
	off := int(mobi.HuffmanRecordOffset)
	cnt := int(mobi.HuffmanRecordCount)
	if off < 0 || off >= len(pdbDb.PdbRecords) || cnt <= 0 || off+cnt > len(pdbDb.PdbRecords) {
		return nil, fmt.Errorf("invalid huffman record range %d..%d", off, off+cnt)
	}

	h := &HuffCdicReader{}
	huffData, err := pdbDb.PdbRecords[off].Data()
	if err != nil {
		return nil, fmt.Errorf("failed to read huffman table record: %w", err)
	}
	if err := h.LoadHuff(huffData); err != nil {
		return nil, fmt.Errorf("invalid huffman table: %w", err)
	}
	for i := off + 1; i < off+cnt; i++ {
		cdicData, err := pdbDb.PdbRecords[i].Data()
		if err != nil {
			return nil, fmt.Errorf("failed to read cdic record %d: %w", i, err)
		}
		if err := h.LoadCdic(cdicData); err != nil {
			return nil, fmt.Errorf("invalid cdic record %d: %w", i, err)
		}
	}
	return h, nil
}

// sizeofTrailingEntries returns the number of trailing-data bytes at the end
// of a text record as described by the MOBI ExtraRecordFlags field. The
// algorithm mirrors calibre's sizeof_trailing_entries.
func sizeofTrailingEntries(data []byte, extraFlags uint32) int {
	num := 0
	size := len(data)
	flags := extraFlags >> 1
	for flags != 0 {
		if flags&1 != 0 {
			num += sizeofTrailingEntry(data, size-num)
		}
		flags >>= 1
	}
	if extraFlags&1 != 0 {
		off := size - num - 1
		if off >= 0 && off < size {
			num += int(data[off]&0x3) + 1
		}
	}
	return min(num, size)
}

// sizeofTrailingEntry returns the size of a single trailing-data entry,
// encoded as a Mobipocket backward-encoded variable-width integer. In this
// encoding the least significant 7 bits are stored in the byte closest to the
// end of the record and only the highest-order byte has bit 8 set; reading
// backward, a byte with bit 0x80 set terminates the value. It mirrors
// calibre's sizeof_trailing_entry.
func sizeofTrailingEntry(data []byte, psize int) int {
	if psize <= 0 {
		return 0
	}
	bitpos, result := 0, 0
	done := false
	for !done && bitpos < 28 && psize > 0 {
		v := int(data[psize-1])
		result |= (v & 0x7F) << bitpos
		bitpos += 7
		psize--
		done = v&0x80 != 0
	}
	return result
}
