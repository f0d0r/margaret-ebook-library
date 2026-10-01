package epub

import (
	"archive/zip"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/f0d0r/margaret-ebook-library/book"
	compressutil "github.com/f0d0r/margaret-ebook-library/internal/compress"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
	"github.com/f0d0r/margaret-ebook-library/internal/opf"
)

type EpubReader struct {
	cfg       config.Config
	ocfReader OcfReader
}

// NewEpubReader creates a new EPUB reader instance with the given
// safety limits.
func NewEpubReader(cfg config.Config) *EpubReader {
	return &EpubReader{
		cfg:       cfg,
		ocfReader: NewOcfReader(),
	}
}

func (r *EpubReader) Supports(b book.Blob) bool {
	size, err := b.Size()
	if err != nil || size < 22 {
		return false
	}

	if r.hasValidEpubHeader(b) {
		return true
	}

	zr, err := compressutil.Open(b)
	if err != nil {
		return false
	}

	_, err = r.ocfReader.findContainerFile(zr)
	return err == nil
}

func (r *EpubReader) hasValidEpubHeader(b book.Blob) bool {
	// A valid EPUB starts with a ZIP local file header, followed by the uncompressed "mimetype" file and its 20-byte value.
	buf := make([]byte, 100)
	n, err := b.ReadAt(buf, 0)
	if err != nil || n < 58 {
		return false
	}

	// 1. Verify ZIP signature. Valid ZIP files can start with one of several PK headers.
	if buf[0] != 0x50 || buf[1] != 0x4B {
		return false
	}

	switch {
	case buf[2] == 0x03 && buf[3] == 0x04:
		// ZIP local file header
	case buf[2] == 0x05 && buf[3] == 0x06:
		// ZIP end of central directory (empty archive style)
	case buf[2] == 0x07 && buf[3] == 0x08:
		// ZIP data descriptor / other valid ZIP signature
	default:
		return false
	}

	// 2. Read the file name length and extra field length (Little Endian)
	fileNameLen := binary.LittleEndian.Uint16(buf[26:28])
	extraLen := binary.LittleEndian.Uint16(buf[28:30])

	// The "mimetype" file name is exactly 8 characters long
	if fileNameLen != 8 {
		return false
	}

	// 3. Verify the file name is "mimetype"
	if string(buf[30:38]) != "mimetype" {
		return false
	}

	// 4. Compute and verify the mimetype content location.
	// According to the spec extraLen should be 0, but we tolerate malformed files.
	mimetypeStart := 30 + int(fileNameLen) + int(extraLen)
	mimetypeEnd := mimetypeStart + 20

	if len(buf) < mimetypeEnd {
		return false
	}

	return string(buf[mimetypeStart:mimetypeEnd]) == "application/epub+zip"
}

func (r *EpubReader) Read(b book.Blob) (book.Book, error) {
	zr, err := compressutil.Open(b)
	if err != nil {
		return nil, fmt.Errorf("failed to open epub: %w", err)
	}

	c, err := r.ocfReader.Read(zr)
	if err != nil {
		return nil, fmt.Errorf("failed to read ocf file: %w", err)
	}

	p, err := readPackage(zr, c)
	if err != nil {
		return nil, fmt.Errorf("failed to read opf file: %w", err)
	}

	if err := checkDRM(zr, p); err != nil {
		return nil, err
	}

	resources, readingOrder, cover := r.readResources(zr, p)

	return &epubBook{
		metadata: book.Metadata{
			Title:       p.Title(),
			Authors:     p.Authors(),
			Description: p.Description(),
			Languages:   p.Languages(),
		},
		resources:  book.NewResourceSet(resources, readingOrder, cover),
		version:    p.Version,
		packageDoc: p,
	}, nil
}

// readResources returns all manifest resources, spine reading order with Linear flag, and cover.
func (r *EpubReader) readResources(zr *zip.Reader, p opf.Package) ([]*book.Resource, []book.ReadingOrderItem, *book.Resource) {
	return opf.BuildResources(p, func(item opf.Item) (opf.EntrySource, bool) {
		file := compressutil.Find(zr, p.ResolvePath(item.Href))
		if file == nil {
			return opf.EntrySource{}, false
		}
		return opf.EntrySource{
			Size: int64(file.UncompressedSize64),
			Open: func() (io.ReadCloser, error) {
				maxSize := r.cfg.MaxResourceSize
				if maxSize <= 0 {
					maxSize = config.DefaultConfig().MaxResourceSize
				}
				return compressutil.OpenLimited(file, maxSize)
			},
		}, true
	})
}
