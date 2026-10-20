package epub

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
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

// consumeCover reads the cover data, if any, so that the benchmarks exercise
// the lazy cover loading path (including the underlying zip central-directory
// access) rather than skipping it.
func consumeCover(b *testing.B, m book.Book) {
	b.Helper()
	cover, ok := m.Resources().CoverImage()
	if !ok || cover == nil {
		return
	}
	if _, err := cover.Data(); err != nil {
		b.Fatalf("cover data error: %v", err)
	}
}

func BenchmarkEpubRead(b *testing.B) {
	tmpDir := b.TempDir()
	path := filepath.Join(tmpDir, "book.epub")
	createBenchEPUB(b, path, 2000)

	reader := NewEpubReader(config.DefaultConfig())

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ebook, err := reader.Read(book.NewPathBlob(path))
			if err != nil {
				b.Fatalf("Read() error: %v", err)
			}
			consumeCover(b, ebook)
		}
	})
}

func BenchmarkEpubReadFromFile(b *testing.B) {
	tmpDir := b.TempDir()
	path := filepath.Join(tmpDir, "book.epub")
	createBenchEPUB(b, path, 2000)

	reader := NewEpubReader(config.DefaultConfig())

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			f, err := os.Open(path)
			if err != nil {
				b.Fatalf("failed to open file: %v", err)
			}
			blob, err := book.NewFileBlob(f)
			if err != nil {
				b.Fatalf("failed to create file blob: %v", err)
			}
			ebook, err := reader.Read(blob)
			if err != nil {
				b.Fatalf("Read() error: %v", err)
			}
			consumeCover(b, ebook)
			_ = f.Close()
		}
	})
}
