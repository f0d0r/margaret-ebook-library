package ebook

import (
	"archive/zip"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/pkg/model"
)

// createBenchEPUB writes a valid EPUB with nEntries additional content files
// and a cover image, to stress central-directory scanning in zip.NewReader.
func createBenchEPUB(b *testing.B, path string, nEntries int) {
	b.Helper()

	opf := `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Bench Book</dc:title>
  </metadata>
  <manifest>
    <item id="cover" href="cover.jpg" media-type="image/jpeg" properties="cover-image"/>
  </manifest>
  <spine>
    <itemref idref="cover"/>
  </spine>
</package>`
	container := `<?xml version="1.0" encoding="UTF-8"?>
<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`

	f, err := os.Create(path)
	if err != nil {
		b.Fatalf("failed to create EPUB file: %v", err)
	}
	defer func() { _ = f.Close() }()

	w := zip.NewWriter(f)
	defer func() { _ = w.Close() }()

	writeEntry := func(name string, content []byte, method uint16) {
		h := &zip.FileHeader{Name: name, Method: method}
		fw, err := w.CreateHeader(h)
		if err != nil {
			b.Fatalf("failed to create zip entry %s: %v", name, err)
		}
		if _, err := fw.Write(content); err != nil {
			b.Fatalf("failed to write zip entry %s: %v", name, err)
		}
	}

	writeEntry("mimetype", []byte("application/epub+zip"), zip.Store)
	writeEntry("META-INF/container.xml", []byte(container), zip.Deflate)
	writeEntry("OEBPS/content.opf", []byte(opf), zip.Deflate)
	writeEntry("OEBPS/cover.jpg", make([]byte, 4096), zip.Deflate)
	for i := 0; i < nEntries; i++ {
		writeEntry(fmt.Sprintf("OEBPS/text/chapter-%05d.xhtml", i), []byte("<html></html>"), zip.Deflate)
	}
}

// consumeCover reads the cover data of the metadata, if any, so that the
// benchmarks exercise the lazy cover loading path (including the underlying
// zip central-directory access) rather than skipping it.
func consumeCover(b *testing.B, m model.Metadata) {
	b.Helper()
	if m.Cover == nil {
		return
	}
	if _, err := m.Cover.Data(); err != nil {
		b.Fatalf("cover data error: %v", err)
	}
}

func BenchmarkRead(b *testing.B) {
	tmpDir := b.TempDir()
	path := filepath.Join(tmpDir, "book.epub")
	createBenchEPUB(b, path, 2000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ebook, err := Read(path)
			if err != nil {
				b.Fatalf("Read() error: %v", err)
			}
			consumeCover(b, ebook.Metadata)
		}
	})
}

func BenchmarkReadFromFile(b *testing.B) {
	tmpDir := b.TempDir()
	path := filepath.Join(tmpDir, "book.epub")
	createBenchEPUB(b, path, 2000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			f, err := os.Open(path)
			if err != nil {
				b.Fatalf("failed to open file: %v", err)
			}
			ebook, err := ReadFromFile(f)
			if err != nil {
				b.Fatalf("ReadFromFile() error: %v", err)
			}
			consumeCover(b, ebook.Metadata)
			_ = f.Close()
		}
	})
}

func BenchmarkReadFromPathBlob(b *testing.B) {
	tmpDir := b.TempDir()
	path := filepath.Join(tmpDir, "book.epub")
	createBenchEPUB(b, path, 2000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ebook, err := ReadFromBlob(model.NewPathBlob(path))
			if err != nil {
				b.Fatalf("ReadFromBlob() error: %v", err)
			}
			consumeCover(b, ebook.Metadata)
		}
	})
}

func createBenchMOBI(b *testing.B, path string, records int) []byte {
	b.Helper()

	const pdbHeaderSize = 78
	const recSize = 4096

	data := make([]byte, 0, pdbHeaderSize+records*8+records*recSize)

	header := make([]byte, pdbHeaderSize)
	copy(header[0:], "TestBook")
	binary.BigEndian.PutUint16(header[34:36], 0)
	binary.BigEndian.PutUint32(header[36:40], 2082844800+1000)
	binary.BigEndian.PutUint32(header[40:44], 2082844800+1000+86400)
	binary.BigEndian.PutUint32(header[52:56], 0)
	binary.BigEndian.PutUint32(header[56:60], 0)
	copy(header[60:64], "BOOK")
	copy(header[64:68], "MOBI")
	binary.BigEndian.PutUint32(header[68:72], 1)
	binary.BigEndian.PutUint32(header[72:76], 0)
	binary.BigEndian.PutUint16(header[76:78], uint16(records))
	data = append(data, header...)

	firstOffset := uint32(pdbHeaderSize + records*8)
	for i := 0; i < records; i++ {
		rec := make([]byte, 8)
		binary.BigEndian.PutUint32(rec[0:4], firstOffset+uint32(i*recSize))
		rec[4] = 0x00
		rec[5] = byte(i >> 16)
		rec[6] = byte(i >> 8)
		rec[7] = byte(i)
		data = append(data, rec...)
	}

	mobiData := make([]byte, recSize)
	binary.BigEndian.PutUint16(mobiData[0:2], 1)
	binary.BigEndian.PutUint32(mobiData[4:8], 4096)
	binary.BigEndian.PutUint16(mobiData[8:10], uint16(records))
	binary.BigEndian.PutUint16(mobiData[10:12], recSize)
	binary.BigEndian.PutUint32(mobiData[16:20], 0x4D4F4249) // "MOBI"
	binary.BigEndian.PutUint32(mobiData[20:24], 232)
	binary.BigEndian.PutUint32(mobiData[24:28], 2)
	binary.BigEndian.PutUint32(mobiData[28:32], 65001)
	data = append(data, mobiData...)

	for i := 1; i < records; i++ {
		data = append(data, make([]byte, recSize)...)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		b.Fatalf("failed to write MOBI file: %v", err)
	}
	return data
}

func BenchmarkReadMOBIPath(b *testing.B) {
	tmpDir := b.TempDir()
	path := filepath.Join(tmpDir, "book.mobi")
	createBenchMOBI(b, path, 1000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := Read(path); err != nil {
				b.Fatalf("Read() error: %v", err)
			}
		}
	})
}

func BenchmarkReadMOBIFromFile(b *testing.B) {
	tmpDir := b.TempDir()
	path := filepath.Join(tmpDir, "book.mobi")
	createBenchMOBI(b, path, 1000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			f, err := os.Open(path)
			if err != nil {
				b.Fatalf("failed to open file: %v", err)
			}
			if _, err := ReadFromFile(f); err != nil {
				b.Fatalf("ReadFromFile() error: %v", err)
			}
			_ = f.Close()
		}
	})
}

func BenchmarkReadMOBIFromPathBlob(b *testing.B) {
	tmpDir := b.TempDir()
	path := filepath.Join(tmpDir, "book.mobi")
	createBenchMOBI(b, path, 1000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := ReadFromBlob(model.NewPathBlob(path)); err != nil {
				b.Fatalf("ReadFromBlob() error: %v", err)
			}
		}
	})
}
