package mobi

import (
	"fmt"
)

// CNCX mirrors calibre.mobi.reader.index.CNCX.
// It maps absolute string offset (record_offset + pos) -> string.
type CNCX map[int]string

// parseCNCX parses the compiled NCX records.
// records are raw bytes of each CNCX record (sections[off: off+ncncx]).
// codec is "utf-8" or "cp1252".
func parseCNCX(records [][]byte, codec string) (CNCX, error) {
	cncx := make(CNCX)
	recordOffset := 0
	for _, raw := range records {
		pos := 0
		for pos < len(raw) {
			length, consumed := decInt(raw[pos:], true)
			if consumed == 0 {
				break
			}
			if length > 0 {
				start := pos + consumed
				end := start + length
				if end > len(raw) {
					// calibre just prints warning and breaks; we treat as error but continue
					break
				}
				data := raw[start:end]
				var s string
				// decode via codec
				enc := UTF8
				if codec == "cp1252" {
					enc = CP1252
				}
				// Need to handle ordt_map? CNCX doesn't use ordt_map in calibre; simple decode.
				s = decodeString(data, enc)
				// calibre checks for unknown format printing, we just store decoded string
				// If decode fails, store hex representation (not needed).
				cncx[pos+recordOffset] = s
				// If length==0, still advance?
			}
			// calibre: pos += consumed + length
			// But we already handled length>0; if length==0, just advance by consumed
			if length == 0 {
				pos += consumed
				// avoid infinite loop: if consumed==0 break
				if consumed == 0 {
					break
				}
				continue
			}
			pos += consumed + length
		}
		recordOffset += 0x10000
	}
	return cncx, nil
}

// getCNCX is a helper that returns empty CNCX on nil.
func (c CNCX) Get(offset int) (string, bool) {
	s, ok := c[offset]
	return s, ok
}

// Ensure that parseCNCX handles empty records correctly.
var _ = fmt.Sprintf
