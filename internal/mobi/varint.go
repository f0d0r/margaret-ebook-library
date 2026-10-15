package mobi

import "fmt"

// decInt decodes a Mobipocket variable-width integer (calibre decint).
// Forward reads from the start (TAGX/INDX); backward from the end.
func decInt(raw []byte, forward bool) (int, int) {
	if len(raw) == 0 {
		return 0, 0
	}
	val := 0
	byts := make([]byte, 0, 8)
	src := raw
	if !forward {
		tmp := make([]byte, len(raw))
		for i, b := range raw {
			tmp[len(raw)-1-i] = b
		}
		src = tmp
	}
	for _, bnum := range src {
		byts = append(byts, bnum&0x7F)
		if bnum&0x80 != 0 {
			break
		}
		if len(byts) >= 8 {
			break
		}
	}
	if !forward {
		for i, j := 0, len(byts)-1; i < j; i, j = i+1, j-1 {
			byts[i], byts[j] = byts[j], byts[i]
		}
	}
	for _, b := range byts {
		val <<= 7
		val |= int(b)
	}
	return val, len(byts)
}

// countSetBits mirrors calibre.utils.count_set_bits.
func countSetBits(num int) int {
	if num < 0 {
		num = -num
	}
	ans := 0
	for num > 0 {
		ans += num & 1
		num >>= 1
	}
	return ans
}

// decodeStringWithByteLength decodes a length-prefixed INDX string (calibre decode_string).
func decodeStringWithByteLength(raw []byte, codec string) (string, int, error) {
	if len(raw) == 0 {
		return "", 0, fmt.Errorf("empty raw for decodeString")
	}
	length := int(raw[0])
	if 1+length > len(raw) {
		return "", 0, fmt.Errorf("decodeString length %d exceeds raw %d", length, len(raw))
	}
	data := raw[1 : 1+length]
	consumed := 1 + length
	enc := UTF8
	if codec == "cp1252" {
		enc = CP1252
	}
	s := decodeString(data, enc)
	return s, consumed, nil
}
