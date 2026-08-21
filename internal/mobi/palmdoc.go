package mobi

import "github.com/f0d0r/margaret-ebook-library/pkg/errs"

// DecompressPalmDoc decompresses a block compressed with PalmDOC (LZ77)
// compression, as used for the text records of a MOBI file. It mirrors the
// reference calibre cPalmdoc.decompress behaviour: malformed tokens (truncated
// literal runs or back-references whose distance exceeds the output produced
// so far) are silently skipped rather than reported as errors.
//
// remain is the remaining budget for decompressed bytes (maxSize - len(out) from
// the caller). If remain >=0, decompression fails with errs.ErrLimitExceeded
// when the output would exceed the remaining budget. Pass remain <0 for unlimited.
func DecompressPalmDoc(in []byte, remain int64) ([]byte, error) {
	capHint := len(in) * 2
	if remain >= 0 && int64(capHint) > remain {
		capHint = int(remain)
		if capHint < 0 {
			capHint = 0
		}
	}
	out := make([]byte, 0, capHint)
	p := 0
	for p < len(in) {
		c := int(in[p])
		p++
		switch {
		case c >= 1 && c <= 8:
			// Copy the next c bytes literally.
			if p+c > len(in) {
				c = len(in) - p
			}
			if remain >= 0 && int64(len(out)+c) > remain {
				return nil, errs.ErrLimitExceeded
			}
			out = append(out, in[p:p+c]...)
			p += c
		case c < 128:
			// Literal byte.
			if remain >= 0 && int64(len(out)+1) > remain {
				return nil, errs.ErrLimitExceeded
			}
			out = append(out, byte(c))
		case c >= 192:
			// Literal space plus the byte with bit 0x80 cleared.
			if remain >= 0 && int64(len(out)+2) > remain {
				return nil, errs.ErrLimitExceeded
			}
			out = append(out, ' ', byte(c^128))
		default:
			// Two-byte back-reference: distance m, length n. If the second
			// byte is missing or the distance exceeds the output produced so
			// far, the token is ignored.
			if p >= len(in) {
				continue
			}
			c = (c << 8) | int(in[p])
			p++
			m := (c >> 3) & 0x07ff
			n := (c & 7) + 3
			if m > len(out) {
				continue
			}
			if m > n {
				// No overlap between source and destination ranges.
				if remain >= 0 && int64(len(out)+n) > remain {
					return nil, errs.ErrLimitExceeded
				}
				out = append(out, out[len(out)-m:len(out)-m+n]...)
				continue
			}
			if m == 0 {
				if remain >= 0 && int64(len(out)+n) > remain {
					return nil, errs.ErrLimitExceeded
				}
				out = append(out, make([]byte, n)...)
				continue
			}
			// Overlapping copy: byte at distance m, re-evaluated as the
			// output grows, repeated n times.
			if remain >= 0 && int64(len(out)+n) > remain {
				return nil, errs.ErrLimitExceeded
			}
			for range n {
				out = append(out, out[len(out)-m])
			}
		}
	}
	return out, nil
}
