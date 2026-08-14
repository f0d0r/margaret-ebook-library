package ebook

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/pkg/errs"
	"github.com/f0d0r/margaret-ebook-library/pkg/model"
)

func TestReadMetadata_EmptyPath(t *testing.T) {
	_, err := ReadMetadata("")
	if err != errs.ErrUnsupportedFormat {
		t.Fatalf("expected ErrUnsupportedFormat for empty path, got %v", err)
	}
}

func TestReadMetadata_WhitespacePath(t *testing.T) {
	_, err := ReadMetadata("   \t\n  ")
	if err != errs.ErrUnsupportedFormat {
		t.Fatalf("expected ErrUnsupportedFormat for whitespace-only path, got %v", err)
	}
}

func TestReadMetadataFromFile_UnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "book.txt")
	if err := os.WriteFile(path, []byte("plain text"), 0644); err != nil {
		t.Fatalf("failed to write text file: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	defer func() { _ = f.Close() }()

	_, err = ReadMetadataFromFile(f)
	if !errors.Is(err, errs.ErrUnsupportedFormat) {
		t.Errorf("ReadMetadataFromFile() error = %v, want ErrUnsupportedFormat", err)
	}
}

func TestReadMetadataFromFile_ValidEPUB(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "book.epub")
	createTestEPUB(t, path, "My Test Book")

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open EPUB: %v", err)
	}
	defer func() { _ = f.Close() }()

	metadata, err := ReadMetadataFromFile(f)
	if err != nil {
		t.Fatalf("ReadMetadataFromFile() error: %v", err)
	}
	if metadata.FileType != model.EPUB {
		t.Errorf("FileType = %v, want %v", metadata.FileType, model.EPUB)
	}
	if metadata.Title != "My Test Book" {
		t.Errorf("Title = %q, want %q", metadata.Title, "My Test Book")
	}
}

func TestReadMetadata_ZeroRecordMOBI_NoPanic(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.mobi")
	if err := os.WriteFile(path, buildZeroRecordMOBI(), 0644); err != nil {
		t.Fatalf("failed to write MOBI file: %v", err)
	}

	if _, err := ReadMetadata(path); err == nil {
		t.Errorf("ReadMetadata() expected error for zero-record MOBI, got nil")
	}
}

func TestReadMetadataFromFile_ValidMOBI(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "book.mobi")
	if err := os.WriteFile(path, buildMinimalMOBI(), 0644); err != nil {
		t.Fatalf("failed to write MOBI file: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open MOBI: %v", err)
	}
	defer func() { _ = f.Close() }()

	metadata, err := ReadMetadataFromFile(f)
	if err != nil {
		t.Fatalf("ReadMetadataFromFile() error: %v", err)
	}
	if metadata.FileType != model.MOBI {
		t.Errorf("FileType = %v, want %v", metadata.FileType, model.MOBI)
	}
}

func TestReadMetadataFromFile_PreservesPosition(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "book.epub")
	createTestEPUB(t, path, "My Test Book")

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open EPUB: %v", err)
	}
	defer func() { _ = f.Close() }()

	// Move to a non-zero position before the call
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		t.Fatalf("failed to seek to end: %v", err)
	}
	posBefore, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		t.Fatalf("failed to get current position: %v", err)
	}

	if _, err := ReadMetadataFromFile(f); err != nil {
		t.Fatalf("ReadMetadataFromFile() error: %v", err)
	}

	posAfter, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		t.Fatalf("failed to get current position: %v", err)
	}
	if posAfter != posBefore {
		t.Errorf("file position changed: before=%d after=%d", posBefore, posAfter)
	}
}

func TestReadMetadataFromBlob_ValidEPUB(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "book.epub")
	createTestEPUB(t, path, "My Test Book")

	metadata, err := ReadMetadataFromBlob(model.NewPathBlob(path))
	if err != nil {
		t.Fatalf("ReadMetadataFromBlob() error: %v", err)
	}
	if metadata.FileType != model.EPUB {
		t.Errorf("FileType = %v, want %v", metadata.FileType, model.EPUB)
	}
	if metadata.Title != "My Test Book" {
		t.Errorf("Title = %q, want %q", metadata.Title, "My Test Book")
	}
}

func TestReadMetadataFromBlob_UnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "book.txt")
	if err := os.WriteFile(path, []byte("plain text"), 0644); err != nil {
		t.Fatalf("failed to write text file: %v", err)
	}

	_, err := ReadMetadataFromBlob(model.NewPathBlob(path))
	if !errors.Is(err, errs.ErrUnsupportedFormat) {
		t.Errorf("ReadMetadataFromBlob() error = %v, want ErrUnsupportedFormat", err)
	}
}

func TestReadMetadata_DefaultConfigReadsCover(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "book.epub")
	createTestEPUB(t, path, "My Test Book")

	m, err := ReadMetadata(path)
	if err != nil {
		t.Fatalf("ReadMetadata() error: %v", err)
	}
	if m.Cover == nil {
		t.Fatal("Cover = nil, want non-nil")
	}
	if _, err := m.Cover.Data(); err != nil {
		t.Fatalf("Cover.Data() error: %v", err)
	}
}

func TestReadMetadata_ConfigOverrideRejectsOversizedCover(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "book.epub")
	createTestEPUB(t, path, "My Test Book")

	cfg := model.DefaultConfig()
	cfg.MaxCoverSize = 16

	m, err := ReadMetadata(path, &cfg)
	if err != nil {
		t.Fatalf("ReadMetadata() error: %v", err)
	}
	if m.Cover == nil {
		t.Fatal("Cover = nil, want non-nil")
	}
	if _, err := m.Cover.Data(); err == nil {
		t.Fatal("Cover.Data() expected error for oversized cover, got nil")
	}
}

func TestReadMetadata_NilConfigUsesDefault(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "book.epub")
	createTestEPUB(t, path, "My Test Book")

	m, err := ReadMetadata(path, nil)
	if err != nil {
		t.Fatalf("ReadMetadata() error: %v", err)
	}
	if m.Cover == nil {
		t.Fatal("Cover = nil, want non-nil")
	}
	if _, err := m.Cover.Data(); err != nil {
		t.Fatalf("Cover.Data() error: %v", err)
	}
}

func TestReadMetadata_ZeroValueConfigUsesDefault(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "book.epub")
	createTestEPUB(t, path, "My Test Book")

	m, err := ReadMetadata(path, &model.Config{})
	if err != nil {
		t.Fatalf("ReadMetadata() error: %v", err)
	}
	if m.Cover == nil {
		t.Fatal("Cover = nil, want non-nil")
	}
	if _, err := m.Cover.Data(); err != nil {
		t.Fatalf("Cover.Data() error: %v", err)
	}
}

func TestReadMetadataFromBlob_ConfigOverride(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "book.epub")
	createTestEPUB(t, path, "My Test Book")

	cfg := model.DefaultConfig()
	cfg.MaxCoverSize = 16

	m, err := ReadMetadataFromBlob(model.NewPathBlob(path), &cfg)
	if err != nil {
		t.Fatalf("ReadMetadataFromBlob() error: %v", err)
	}
	if m.Cover == nil {
		t.Fatal("Cover = nil, want non-nil")
	}
	if _, err := m.Cover.Data(); err == nil {
		t.Fatal("Cover.Data() expected error for oversized cover, got nil")
	}
}

func TestCalculateFileHashFromBlob(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "book.bin")
	content := []byte("hello world")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	want := fmt.Sprintf("%x", sha256.Sum256(content))
	got, err := CalculateFileHashFromBlob(model.NewPathBlob(path))
	if err != nil {
		t.Fatalf("CalculateFileHashFromBlob() error: %v", err)
	}
	if got != want {
		t.Errorf("hash = %q, want %q", got, want)
	}
}

func TestCalculateFileHashFromBlobPreservesPosition(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "book.bin")
	if err := os.WriteFile(path, []byte("hello world"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		t.Fatalf("failed to seek: %v", err)
	}

	b, err := model.NewFileBlob(f)
	if err != nil {
		t.Fatalf("NewFileBlob() error: %v", err)
	}
	if _, err := CalculateFileHashFromBlob(b); err != nil {
		t.Fatalf("CalculateFileHashFromBlob() error: %v", err)
	}

	posAfter, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		t.Fatalf("failed to get current position: %v", err)
	}
	if posAfter != int64(len("hello world")) {
		t.Errorf("file position changed: before=%d after=%d", len("hello world"), posAfter)
	}
}

// createTestEPUB writes a minimal but valid EPUB file to path.
func createTestEPUB(t *testing.T, path, title string) {
	t.Helper()

	opf := `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>` + title + `</dc:title>
  </metadata>
  <manifest>
    <item id="cover" href="cover.jpg" media-type="image/jpeg" properties="cover-image"/>
  </manifest>
</package>`

	container := `<?xml version="1.0" encoding="UTF-8"?>
<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create EPUB file: %v", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Fatalf("failed to close EPUB file: %v", err)
		}
	}()

	w := zip.NewWriter(f)
	defer func() {
		if err := w.Close(); err != nil {
			t.Fatalf("failed to close zip writer: %v", err)
		}
	}()

	files := []struct {
		name    string
		content string
		method  uint16
	}{
		{name: "mimetype", content: "application/epub+zip", method: zip.Store},
		{name: "META-INF/container.xml", content: container, method: zip.Deflate},
		{name: "OEBPS/content.opf", content: opf, method: zip.Deflate},
		{name: "OEBPS/cover.jpg", content: string(createTestCoverImage()), method: zip.Store},
	}
	for _, file := range files {
		fw, err := w.CreateHeader(&zip.FileHeader{Name: file.name, Method: file.method})
		if err != nil {
			t.Fatalf("failed to create zip entry: %v", err)
		}
		if _, err := fw.Write([]byte(file.content)); err != nil {
			t.Fatalf("failed to write zip entry: %v", err)
		}
	}
}

// createTestCoverImage returns a small JPEG-ish blob used as a cover image.
func createTestCoverImage() []byte {
	img := make([]byte, 128)
	for i := range img {
		img[i] = byte(i)
	}
	return img
}

// buildMinimalMOBI returns the bytes of a minimal but parseable MOBI (PDB) file.
func buildMinimalMOBI() []byte {
	const pdbHeaderSize = 78

	var buf []byte

	header := make([]byte, pdbHeaderSize)
	copy(header[0:], "TestBook")
	binary.BigEndian.PutUint16(header[32:34], 0x0000) // attributes
	binary.BigEndian.PutUint16(header[34:36], 0x0000) // file version
	binary.BigEndian.PutUint32(header[36:40], 2082844800+1000)
	binary.BigEndian.PutUint32(header[40:44], 2082844800+1000+86400)
	binary.BigEndian.PutUint32(header[44:48], 0)
	binary.BigEndian.PutUint32(header[48:52], 1) // modification number
	binary.BigEndian.PutUint32(header[52:56], 0) // app info offset
	binary.BigEndian.PutUint32(header[56:60], 0) // sort info offset
	copy(header[60:64], "BOOK")
	copy(header[64:68], "MOBI")
	binary.BigEndian.PutUint32(header[68:72], 1) // unique id seed
	binary.BigEndian.PutUint32(header[72:76], 0) // next record list id
	binary.BigEndian.PutUint16(header[76:78], 1) // number of records
	buf = append(buf, header...)

	recordInfo := make([]byte, 8)
	recordOffset := uint32(pdbHeaderSize + 8)
	binary.BigEndian.PutUint32(recordInfo[0:4], recordOffset)
	recordInfo[4] = 0x00
	copy(recordInfo[5:8], []byte{0x00, 0x00, 0x01})
	buf = append(buf, recordInfo...)

	mobiData := make([]byte, 100)
	copy(mobiData[16:20], "MOBI")
	binary.BigEndian.PutUint32(mobiData[20:24], 232)   // header length
	binary.BigEndian.PutUint16(mobiData[0:2], 1)       // compression: none
	binary.BigEndian.PutUint32(mobiData[4:8], 1000)    // text length
	binary.BigEndian.PutUint16(mobiData[8:10], 1)      // record count
	binary.BigEndian.PutUint16(mobiData[10:12], 4096)  // record size
	binary.BigEndian.PutUint32(mobiData[24:28], 2)     // mobi type: book
	binary.BigEndian.PutUint32(mobiData[28:32], 65001) // text encoding: UTF-8
	buf = append(buf, mobiData...)

	return buf
}

// buildZeroRecordMOBI returns a PDB header that passes the "BOOKMOBI" magic
// check but declares zero records. It exercises the guard that prevents an
// index-out-of-range panic when no record data is present.
func buildZeroRecordMOBI() []byte {
	const pdbHeaderSize = 78

	header := make([]byte, pdbHeaderSize)
	copy(header[0:], "EmptyBook")
	binary.BigEndian.PutUint16(header[32:34], 0x0000) // attributes
	binary.BigEndian.PutUint16(header[34:36], 0x0000) // file version
	binary.BigEndian.PutUint32(header[36:40], 0)      // created
	binary.BigEndian.PutUint32(header[40:44], 0)      // updated
	binary.BigEndian.PutUint32(header[44:48], 0)      // backup
	binary.BigEndian.PutUint32(header[48:52], 0)      // modification number
	binary.BigEndian.PutUint32(header[52:56], 0)      // app info offset
	binary.BigEndian.PutUint32(header[56:60], 0)      // sort info offset
	copy(header[60:64], "BOOK")                       // type
	copy(header[64:68], "MOBI")                       // creator -> "BOOKMOBI" magic
	binary.BigEndian.PutUint32(header[68:72], 0)      // unique id seed
	binary.BigEndian.PutUint32(header[72:76], 0)      // next record list id
	binary.BigEndian.PutUint16(header[76:78], 0)      // number of records: 0

	return header
}
