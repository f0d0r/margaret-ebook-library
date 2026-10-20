package mobi

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
)

func createBenchMOBI(b *testing.B, path string, records int) {
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
	binary.BigEndian.PutUint32(mobiData[168:172], 0xFFFFFFFF) // DRM offset: none
	data = append(data, mobiData...)

	for i := 1; i < records; i++ {
		data = append(data, make([]byte, recSize)...)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		b.Fatalf("failed to write MOBI file: %v", err)
	}
}

func BenchmarkMobiRead(b *testing.B) {
	tmpDir := b.TempDir()
	path := filepath.Join(tmpDir, "book.mobi")
	createBenchMOBI(b, path, 1000)

	reader := NewMobiReader(config.DefaultConfig())

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := reader.Read(book.NewPathBlob(path)); err != nil {
				b.Fatalf("Read() error: %v", err)
			}
		}
	})
}

func BenchmarkMobiReadFromFile(b *testing.B) {
	tmpDir := b.TempDir()
	path := filepath.Join(tmpDir, "book.mobi")
	createBenchMOBI(b, path, 1000)

	reader := NewMobiReader(config.DefaultConfig())

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
			if _, err := reader.Read(blob); err != nil {
				b.Fatalf("Read() error: %v", err)
			}
			_ = f.Close()
		}
	})
}
