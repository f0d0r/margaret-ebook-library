package lit

import (
	"crypto/des"
	"encoding/binary"
	"fmt"

	"github.com/f0d0r/margaret-ebook-library/book"
)

// Sealed (level 1) and inscribed (level 3) books encrypt sections with a
// content-derived DES key, transparently to the reader: only level 5
// ("owner exclusive", passport-locked) is truly unreadable. Key derivation
// hashes selected raw entries with Microsoft's SHA-1 variant (standard
// compression, custom initial state) and folds the digest into 8 bytes.

// drmLevel reports the protection level from directory entries: 5, 3, 1 or
// 0 (unprotected).
func drmLevel(entries map[string]directoryEntry) int {
	if _, ok := entries["/DRMStorage/Licenses/EUL"]; ok {
		return 5
	}
	if _, ok := entries["/DRMStorage/DRMBookplate"]; ok {
		return 3
	}
	if _, ok := entries["/DRMStorage/DRMSealed"]; ok {
		return 1
	}
	return 0
}

// desBookKey derives and unwraps the section DES key for protection levels
// 1 and 3.
func (c *container) desBookKey() ([]byte, error) {
	if c.desKey != nil {
		return c.desKey, nil
	}
	level := drmLevel(c.entries)
	if level != 1 && level != 3 {
		return nil, fmt.Errorf("lit: no decodable DRM level: %w", book.ErrDRM)
	}
	names := []string{metaEntry, "/DRMStorage/DRMSource"}
	if level == 3 {
		names = append(names, "/DRMStorage/DRMBookplate")
	}
	// The hashed stream is continuous: two leading zero bytes, then the
	// raw files back to back, the tail zero-padded to a 64-byte block.
	// (Per-file padding, as opposed to this, yields a different digest.)
	var stream []byte
	stream = append(stream, 0, 0)
	for _, name := range names {
		data, err := c.getFile(name)
		if err != nil {
			return nil, err
		}
		stream = append(stream, data...)
	}
	if rem := len(stream) % 64; rem != 0 {
		stream = append(stream, make([]byte, 64-rem)...)
	}
	h := newMsSHA1()
	h.write(stream)
	sum := h.sum()
	var contentKey [8]byte
	for i, d := range sum {
		contentKey[i%8] ^= d
	}
	sealed, err := c.getFile("/DRMStorage/DRMSealed")
	if err != nil {
		return nil, err
	}
	bookKey := desDecrypt(sealed, contentKey[:])
	if len(bookKey) == 0 || bookKey[0] != 0 {
		return nil, fmt.Errorf("lit: unable to decrypt title key: %w", book.ErrDRM)
	}
	c.desKey = append([]byte(nil), bookKey[1:9]...)
	return c.desKey, nil
}

// desDecrypt decrypts data with single-DES ECB, zero-padding to a multiple
// of the block size like the reference does.
func desDecrypt(data, key []byte) []byte {
	if rem := len(data) % 8; rem != 0 {
		data = append(append([]byte(nil), data...), make([]byte, 8-rem)...)
	}
	block, err := des.NewCipher(key)
	if err != nil {
		return nil
	}
	out := make([]byte, len(data))
	for i := 0; i < len(data); i += 8 {
		block.Decrypt(out[i:], data[i:])
	}
	return out
}

// msSHA1 is SHA-1 with Microsoft's initial state, as used for LIT key
// derivation. Compression, constants and padding are standard.
type msSHA1 struct {
	h [5]uint32
	x [64]byte
	n int
	l uint64
}

func newMsSHA1() *msSHA1 {
	return &msSHA1{h: [5]uint32{0x32107654, 0x23016745, 0xC4E680A2, 0xDC679823, 0xD0857A34}}
}

func (s *msSHA1) write(p []byte) {
	s.l += uint64(len(p)) * 8
	for len(p) > 0 {
		if s.n == 64 {
			s.block()
			s.n = 0
		}
		m := copy(s.x[s.n:], p)
		s.n += m
		p = p[m:]
	}
}

func (s *msSHA1) sum() (out [20]byte) {
	// Standard Merkle-Damgård padding on a copy.
	c := *s
	if c.n == 64 {
		c.block()
		c.n = 0
	}
	c.x[c.n] = 0x80
	c.n++
	if c.n > 56 {
		for c.n < 64 {
			c.x[c.n] = 0
			c.n++
		}
		c.block()
		c.n = 0
	}
	for c.n < 56 {
		c.x[c.n] = 0
		c.n++
	}
	binary.BigEndian.PutUint64(c.x[56:], s.l)
	c.block()
	for i, v := range c.h {
		binary.BigEndian.PutUint32(out[i*4:], v)
	}
	return out
}

// msRoundFunc selects the compression function for round t, including
// Microsoft's swapped rounds.
func msRoundFunc(t int, b, c, d uint32) uint32 {
	switch t {
	case 3, 10, 15, 51:
		return b ^ c ^ d
	case 6, 42:
		return (b + c) ^ c
	case 26, 68:
		return (b & (c ^ d)) ^ d
	case 31:
		return ((b | c) & d) | (b & c)
	}
	switch t / 20 {
	case 0:
		return (b & (c ^ d)) ^ d
	case 1, 3:
		return b ^ c ^ d
	default:
		return ((b | c) & d) | (b & c)
	}
}

func (s *msSHA1) block() {
	var w [80]uint32
	for i := 0; i < 16; i++ {
		w[i] = binary.BigEndian.Uint32(s.x[i*4:])
	}
	for i := 16; i < 80; i++ {
		v := w[i-3] ^ w[i-8] ^ w[i-14] ^ w[i-16]
		w[i] = v<<1 | v>>31
	}
	a, b, c, d, e := s.h[0], s.h[1], s.h[2], s.h[3], s.h[4]
	for i := 0; i < 80; i++ {
		var k uint32
		switch i / 20 {
		case 0:
			k = 0x5A827999
		case 1:
			k = 0x6ED9EBA1
		case 2:
			k = 0x8F1BBCDC
		default:
			k = 0xCA62C1D6
		}
		t := rotl(a, 5) + msRoundFunc(i, b, c, d) + e + w[i] + k
		e, d, c, b, a = d, c, rotl(b, 30), a, t
	}
	s.h[0] += a
	s.h[1] += b
	s.h[2] += c
	s.h[3] += d
	s.h[4] += e
}

func rotl(v uint32, n uint) uint32 {
	return v<<n | v>>(32-n)
}
