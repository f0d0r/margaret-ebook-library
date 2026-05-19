package epub

import (
	"archive/zip"
	"encoding/binary"
	"fmt"
	"os"
	"strings"

	"faun.projects/margaret/margaret-ebook-library/pkg/model"
)

type EpubReader struct{}

func (r *EpubReader) Supports(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

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

	hasContainer := false
	for _, file := range zr.File {
		if file.Name == "META-INF/container.xml" {
			hasContainer = true
			break
		}
	}

	return hasContainer
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

func (r *EpubReader) ReadMetadata(path string) (*model.Metadata, error) {
	fmt.Printf("[epub-reader] reading metadata from: %s\n", path)

	return &model.Metadata{
		Title:    "Dummy EPUB Book",
		FileType: model.EPUB,
	}, nil
}
