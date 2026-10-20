package lrf

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/f0d0r/margaret-ebook-library/book"
)

// lrfHeaderSize covers the fixed header up to the start of the compressed
// metadata block for version > 800 files. For version <= 800 files the
// thumbnail and uncompressed-size fields are absent and the block starts at
// 0x53, but any valid LRF file is larger than this.
const lrfHeaderSize = 0x58

const (
	offVersion            = 0x08
	offXorKey             = 0x0A
	offRootObjectID       = 0x0C
	offNumberOfObjects    = 0x10
	offObjectTableOffset  = 0x18
	offCompressedInfoSize = 0x4C
	offThumbnailType      = 0x4E
	offThumbnailSize      = 0x50
	offUncompressedInfo   = 0x54
)

type LrfHeader struct {
	Magic              [8]byte
	Version            uint16
	XorKey             uint16
	RootObjectID       uint32
	NumberOfObjects    uint64
	ObjectTableOffset  uint64
	CompressedInfoSize uint16
	ThumbnailType      uint16
	ThumbnailSize      uint32
	UncompressedInfo   uint32
	InfoStart          int64
}

func (r *LrfReader) readHeader(b book.Blob) (*LrfHeader, error) {
	var buf [lrfHeaderSize]byte
	n, err := b.ReadAt(buf[:], 0)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("read header: %w", err)
	}
	if n < lrfHeaderSize {
		return nil, fmt.Errorf("%w: LRF file too small: %d bytes", book.ErrCorrupt, n)
	}
	if string(buf[:len(lrfMagic)]) != lrfMagic {
		return nil, fmt.Errorf("%w: not an LRF file", book.ErrCorrupt)
	}

	h := &LrfHeader{
		Magic:              [8]byte(buf[:8]),
		Version:            binary.LittleEndian.Uint16(buf[offVersion : offVersion+2]),
		XorKey:             binary.LittleEndian.Uint16(buf[offXorKey : offXorKey+2]),
		RootObjectID:       binary.LittleEndian.Uint32(buf[offRootObjectID : offRootObjectID+4]),
		NumberOfObjects:    binary.LittleEndian.Uint64(buf[offNumberOfObjects : offNumberOfObjects+8]),
		ObjectTableOffset:  binary.LittleEndian.Uint64(buf[offObjectTableOffset : offObjectTableOffset+8]),
		CompressedInfoSize: binary.LittleEndian.Uint16(buf[offCompressedInfoSize : offCompressedInfoSize+2]),
		InfoStart:          0x58,
	}
	if h.Version > 800 {
		h.ThumbnailType = binary.LittleEndian.Uint16(buf[offThumbnailType:offThumbnailSize])
		h.ThumbnailSize = binary.LittleEndian.Uint32(buf[offThumbnailSize:offUncompressedInfo])
		h.UncompressedInfo = binary.LittleEndian.Uint32(buf[offUncompressedInfo:lrfHeaderSize])
	} else {
		h.InfoStart = 0x53
	}

	return h, nil
}
