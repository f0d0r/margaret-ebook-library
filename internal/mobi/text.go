package mobi

import (
	"bytes"
	"fmt"

	"github.com/f0d0r/margaret-ebook-library/book"
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
// It mirrors calibre's MobiReader.extract_text(offset=...):
// text_sections = [text_section(i) for i in range(offset, min(records+offset, len(sections)))].
// Like calibre, the FirstTextRecord header field is intentionally ignored:
// some real-world MOBI6 files (e.g. calibre-generated) leave it as the
// 0xFFFF sentinel, which must not be treated as a record index.
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

	first := max(offset, 0)
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

		dec, err := decompressTextSection(section, mobi.Compression, huff, remainingBudget(maxSize, len(out)), i)
		if err != nil {
			return nil, err
		}

		if maxSize > 0 && int64(len(out)+len(dec)) > maxSize {
			return nil, book.ErrLimitExceeded
		}
		out = append(out, dec...)
	}

	return finishText(out, mobi.TextEncoding), nil
}

// extractPalmDocText returns the full text of a pure PalmDOC book.
// Unlike MOBI6 it starts inside record 0 (past the 16-byte header) and
// reads every record: PalmDOC has no image, HUFF, FCIS or FLIS records,
// and real-world files set textRecordCount one short, so bounding by it
// truncates mid-sentence. textLength is only a sanity bound (padding can
// make the result slightly longer), never a truncation point.
func extractPalmDocText(pdbDb *PdbDb, mobi *Mobi, maxSize int64) ([]byte, error) {
	switch mobi.Compression {
	case CompressionNone, 0, CompressionPalmDOC, CompressionPalmDOCAlt:
		// handled per record below
	default:
		return nil, fmt.Errorf("unsupported PalmDOC compression type %d", mobi.Compression)
	}

	if len(pdbDb.PdbRecords) == 0 {
		return nil, fmt.Errorf("no records in PDB database")
	}

	var out []byte
	for i := range pdbDb.PdbRecords {
		data, err := pdbDb.PdbRecords[i].Data()
		if err != nil {
			return nil, fmt.Errorf("failed to read text record %d: %w", i, err)
		}
		section := data
		if i == 0 {
			if len(data) < PALM_DOC_HEADER_SIZE {
				return nil, fmt.Errorf("pdb record 0 is too short for PalmDOC header")
			}
			section = data[PALM_DOC_HEADER_SIZE:]
		}

		dec, err := decompressTextSection(section, mobi.Compression, nil, remainingBudget(maxSize, len(out)), i)
		if err != nil {
			return nil, err
		}

		if maxSize > 0 && int64(len(out)+len(dec)) > maxSize {
			return nil, book.ErrLimitExceeded
		}
		out = append(out, dec...)
	}

	return finishText(out, mobi.TextEncoding), nil
}

// finishText applies the shared trailing hygiene to concatenated
// decompressed text: a trailing '#' is removed, then NUL and
// encoding-specific control bytes are dropped.
func finishText(out []byte, enc TextEncodingType) []byte {
	if len(out) > 0 && out[len(out)-1] == '#' {
		out = out[:len(out)-1]
	}
	return cleanDecompressedText(out, enc)
}

// remainingBudget returns the decompression budget left for one record:
// maxSize minus the bytes gathered so far, or -1 for unlimited.
// A non-positive maxSize disables the limit.
func remainingBudget(maxSize int64, have int) int64 {
	if maxSize <= 0 {
		return -1
	}
	return max(maxSize-int64(have), 0)
}

// decompressTextSection decompresses one text record section according to
// the book's compression type, shared by the MOBI and PalmDOC extractors.
func decompressTextSection(section []byte, compression CompressionType, huff *HuffCdicReader, remain int64, idx int) ([]byte, error) {
	var dec []byte
	var err error
	switch compression {
	case CompressionPalmDOC, CompressionPalmDOCAlt:
		dec, err = DecompressPalmDoc(section, remain)
	case CompressionHUFF:
		if huff == nil {
			return nil, fmt.Errorf("missing huffman table for text record %d", idx)
		}
		dec, err = huff.Unpack(section, remain)
	default:
		dec = section
	}
	if err != nil {
		return nil, fmt.Errorf("failed to decompress text record %d: %w", idx, err)
	}
	return dec, nil
}

// cleanDecompressedText applies the shared calibre-parity hygiene to
// decompressed text: a trailing '#' is removed, NUL bytes are dropped,
// and for cp1252 books the record separator (0x1E) and start-of-text
// (0x02) control bytes are removed.
func cleanDecompressedText(out []byte, enc TextEncodingType) []byte {
	out = bytes.ReplaceAll(out, []byte{0}, nil)
	if enc == CP1252 {
		out = bytes.ReplaceAll(out, []byte{0x1e}, nil)
		out = bytes.ReplaceAll(out, []byte{0x02}, nil)
	}
	return out
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
