package registry

import (
	"archive/zip"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/internal/epub"
	"github.com/f0d0r/margaret-ebook-library/internal/mobi"
	"github.com/f0d0r/margaret-ebook-library/pkg/errs"
	"github.com/f0d0r/margaret-ebook-library/pkg/model"
)

func TestRegistry_ReaderForBlob(t *testing.T) {
	tmpDir := t.TempDir()

	epubPath := filepath.Join(tmpDir, "book.epub")
	createTestEPUB(t, epubPath, "Test Book")

	mobiPath := filepath.Join(tmpDir, "book.mobi")
	if err := os.WriteFile(mobiPath, buildMinimalMOBI(), 0644); err != nil {
		t.Fatalf("failed to write MOBI file: %v", err)
	}

	unsupportedPath := filepath.Join(tmpDir, "book.txt")
	if err := os.WriteFile(unsupportedPath, []byte("plain text"), 0644); err != nil {
		t.Fatalf("failed to write text file: %v", err)
	}

	r := New()

	t.Run("EPUB blob returns EpubReader", func(t *testing.T) {
		reader, err := r.ReaderForBlob(model.NewPathBlob(epubPath))
		if err != nil {
			t.Fatalf("ReaderForBlob() error: %v", err)
		}
		if _, ok := reader.(*epub.EpubReader); !ok {
			t.Errorf("ReaderForBlob() = %T, want *epub.EpubReader", reader)
		}
	})

	t.Run("MOBI blob returns MobiReader", func(t *testing.T) {
		reader, err := r.ReaderForBlob(model.NewPathBlob(mobiPath))
		if err != nil {
			t.Fatalf("ReaderForBlob() error: %v", err)
		}
		if _, ok := reader.(*mobi.MobiReader); !ok {
			t.Errorf("ReaderForBlob() = %T, want *mobi.MobiReader", reader)
		}
	})

	t.Run("Unsupported blob returns ErrUnsupportedFormat", func(t *testing.T) {
		_, err := r.ReaderForBlob(model.NewPathBlob(unsupportedPath))
		if !errors.Is(err, errs.ErrUnsupportedFormat) {
			t.Errorf("ReaderForBlob() error = %v, want ErrUnsupportedFormat", err)
		}
	})
}

// createTestEPUB writes a minimal but valid EPUB file to path.
func createTestEPUB(t *testing.T, path, title string) {
	t.Helper()

	opf := `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>` + title + `</dc:title>
  </metadata>
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
