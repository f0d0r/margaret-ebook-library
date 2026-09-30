package ebook

import (
	"archive/zip"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
)

func TestRead_EmptyPath(t *testing.T) {
	_, err := Read("")
	if err != book.ErrUnsupportedFormat {
		t.Fatalf("expected ErrUnsupportedFormat for empty path, got %v", err)
	}
}

func TestRead_WhitespacePath(t *testing.T) {
	_, err := Read("   \t\n  ")
	if err != book.ErrUnsupportedFormat {
		t.Fatalf("expected ErrUnsupportedFormat for whitespace-only path, got %v", err)
	}
}

func TestReadFromFile_UnsupportedFormat(t *testing.T) {
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

	_, err = ReadFromFile(f)
	if !errors.Is(err, book.ErrUnsupportedFormat) {
		t.Errorf("ReadFromFile() error = %v, want ErrUnsupportedFormat", err)
	}
}

func TestReadFromFile_ValidEPUB(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "book.epub")
	createTestEPUB(t, path, "My Test Book")

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open EPUB: %v", err)
	}
	defer func() { _ = f.Close() }()

	ebook, err := ReadFromFile(f)
	if err != nil {
		t.Fatalf("ReadFromFile() error: %v", err)
	}
	if ebook.FileType() != book.EPUB {
		t.Errorf("FileType = %v, want %v", ebook.FileType(), book.EPUB)
	}
	if ebook.Metadata().Title != "My Test Book" {
		t.Errorf("Title = %q, want %q", ebook.Metadata().Title, "My Test Book")
	}
}

func TestRead_ZeroRecordMOBI_NoPanic(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.mobi")
	if err := os.WriteFile(path, buildZeroRecordMOBI(), 0644); err != nil {
		t.Fatalf("failed to write MOBI file: %v", err)
	}

	if _, err := Read(path); err == nil {
		t.Errorf("Read() expected error for zero-record MOBI, got nil")
	}
}

func TestReadFromFile_ValidFB2(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "book.fb2")
	fb2 := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">` +
		`<description><title-info>` +
		`<author><first-name>API</first-name><last-name>Author</last-name></author>` +
		`<book-title>API Test Book</book-title><lang>en</lang>` +
		`</title-info></description>` +
		`<body><section><p>Hello API</p></section></body>` +
		`</FictionBook>`
	if err := os.WriteFile(path, []byte(fb2), 0644); err != nil {
		t.Fatalf("failed to write FB2 file: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open FB2: %v", err)
	}
	defer func() { _ = f.Close() }()

	ebook, err := ReadFromFile(f)
	if err != nil {
		t.Fatalf("ReadFromFile() error: %v", err)
	}
	if ebook.FileType() != book.FB2 {
		t.Errorf("FileType = %v, want %v", ebook.FileType(), book.FB2)
	}
	if ebook.Metadata().Title != "API Test Book" {
		t.Errorf("Title = %q, want %q", ebook.Metadata().Title, "API Test Book")
	}
	if len(ebook.Resources().ReadingOrder()) != 1 {
		t.Errorf("len(ReadingOrder) = %d, want 1", len(ebook.Resources().ReadingOrder()))
	}
}

func TestRead_ValidFB2Zipped(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "book.fbz")
	fb2 := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">` +
		`<description><title-info>` +
		`<author><nickname>zipauthor</nickname></author>` +
		`<book-title>Zipped Test Book</book-title><lang>en</lang>` +
		`</title-info></description>` +
		`<body><section><p>Hello Zip</p></section></body>` +
		`</FictionBook>`

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create FBZ file: %v", err)
	}
	w := zip.NewWriter(f)
	fw, err := w.Create("book.fb2")
	if err != nil {
		t.Fatalf("failed to create zip entry: %v", err)
	}
	if _, err := fw.Write([]byte(fb2)); err != nil {
		t.Fatalf("failed to write zip entry: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("failed to close FBZ file: %v", err)
	}

	ebook, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if ebook.FileType() != book.FB2 {
		t.Errorf("FileType = %v, want %v", ebook.FileType(), book.FB2)
	}
	if ebook.Metadata().Title != "Zipped Test Book" {
		t.Errorf("Title = %q, want %q", ebook.Metadata().Title, "Zipped Test Book")
	}
}

func TestReadFromFile_ValidMOBI(t *testing.T) {
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

	ebook, err := ReadFromFile(f)
	if err != nil {
		t.Fatalf("ReadFromFile() error: %v", err)
	}
	if ebook.FileType() != book.MOBI {
		t.Errorf("FileType = %v, want %v", ebook.FileType(), book.MOBI)
	}
}

func TestReadFromFile_PreservesPosition(t *testing.T) {
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

	if _, err := ReadFromFile(f); err != nil {
		t.Fatalf("ReadFromFile() error: %v", err)
	}

	posAfter, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		t.Fatalf("failed to get current position: %v", err)
	}
	if posAfter != posBefore {
		t.Errorf("file position changed: before=%d after=%d", posBefore, posAfter)
	}
}

func TestReadFromBlob_ValidEPUB(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "book.epub")
	createTestEPUB(t, path, "My Test Book")

	ebook, err := ReadFromBlob(book.NewPathBlob(path))
	if err != nil {
		t.Fatalf("ReadFromBlob() error: %v", err)
	}
	if ebook.FileType() != book.EPUB {
		t.Errorf("FileType = %v, want %v", ebook.FileType(), book.EPUB)
	}
	if ebook.Metadata().Title != "My Test Book" {
		t.Errorf("Title = %q, want %q", ebook.Metadata().Title, "My Test Book")
	}
}

func TestReadFromBlob_UnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "book.txt")
	if err := os.WriteFile(path, []byte("plain text"), 0644); err != nil {
		t.Fatalf("failed to write text file: %v", err)
	}

	_, err := ReadFromBlob(book.NewPathBlob(path))
	if !errors.Is(err, book.ErrUnsupportedFormat) {
		t.Errorf("ReadFromBlob() error = %v, want ErrUnsupportedFormat", err)
	}
}

func TestRead_DefaultConfigReadsCover(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "book.epub")
	createTestEPUB(t, path, "My Test Book")

	m, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	cover, ok := m.Resources().CoverImage()
	if !ok || cover == nil {
		t.Fatal("Cover = nil, want non-nil")
	}
	if _, err := cover.Data(); err != nil {
		t.Fatalf("Cover.Data() error: %v", err)
	}
}

func TestRead_CoverOpenStreams(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "book.epub")
	createTestEPUB(t, path, "My Test Book")

	m, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	cover, ok := m.Resources().CoverImage()
	if !ok || cover == nil {
		t.Fatal("Cover = nil, want non-nil")
	}
	if cover.Open == nil {
		t.Fatal("Cover.Open = nil, want non-nil")
	}

	rc, err := cover.Open()
	if err != nil {
		t.Fatalf("Cover.Open() error: %v", err)
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	if int64(len(data)) != cover.Size {
		t.Errorf("streamed %d bytes, want Cover.Size %d", len(data), cover.Size)
	}
}

func TestRead_ConfigOverrideRejectsOversizedCover(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "book.epub")
	createTestEPUB(t, path, "My Test Book")

	m, err := Read(path, WithMaxResourceSize(16))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	cover, ok := m.Resources().CoverImage()
	if !ok || cover == nil {
		t.Fatal("Cover = nil, want non-nil")
	}
	if _, err := cover.Data(); err == nil {
		t.Fatal("Cover.Data() expected error for oversized cover, got nil")
	}
}

func TestRead_OptionsAccumulate(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "book.epub")
	createTestEPUB(t, path, "My Test Book")

	m, err := Read(path, WithMaxResourceSize(1<<30), WithMaxResourceSize(16))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	cover, ok := m.Resources().CoverImage()
	if !ok || cover == nil {
		t.Fatal("Cover = nil, want non-nil")
	}
	if _, err := cover.Data(); err == nil {
		t.Fatal("Cover.Data() expected error for oversized cover, got nil")
	}
}

func TestRead_MobiRecordSizeOption(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "book.mobi")
	if err := os.WriteFile(path, buildMinimalMOBI(), 0644); err != nil {
		t.Fatalf("failed to write MOBI file: %v", err)
	}

	if _, err := Read(path, WithMaxRecordSize(10)); err == nil {
		t.Fatal("Read() expected error with reduced MaxRecordSize, got nil")
	}

	m, err := Read(path, WithMaxRecordSize(100*1024*1024))
	if err != nil {
		t.Fatalf("Read() with default MaxRecordSize error: %v", err)
	}
	if m.FileType() != book.MOBI {
		t.Errorf("FileType = %v, want %v", m.FileType(), book.MOBI)
	}
}

func TestReadFromBlob_ConfigOverride(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "book.epub")
	createTestEPUB(t, path, "My Test Book")

	m, err := ReadFromBlob(book.NewPathBlob(path), WithMaxResourceSize(16))
	if err != nil {
		t.Fatalf("ReadFromBlob() error: %v", err)
	}
	cover, ok := m.Resources().CoverImage()
	if !ok || cover == nil {
		t.Fatal("Cover = nil, want non-nil")
	}
	if _, err := cover.Data(); err == nil {
		t.Fatal("Cover.Data() expected error for oversized cover, got nil")
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

// TestReadFromBlob_MalformedMOBIReturnsCorrupt pins the public sentinel
// contract for a malformed MOBI container: the caller gets ErrCorrupt through
// errors.Is instead of an unclassified error.
func TestReadFromBlob_MalformedMOBIReturnsCorrupt(t *testing.T) {
	// Passes format detection, but the record table runs past EOF.
	raw := buildMinimalMOBI()
	_, err := ReadFromBlob(book.NewBytesBlob(raw[:80]))
	if !errors.Is(err, book.ErrCorrupt) {
		t.Errorf("ReadFromBlob() error = %v, want ErrCorrupt", err)
	}
}

// TestKF8FallbackOptionIsPublicAPI keeps the exported option wired to the
// reader: it must be usable straight from the top-level package.
func TestKF8FallbackOptionIsPublicAPI(t *testing.T) {
	opt := WithKF8FallbackToMOBI6(false)
	if opt == nil {
		t.Fatal("WithKF8FallbackToMOBI6 returned nil")
	}
	cfg := resolveOptions([]Option{opt})
	if cfg.KF8Fallback() {
		t.Error("KF8Fallback() = true after WithKF8FallbackToMOBI6(false)")
	}
}
