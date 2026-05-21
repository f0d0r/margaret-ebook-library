package epub

import (
	"archive/zip"
	"encoding/binary"
	"encoding/xml"
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

	_, err = r.getContainerFile(zr)
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

func (r *EpubReader) getContainerFile(zr *zip.Reader) (*zip.File, error) {
	containerPath := "META-INF/container.xml"
	f := r.findFileInZip(zr, containerPath)
	if f == nil {
		return nil, fmt.Errorf("container.xml not found")
	}
	return f, nil
}

func (r *EpubReader) findFileInZip(zr *zip.Reader, path string) *zip.File {
	for _, file := range zr.File {
		if file.Name == path {
			return file
		}
	}
	return nil
}

func (r *EpubReader) ReadMetadata(path string) (*model.Metadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open the file %s: %w", path, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	zr, err := zip.NewReader(f, info.Size())
	if err != nil {
		return nil, fmt.Errorf("failed to create zip reader: %w", err)
	}

	c, err := r.getContainer(zr)
	if err != nil {
		return nil, fmt.Errorf("failed to read ocf file %w", err)
	}

	p, err := r.readOPF(zr, c)
	if err != nil {
		return nil, fmt.Errorf("failed to read opf file: %w", err)
	}

	return &model.Metadata{
		Title:    r.getTitle(p),
		FileType: model.EPUB,
	}, nil
}

func (r *EpubReader) getContainer(zr *zip.Reader) (Container, error) {
	containerFile, err := r.getContainerFile(zr)
	if err != nil {
		return Container{}, fmt.Errorf("failed to get container file: %w", err)
	}

	cfr, err := containerFile.Open()
	if err != nil {
		return Container{}, fmt.Errorf("failed to open container.xml: %w", err)
	}
	defer cfr.Close()

	var c Container
	if err := xml.NewDecoder(cfr).Decode(&c); err != nil {
		return Container{}, fmt.Errorf("failed to decode container.xml: %w", err)
	}
	return c, nil
}

func (r *EpubReader) readOPF(zr *zip.Reader, c Container) (Package, error) {
	if len(c.Rootfiles.RootfileList) == 0 {
		return Package{}, fmt.Errorf("no rootfile found in container.xml")
	}

	for _, rootfile := range c.Rootfiles.RootfileList {
		if rootfile.MediaType == "application/oebps-package+xml" {
			return r.getPackage(zr, rootfile)
		}
	}

	return Package{}, fmt.Errorf("no opf file found")
}

func (r *EpubReader) getPackage(zr *zip.Reader, rootfile Rootfile) (Package, error) {
	opf := r.findFileInZip(zr, rootfile.FullPath)
	if opf == nil {
		return Package{}, fmt.Errorf("opf file not found")
	}

	opfr, err := opf.Open()
	if err != nil {
		return Package{}, fmt.Errorf("failed to open opf file: %w", err)
	}
	defer opfr.Close()

	var p Package
	if err := xml.NewDecoder(opfr).Decode(&p); err != nil {
		return Package{}, fmt.Errorf("failed to decode opf file: %w", err)
	}
	return p, nil
}

func (r *EpubReader) getTitle(p Package) (string) {
	if len(p.Metadata.Title) == 0 {
		return ""
	}
	return p.Metadata.Title[0].Value
}
