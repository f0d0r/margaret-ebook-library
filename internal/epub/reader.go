package epub

import (
	"archive/zip"
	"encoding/binary"
	"fmt"
	"io"
	"path"

	"github.com/f0d0r/margaret-ebook-library/pkg/model"
)

type EpubReader struct {
	ocfReader OcfReader
	opfReader OpfReader
}

// NewEpubReader creates a new EPUB reader instance
func NewEpubReader() *EpubReader {
	return &EpubReader{
		ocfReader: NewOcfReader(),
		opfReader: NewOpfReader(),
	}
}

func (r *EpubReader) Supports(b model.Blob) bool {
	size, err := b.Size()
	if err != nil || size < 22 {
		return false
	}

	if r.hasValidEpubHeader(b) {
		return true
	}

	zr, err := zip.NewReader(b, size)
	if err != nil {
		return false
	}

	_, err = r.ocfReader.findContainerFile(zr)
	return err == nil
}

func (r *EpubReader) hasValidEpubHeader(b model.Blob) bool {
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

// ReadMetadata reads ebook metadata from a random-access source. It
// never modifies the source's position and does not take ownership of it.
func (r *EpubReader) ReadMetadata(b model.Blob) (*model.Metadata, error) {
	zr, err := openZip(b)
	if err != nil {
		return nil, fmt.Errorf("failed to open epub: %w", err)
	}

	c, err := r.ocfReader.Read(zr)
	if err != nil {
		return nil, fmt.Errorf("failed to read ocf file: %w", err)
	}

	p, err := r.opfReader.Read(zr, c)
	if err != nil {
		return nil, fmt.Errorf("failed to read opf file: %w", err)
	}

	return &model.Metadata{
		Title:       r.opfReader.Title(p),
		Authors:     r.opfReader.Authors(p),
		Description: r.opfReader.Description(p),
		Languages:   r.opfReader.Languages(p),
		Cover:       r.cover(zr, p),
		FileType:    model.EPUB,
	}, nil
}

// openZip builds a zip.Reader from the blob.
func openZip(b model.Blob) (*zip.Reader, error) {
	size, err := b.Size()
	if err != nil {
		return nil, err
	}
	return zip.NewReader(b, size)
}

// maxCoverSize bounds how many bytes of a cover image we are willing to
// decompress, protecting against zip-bomb style EPUBs. It mirrors the safety
// limit used by the MOBI reader.
const maxCoverSize = 50 * 1024 * 1024

// cover resolves the cover image of an EPUB from the OPF package and returns
// a Resource whose Data closure reads the cover lazily from the already-open
// zip.Reader. The caller must keep the underlying blob usable until the Data
// closure has been consumed.
func (r *EpubReader) cover(zr *zip.Reader, p Package) *model.Resource {
	coverItems := r.opfReader.ItemsByProperty(p, "cover-image")
	var coverItem *Item
	if len(coverItems) > 0 {
		coverItem = &coverItems[0]
	} else {
		coverMeta := r.opfReader.MetaByName(p, "cover")
		if coverMeta != nil {
			coverItem = r.opfReader.ItemById(p, coverMeta.Content)
		}
	}
	if coverItem == nil {
		return nil
	}
	coverPath := p.ResolvePath(coverItem.Href)

	coverFile := findFileInZip(zr, coverPath)
	if coverFile == nil {
		return nil
	}
	return &model.Resource{
		Id:        coverItem.ID,
		Name:      path.Base(coverFile.Name),
		MediaType: coverItem.MediaType,
		Size:      int(coverFile.UncompressedSize64),
		Data: func() ([]byte, error) {
			return readZipFile(coverFile)
		},
	}
}

// readZipFile reads the full contents of a zip entry, refusing entries that
// would decompress to more than maxCoverSize bytes.
func readZipFile(f *zip.File) ([]byte, error) {
	if f.UncompressedSize64 > maxCoverSize {
		return nil, fmt.Errorf("resource %q too large (%d bytes)", f.Name, f.UncompressedSize64)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(io.LimitReader(rc, maxCoverSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxCoverSize {
		return nil, fmt.Errorf("resource %q too large", f.Name)
	}
	return data, nil
}
