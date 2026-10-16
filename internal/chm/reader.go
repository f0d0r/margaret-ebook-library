package chm

import (
	"encoding/binary"
	"io"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
)

type ChmReader struct {
	cfg config.Config
}

func NewChmReader(cfg config.Config) *ChmReader {
	return &ChmReader{cfg: cfg.Normalize()}
}

// Supports reports whether b looks like a CHM file. It checks the 4-byte
// "ITSF" magic at offset 0 plus the header version and length, so detection
// is a single Size call and a single 12-byte ReadAt with no decompression.
// Stricter validation (ITSP directory, section tables) is left to Read.
func (r *ChmReader) Supports(b book.Blob) bool {
	size, err := b.Size()
	if err != nil || size < itsfPreamble {
		return false
	}

	var buf [itsfPreamble]byte
	n, err := b.ReadAt(buf[:], 0)
	if err != nil && err != io.EOF {
		return false
	}
	if n < len(buf) {
		return false
	}
	if string(buf[:4]) != itsfMagic {
		return false
	}

	ver := binary.LittleEndian.Uint32(buf[4:])
	headerLen := binary.LittleEndian.Uint32(buf[8:])
	var minLen uint32
	switch ver {
	case 2:
		minLen = itsfV2Len
	case 3:
		minLen = itsfV3Len
	default:
		return false
	}
	if headerLen < minLen || int64(headerLen) > size {
		return false
	}
	return true
}

// Read parses the CHM container into metadata and lazily-opened resources.
func (r *ChmReader) Read(b book.Blob) (book.Book, error) {
	c, err := openContainer(b, r.cfg.MaxResourceSize)
	if err != nil {
		return nil, err
	}
	all, order, cover := c.buildResources()
	return &chmBook{
		metadata:  c.readMetadata(),
		resources: book.NewResourceSet(all, order, cover),
		version:   c.version,
	}, nil
}
