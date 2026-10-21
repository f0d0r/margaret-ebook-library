package lrf

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
)

func TestDecodeStreamPlain(t *testing.T) {
	r := NewLrfReader(config.DefaultConfig())
	got, err := r.decodeStream(&LrfHeader{}, 0, []byte("plain bytes"), false)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "plain bytes" {
		t.Errorf("decodeStream() = %q, want %q", got, "plain bytes")
	}
}

func TestDecodeStreamCompressed(t *testing.T) {
	plain := []byte("zlib compressed stream payload")
	payload := zlibStreamPayload(t, plain)

	r := NewLrfReader(config.DefaultConfig())
	got, err := r.decodeStream(&LrfHeader{}, streamFlagCompressed, payload, false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Errorf("decodeStream() = %q, want %q", got, plain)
	}
}

func TestDecodeStreamCompressedSizeMismatch(t *testing.T) {
	payload := zlibStreamPayload(t, []byte("data"))
	binary.LittleEndian.PutUint32(payload, 9999)

	r := NewLrfReader(config.DefaultConfig())
	if _, err := r.decodeStream(&LrfHeader{}, streamFlagCompressed, payload, false); !errors.Is(err, book.ErrCorrupt) {
		t.Errorf("decodeStream() error = %v, want book.ErrCorrupt", err)
	}
}

func TestDecodeStreamCompressedTooShort(t *testing.T) {
	r := NewLrfReader(config.DefaultConfig())
	if _, err := r.decodeStream(&LrfHeader{}, streamFlagCompressed, []byte{1, 2}, false); !errors.Is(err, book.ErrCorrupt) {
		t.Errorf("decodeStream() error = %v, want book.ErrCorrupt", err)
	}
}

func TestDecodeStreamLimit(t *testing.T) {
	payload := zlibStreamPayload(t, bytes.Repeat([]byte{0x41}, 1024))

	r := NewLrfReader(config.Config{MaxResourceSize: 16})
	if _, err := r.decodeStream(&LrfHeader{}, streamFlagCompressed, payload, false); !errors.Is(err, book.ErrLimitExceeded) {
		t.Errorf("decodeStream() error = %v, want book.ErrLimitExceeded", err)
	}
}

func TestDecodeStreamScrambled(t *testing.T) {
	plain := bytes.Repeat([]byte{0x11, 0x22, 0x33}, 20)
	const xorKey = 0x0050
	scrambled := scrambleTestBytes(plain, xorKey, 0)

	r := NewLrfReader(config.DefaultConfig())
	got, err := r.decodeStream(&LrfHeader{XorKey: xorKey}, streamFlagScrambled, bytes.Clone(scrambled), false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Errorf("decodeStream() = % X, want % X", got, plain)
	}
}

func TestDecodeStreamScrambledPartial(t *testing.T) {
	plain := bytes.Repeat([]byte{0x5A}, 0x800)
	const xorKey = 0x0020

	scrambledFull := scrambleTestBytes(plain, xorKey, 0)
	scrambledPartial := scrambleTestBytes(plain, xorKey, scrambleMaxBytes)

	r := NewLrfReader(config.DefaultConfig())
	got, err := r.decodeStream(&LrfHeader{XorKey: xorKey}, streamFlagScrambled, bytes.Clone(scrambledPartial), true)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Errorf("decodeStream(partial) did not reverse partial scrambling")
	}
	got, err = r.decodeStream(&LrfHeader{XorKey: xorKey}, streamFlagScrambled, bytes.Clone(scrambledFull), false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Errorf("decodeStream(full) did not reverse full-length scrambling")
	}
}

func TestDecodeStreamScrambledThenCompressed(t *testing.T) {
	plain := bytes.Repeat([]byte("text text text "), 8)
	const xorKey = 0x0033
	compressed := zlibStreamPayload(t, plain)
	scrambled := scrambleTestBytes(compressed, xorKey, 0)

	r := NewLrfReader(config.DefaultConfig())
	got, err := r.decodeStream(&LrfHeader{XorKey: xorKey}, streamFlagScrambled|streamFlagCompressed, bytes.Clone(scrambled), false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Errorf("decodeStream() did not reverse scrambling before decompression")
	}
}

func TestUnscrambleKeyEdges(t *testing.T) {
	plain := []byte{1, 2, 3, 4, 5}

	zeroKey := bytes.Clone(plain)
	unscramble(0x0000, zeroKey, false)
	if !bytes.Equal(zeroKey, plain) {
		t.Errorf("unscramble with xor key 0 changed the payload: % X", zeroKey)
	}

	highKey := bytes.Clone(plain)
	unscramble(0x00F1, highKey, false)
	if !bytes.Equal(highKey, plain) {
		t.Errorf("unscramble with key byte 0xF1 changed the payload: % X", highKey)
	}

	lowNibbleOnly := bytes.Clone(plain)
	unscramble(0x1234, lowNibbleOnly, false)
	if bytes.Equal(lowNibbleOnly, plain) {
		t.Errorf("unscramble with key byte 0x34 left the payload unchanged")
	}
}

func TestObjectStream(t *testing.T) {
	var body []byte
	body = append(body, tDword(tagLink, 3)...)
	body = append(body, tStream(0x0100, []byte{0xDE, 0xAD})...)

	flags, payload, ok, err := objectStream(body)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("objectStream() ok = false, want true")
	}
	if flags != 0x0100 {
		t.Errorf("flags = %#x, want 0x100", flags)
	}
	if !bytes.Equal(payload, []byte{0xDE, 0xAD}) {
		t.Errorf("payload = % X, want DE AD", payload)
	}
}

func TestObjectStreamMissing(t *testing.T) {
	body := tDword(tagLink, 3)

	_, _, ok, err := objectStream(body)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("objectStream() ok = true for object without stream")
	}
}
