package lrf

import (
	"encoding/binary"
	"fmt"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/compress"
)

const (
	streamFlagCompressed = 0x100
	streamFlagScrambled  = 0x200

	// scrambleMaxBytes caps the scrambled region of image, font and sound
	// streams.
	scrambleMaxBytes = 0x400
)

// objectStream extracts the stream of an object body: its flags and the raw
// (still scrambled or compressed) payload. ok is false when the object
// carries no stream.
func objectStream(body []byte) (int, []byte, bool, error) {
	sc := newTagScanner(body)
	for {
		id, payload, ok, err := sc.next()
		if err != nil {
			return 0, nil, false, err
		}
		if !ok {
			return 0, nil, false, nil
		}
		if id == tagStreamStart {
			return sc.flags, payload, true, nil
		}
	}
}

// decodeStream reverses the optional scrambling and zlib compression of a
// stream payload. partial marks streams whose scrambling only covers the
// leading bytes (images, fonts, sounds).
func (r *LrfReader) decodeStream(header *LrfHeader, flags int, payload []byte, partial bool) ([]byte, error) {
	if flags&streamFlagScrambled != 0 {
		unscramble(header.XorKey, payload, partial)
	}
	if flags&streamFlagCompressed == 0 {
		return payload, nil
	}
	if len(payload) < 4 {
		return nil, fmt.Errorf("%w: compressed stream too short", book.ErrCorrupt)
	}
	want := int(binary.LittleEndian.Uint32(payload[:4]))
	remain := int64(-1)
	if r.cfg.MaxResourceSize > 0 {
		remain = r.cfg.MaxResourceSize
	}
	data, err := compress.DecompressZlib(payload[4:], remain)
	if err != nil {
		return nil, err
	}
	if len(data) != want {
		return nil, fmt.Errorf("%w: stream size mismatch: got %d, want %d", book.ErrCorrupt, len(data), want)
	}
	return data, nil
}

// objectStreamByID extracts and decodes the stream of the object with the
// given id.
func (r *LrfReader) objectStreamByID(header *LrfHeader, ix *objectIndex, id uint32, partial bool) ([]byte, error) {
	_, body, err := ix.bodyByID(id)
	if err != nil {
		return nil, err
	}
	flags, payload, ok, err := objectStream(body)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%w: object %d has no stream", book.ErrCorrupt, id)
	}
	return r.decodeStream(header, flags, payload, partial)
}

// unscramble XORs the leading bytes of a payload in place. The key derives
// from the file XOR key and the payload
// length; a zero or out-of-range key leaves the payload untouched.
func unscramble(xorKey uint16, payload []byte, partial bool) {
	key := byte(xorKey & 0xFF)
	if key == 0 || key > 0xF0 {
		return
	}
	key = byte(len(payload)%int(key)) + 0xF
	n := len(payload)
	if partial && n > scrambleMaxBytes {
		n = scrambleMaxBytes
	}
	for i := 0; i < n; i++ {
		payload[i] ^= key
	}
}
