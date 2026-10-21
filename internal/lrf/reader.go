package lrf

import (
	"io"
	"strconv"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
)

type LrfReader struct {
	cfg config.Config
}

func NewLrfReader(cfg config.Config) *LrfReader {
	return &LrfReader{cfg: cfg.Normalize()}
}

const lrfMagic = "L\x00R\x00F\x00\x00\x00"

func (r *LrfReader) Supports(b book.Blob) bool {
	var buf [8]byte
	n, err := b.ReadAt(buf[:], 0)
	if err != nil && err != io.EOF {
		return false
	}
	if n < len(buf) {
		return false
	}
	return string(buf[:]) == lrfMagic
}

func (r *LrfReader) Read(b book.Blob) (book.Book, error) {
	header, err := r.readHeader(b)
	if err != nil {
		return nil, err
	}
	metadata, err := r.readMetadata(b, header)
	if err != nil {
		return nil, err
	}
	resources, err := r.readResources(b, header)
	if err != nil {
		return nil, err
	}
	return &lrfBook{
		metadata:  metadata,
		resources: resources,
		version:   strconv.Itoa(int(header.Version)),
	}, nil
}
