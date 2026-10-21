package mobi

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/f0d0r/margaret-ebook-library/internal/pdb"
	"github.com/f0d0r/margaret-ebook-library/internal/xml"
)

// isPalmDocDb reports whether the container holds a PalmDOC book (TEXTREAD ident).
func isPalmDocDb(pdbDb *pdb.PdbDb) bool {
	if pdbDb == nil {
		return false
	}
	return strings.ToUpper(pdbDb.Type+pdbDb.Creator) == pdbIdentPalmDoc
}

// readPalmDocHeader parses the 16-byte PalmDOC header from record 0.
// MOBI layout must not apply past byte 16: the record already carries text.
func readPalmDocHeader(pdbDb *pdb.PdbDb) (*Mobi, error) {
	if len(pdbDb.PdbRecords) == 0 {
		return nil, fmt.Errorf("no records in PDB database")
	}
	data, err := pdbDb.PdbRecords[0].Data()
	if err != nil {
		return nil, fmt.Errorf("failed to read Record 0: %w", err)
	}
	if len(data) < PALM_DOC_HEADER_SIZE {
		return nil, fmt.Errorf("pdb record 0 is too short for PalmDOC header")
	}

	mobi := newMobiDefaults()
	mobi.readBaseHeader(data)
	mobi.Type = MobiTypePalmDoc
	mobi.TextEncoding = CP1252
	mobi.MobiVersion = 1
	mobi.Codec = "cp1252"
	mobi.name = pdbDb.Name
	mobi.recordCount = pdbDb.NumberOfRecords
	return mobi, nil
}

// palmDocSniffSize bounds the HTML sniff window (OEB payloads open with <HTML><HEAD>).
const palmDocSniffSize = 4096

// isPalmDocHTML reports whether PalmDOC text looks like HTML (media type decision only; payload preserved as-is).
func isPalmDocHTML(data []byte) bool {
	trimmed := trimPalmDocPrefix(data)
	if len(trimmed) == 0 {
		return false
	}
	if len(trimmed) > palmDocSniffSize {
		trimmed = trimmed[:palmDocSniffSize]
	}
	for _, tag := range []string{"html", "head", "body"} {
		if xml.ScanOpenTag(trimmed, tag, true, false) {
			return true
		}
	}
	return false
}

// trimPalmDocPrefix drops leading padding for sniffing only.
func trimPalmDocPrefix(data []byte) []byte {
	data = bytes.TrimLeft(data, "\x0e\x00")
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	return bytes.TrimLeft(data, " \t\n\r\f\v")
}

// palmDocPrefixCap bounds the sniff/metadata prefix; decompression stops early at </metadata>.
const palmDocPrefixCap = 32 * 1024

// palmDocPrefixText gathers a best-effort sniff prefix; failures fall back to plain text, full errors surface from Open.
func palmDocPrefixText(pdbDb *pdb.PdbDb, mobi *Mobi) ([]byte, error) {
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
			return out, err
		}
		section := data
		if i == 0 {
			if len(data) < PALM_DOC_HEADER_SIZE {
				return out, fmt.Errorf("pdb record 0 is too short for PalmDOC header")
			}
			section = data[PALM_DOC_HEADER_SIZE:]
		}
		// Unlimited per-record budget: truncation below still yields a usable prefix.
		dec, err := decompressTextSection(section, mobi.Compression, nil, -1, i)
		if err != nil {
			return out, err
		}
		if int64(len(out)+len(dec)) > palmDocPrefixCap {
			out = append(out, dec[:palmDocPrefixCap-len(out)]...)
			return out, nil
		}
		out = append(out, dec...)
		if hasMetadataEnd(out) {
			return out, nil
		}
	}
	return out, nil
}

func hasMetadataEnd(out []byte) bool {
	return containsFold(out, []byte("</metadata>"))
}

// containsFold is a case-insensitive contains without allocating.
func containsFold(haystack, needle []byte) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if !foldEq(haystack[i], needle[0]) {
			continue
		}
		match := true
		for j := 1; j < len(needle); j++ {
			if !foldEq(haystack[i+j], needle[j]) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func foldEq(a, b byte) bool {
	if a == b {
		return true
	}
	if 'A' <= a && a <= 'Z' {
		a += 'a' - 'A'
	}
	if 'A' <= b && b <= 'Z' {
		b += 'a' - 'A'
	}
	return a == b
}
