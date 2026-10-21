package mobi

import (
	"fmt"
)

// CNCX maps absolute string offset to string.
type CNCX map[int]string

// parseCNCX parses compiled NCX records.
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
					break
				}
				data := raw[start:end]
				var s string
				enc := UTF8
				if codec == "cp1252" {
					enc = CP1252
				}
				s = decodeString(data, enc)
				cncx[pos+recordOffset] = s
			}
			if length == 0 {
				pos += consumed
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

func (c CNCX) Get(offset int) (string, bool) {
	s, ok := c[offset]
	return s, ok
}

var _ = fmt.Sprintf
