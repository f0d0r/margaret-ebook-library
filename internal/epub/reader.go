package epub

import (
	"archive/zip"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"faun.projects/margaret/margaret-ebook-library/pkg/model"
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

func (r *EpubReader) Supports(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil || info.Size() < 22 {
		return false
	}

	hasEpubExt := strings.HasSuffix(strings.ToLower(path), ".epub")
	hasValidEpubHeader := r.hasValidEpubHeader(f)

	if hasValidEpubHeader {
		return true
	}

	if !hasEpubExt {
		return false
	}

	zr, err := zip.NewReader(f, info.Size())
	if err != nil {
		return false
	}

	_, err = r.ocfReader.findContainerFile(zr)
	return err == nil
}

func (r *EpubReader) hasValidEpubHeader(f *os.File) bool {
	// A valid EPUB starts with a ZIP local file header, followed by the uncompressed "mimetype" file and its 20-byte value.
	buf := make([]byte, 100)
	n, err := f.ReadAt(buf, 0)
	if err != nil && n < 58 {
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

func (r *EpubReader) openZipReader(path string) (*zip.Reader, io.Closer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}

	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}

	zr, err := zip.NewReader(f, info.Size())
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}

	// Visszaadjuk a readert, és magát a fájlt mint Closert, hogy le lehessen zárni
	return zr, f, nil
}

func (r *EpubReader) ReadMetadata(path string) (*model.Metadata, error) {
	zr, closer, err := r.openZipReader(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open epub: %w", err)
	}
	defer func() { _ = closer.Close() }()

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
		Cover:       r.cover(path, zr, p),
		FileType:    model.EPUB,
	}, nil
}

func (r *EpubReader) cover(epubPath string, zr *zip.Reader, p Package) *model.Resource {
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
			return r.loadRecord(epubPath, coverPath)
		},
	}
}

func (r *EpubReader) loadRecord(filePath string, resourcePath string) ([]byte, error) {
	zr, closer, err := r.openZipReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open epub: %w", err)
	}
	defer func() { _ = closer.Close() }()
	targetFile := findFileInZip(zr, resourcePath)
	if targetFile == nil {
		return nil, fmt.Errorf("resource not found")
	}

	rc, err := targetFile.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()

	return io.ReadAll(rc)
}
