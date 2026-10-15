package mobi

import (
	"bytes"
	"fmt"

	"github.com/f0d0r/margaret-ebook-library/book"
	compressutil "github.com/f0d0r/margaret-ebook-library/internal/compress"
	"github.com/f0d0r/margaret-ebook-library/internal/pdb"
)

// extractText returns the raw MOBI6 markup, mirroring calibre's mobi6.py extract_text.
func extractText(pdbDb *pdb.PdbDb, mobi *Mobi, maxSize int64) ([]byte, error) {
	return extractTextWithOffset(pdbDb, mobi, 1, maxSize)
}

// extractTextWithOffset is the offset-aware variant (1 for MOBI6, kf8_boundary+2 for joint KF8).
// FirstTextRecord is ignored like calibre: real-world files may leave it as the 0xFFFF sentinel.
func extractTextWithOffset(pdbDb *pdb.PdbDb, mobi *Mobi, offset int, maxSize int64) ([]byte, error) {
	var huff *HuffCdicReader
	switch mobi.Compression {
	case CompressionNone, 0:
	case CompressionPalmDOC:
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

// extractPalmDocText returns the full PalmDOC text starting past the record 0 header.
// It reads every record: textRecordCount may be one short, textLength is only a sanity bound.
func extractPalmDocText(pdbDb *pdb.PdbDb, mobi *Mobi, maxSize int64) ([]byte, error) {
	switch mobi.Compression {
	case CompressionNone, 0, CompressionPalmDOC, CompressionPalmDOCAlt:
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

func finishText(out []byte, enc TextEncodingType) []byte {
	if len(out) > 0 && out[len(out)-1] == '#' {
		out = out[:len(out)-1]
	}
	return cleanDecompressedText(out, enc)
}

func remainingBudget(maxSize int64, have int) int64 {
	if maxSize <= 0 {
		return -1
	}
	return max(maxSize-int64(have), 0)
}

func decompressTextSection(section []byte, compression CompressionType, huff *HuffCdicReader, remain int64, idx int) ([]byte, error) {
	var dec []byte
	var err error
	switch compression {
	case CompressionPalmDOC, CompressionPalmDOCAlt:
		dec, err = compressutil.DecompressPalmDoc(section, remain)
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

// cleanDecompressedText applies calibre-parity hygiene to decompressed text.
func cleanDecompressedText(out []byte, enc TextEncodingType) []byte {
	out = bytes.ReplaceAll(out, []byte{0}, nil)
	if enc == CP1252 {
		out = bytes.ReplaceAll(out, []byte{0x1e}, nil)
		out = bytes.ReplaceAll(out, []byte{0x02}, nil)
	}
	return out
}

// newHuffReader builds a HuffCdicReader, mirroring calibre's HuffReader.
func newHuffReader(pdbDb *pdb.PdbDb, mobi *Mobi) (*HuffCdicReader, error) {
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

// sizeofTrailingEntries mirrors calibre's sizeof_trailing_entries.
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

// sizeofTrailingEntry decodes one backward-encoded trailing entry, mirroring calibre.
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
