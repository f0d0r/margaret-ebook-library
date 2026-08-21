package mobi

import "fmt"

// decInt decodes a Mobipocket variable-width integer (vwi).
// See calibre/src/calibre/ebooks/mobi/utils.py decint / encint.
// If forward is true, bytes are read from the start of raw (TAGX/INDX use).
// If forward is false, bytes are read from the end (trailing entries – not used here).
// Returns value and number of bytes consumed.
func decInt(raw []byte, forward bool) (int, int) {
	if len(raw) == 0 {
		return 0, 0
	}
	val := 0
	byts := make([]byte, 0, 8)
	src := raw
	if !forward {
		// reverse copy for backward decoding
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
		// reverse byts back
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

// decodeString mirrors calibre.mobi.utils.decode_string for INDX entries.
// It reads a length-prefixed string: first byte is length via decInt? No, for INDX it's different:
// In utils.decode_string: (length,) = struct.unpack(b'>B', raw[0:1]); raw = raw[1:1+length]
// But for varint length in CNCX/index, we use decInt.
// This helper decodes INDX ident: length is single byte? Actually utils.decode_string uses '>B'.
// For index parsing we need the calibre decode_string that uses '>B' for ident length.
// We implement both variants.
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
	// For ordt_map case, caller handles mapping; we just decode via codec.
	// Use decodeString helper for actual bytes.
	// We need to decode with charset; for now we support utf-8 and cp1252.
	// Use existing decodeString function (lowercase) that already handles CP1252.
	// But we need TextEncodingType: infer from codec string.
	enc := UTF8
	if codec == "cp1252" {
		enc = CP1252
	}
	s := decodeString(data, enc)
	return s, consumed, nil
}
