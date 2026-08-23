package mobi

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/pkg/model"
)

func TestMobiReaderLoadRecord(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "record.bin")
	content := []byte("0123456789ABCDEFGHIJ")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	tests := []struct {
		name    string
		offset  uint32
		length  uint32
		want    string
		wantErr bool
	}{
		{"valid read from start", 0, 5, "01234", false},
		{"valid read from middle", 4, 4, "4567", false},
		{"read entire file", 0, 20, "0123456789ABCDEFGHIJ", false},
		{"offset past file end", 100, 5, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := readRecordData(model.NewPathBlob(path), tt.offset, tt.length, model.DefaultConfig().MaxRecordSize)
			if tt.wantErr {
				if err == nil {
					t.Errorf("readRecordData() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("readRecordData() unexpected error: %v", err)
			}
			if string(data) != tt.want {
				t.Errorf("readRecordData() = %q, want %q", string(data), tt.want)
			}
		})
	}
}

func TestMobiReaderCover(t *testing.T) {
	tmpDir := t.TempDir()
	reader := NewMobiReader(model.DefaultConfig())

	t.Run("CoverRecordIdx returns 0", func(t *testing.T) {
		mobi := &Mobi{}
		result := reader.cover(model.NewPathBlob("/fake/path"), &PdbDb{}, mobi)
		if result != nil {
			t.Errorf("cover() = %v, want nil", result)
		}
	})

	t.Run("coverIdx out of bounds", func(t *testing.T) {
		mobi := &Mobi{
			EXTH: &Exth{
				Records: map[uint32][][]byte{
					COVER_OFFSET: {func() []byte {
						b := make([]byte, 4)
						binary.BigEndian.PutUint32(b, 5)
						return b
					}()},
				},
			},
			FirstImageRecord: 10,
			recordCount:      20,
		}
		pdbDb := &PdbDb{PdbRecords: make([]PdbRecord, 5)}
		// coverIdx = 10 + 5 = 15, len(PdbRecords) = 5 → out of bounds
		result := reader.cover(model.NewPathBlob("/fake/path"), pdbDb, mobi)
		if result != nil {
			t.Errorf("cover() = %v, want nil", result)
		}
	})

	t.Run("DataSlice fails returns nil", func(t *testing.T) {
		mobi := &Mobi{
			EXTH: &Exth{
				Records: map[uint32][][]byte{
					COVER_OFFSET: {func() []byte {
						b := make([]byte, 4)
						binary.BigEndian.PutUint32(b, 0)
						return b
					}()},
				},
			},
			FirstImageRecord: 1,
			recordCount:      10,
		}
		pdbDb := &PdbDb{
			PdbRecords: []PdbRecord{
				{},
				{
					Offset: 0,
					Length: 10,
					DataSlice: func(uint32) ([]byte, error) {
						return nil, fmt.Errorf("read error")
					},
				},
			},
		}
		result := reader.cover(model.NewPathBlob("/fake/path"), pdbDb, mobi)
		if result != nil {
			t.Errorf("cover() = %v, want nil", result)
		}
	})

	t.Run("unrecognized image format returns nil", func(t *testing.T) {
		mobi := &Mobi{
			EXTH: &Exth{
				Records: map[uint32][][]byte{
					COVER_OFFSET: {func() []byte {
						b := make([]byte, 4)
						binary.BigEndian.PutUint32(b, 0)
						return b
					}()},
				},
			},
			FirstImageRecord: 1,
			recordCount:      10,
		}
		pdbDb := &PdbDb{
			PdbRecords: []PdbRecord{
				{},
				{
					Offset: 0,
					Length: 10,
					DataSlice: func(uint32) ([]byte, error) {
						return []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, nil
					},
				},
			},
		}
		result := reader.cover(model.NewPathBlob("/fake/path"), pdbDb, mobi)
		if result != nil {
			t.Errorf("cover() = %v, want nil", result)
		}
	})

	t.Run("zero length cover record returns nil", func(t *testing.T) {
		mobi := &Mobi{
			EXTH: &Exth{
				Records: map[uint32][][]byte{
					COVER_OFFSET: {func() []byte {
						b := make([]byte, 4)
						binary.BigEndian.PutUint32(b, 0)
						return b
					}()},
				},
			},
			FirstImageRecord: 1,
			recordCount:      10,
		}
		pdbDb := &PdbDb{
			PdbRecords: []PdbRecord{
				{},
				{
					Offset: 0,
					Length: 0,
				},
			},
		}
		result := reader.cover(model.NewPathBlob("/fake/path"), pdbDb, mobi)
		if result != nil {
			t.Errorf("cover() = %v, want nil", result)
		}
	})

	t.Run("oversized cover record returns Open error", func(t *testing.T) {
		mobi := &Mobi{
			EXTH: &Exth{
				Records: map[uint32][][]byte{
					COVER_OFFSET: {func() []byte {
						b := make([]byte, 4)
						binary.BigEndian.PutUint32(b, 0)
						return b
					}()},
				},
			},
			FirstImageRecord: 1,
			recordCount:      10,
		}
		pdbDb := &PdbDb{
			PdbRecords: []PdbRecord{
				{},
				{
					Offset: 0,
					Length: 100*1024*1024 + 1,
					DataSlice: func(n uint32) ([]byte, error) {
						return []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46}, nil
					},
				},
			},
		}
		result := reader.cover(model.NewPathBlob("/fake/path"), pdbDb, mobi)
		if result == nil {
			t.Fatalf("cover() = nil, want resource with Open error")
		}
		if _, err := result.Open(); err == nil {
			t.Errorf("Open() = nil, want ErrLimitExceeded")
		}
	})

	t.Run("config override rejects cover", func(t *testing.T) {
		cfg := model.DefaultConfig()
		cfg.MaxResourceSize = 4
		cfgReader := NewMobiReader(cfg)

		mobi := &Mobi{
			EXTH: &Exth{
				Records: map[uint32][][]byte{
					COVER_OFFSET: {func() []byte {
						b := make([]byte, 4)
						binary.BigEndian.PutUint32(b, 0)
						return b
					}()},
				},
			},
			FirstImageRecord: 1,
			recordCount:      10,
		}
		pdbDb := &PdbDb{
			PdbRecords: []PdbRecord{
				{},
				{
					Offset: 0,
					Length: 16,
					DataSlice: func(n uint32) ([]byte, error) {
						return []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46}, nil
					},
				},
			},
		}
		result := cfgReader.cover(model.NewPathBlob("/fake/path"), pdbDb, mobi)
		if result == nil {
			t.Fatalf("cover() = nil, want resource with Open error")
		}
		if _, err := result.Open(); err == nil {
			t.Errorf("Open() = nil, want ErrLimitExceeded")
		}
	})

	t.Run("valid JPEG cover returns Resource with lazy Open", func(t *testing.T) {
		imgData := []byte{
			0xFF, 0xD8, 0xFF, 0xE0, // JPEG magic + APP0 marker
			0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, // rest of JPEG header
			0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, // more data
		}
		imgPath := filepath.Join(tmpDir, "cover_test.bin")
		if err := os.WriteFile(imgPath, imgData, 0644); err != nil {
			t.Fatalf("WriteFile error: %v", err)
		}

		mobi := &Mobi{
			EXTH: &Exth{
				Records: map[uint32][][]byte{
					COVER_OFFSET: {func() []byte {
						b := make([]byte, 4)
						binary.BigEndian.PutUint32(b, 0)
						return b
					}()},
				},
			},
			FirstImageRecord: 1,
			recordCount:      10,
		}
		pdbDb := &PdbDb{
			PdbRecords: []PdbRecord{
				{},
				{
					Offset: 0,
					Length: uint32(len(imgData)),
					DataSlice: func(n uint32) ([]byte, error) {
						return imgData[:n], nil
					},
					Data: func() ([]byte, error) {
						return imgData, nil
					},
				},
			},
		}
		result := reader.cover(model.NewPathBlob(imgPath), pdbDb, mobi)
		if result == nil {
			t.Fatalf("cover() = nil, want Resource")
		}
		if result.Name != "cover.jpg" {
			t.Errorf("Name = %q, want %q", result.Name, "cover.jpg")
		}
		if result.MediaType != "image/jpeg" {
			t.Errorf("MediaType = %q, want %q", result.MediaType, "image/jpeg")
		}
		if result.Size != int64(len(imgData)) {
			t.Errorf("Size = %d, want %d", result.Size, len(imgData))
		}
		if result.Open == nil {
			t.Fatal("Open function is nil")
		}

		rc, err := result.Open()
		if err != nil {
			t.Fatalf("Open() error: %v", err)
		}
		defer func() { _ = rc.Close() }()
		loaded, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("ReadAll() error: %v", err)
		}
		if string(loaded) != string(imgData) {
			t.Errorf("Open() = %x, want %x", loaded, imgData)
		}
	})

	t.Run("valid PNG cover", func(t *testing.T) {
		pngData := []byte{
			0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG magic
			0x00, 0x00, 0x00, 0x0D, // etc
		}
		imgPath := filepath.Join(tmpDir, "png_test.bin")
		if err := os.WriteFile(imgPath, pngData, 0644); err != nil {
			t.Fatalf("WriteFile error: %v", err)
		}

		mobi := &Mobi{
			EXTH: &Exth{
				Records: map[uint32][][]byte{
					COVER_OFFSET: {func() []byte {
						b := make([]byte, 4)
						binary.BigEndian.PutUint32(b, 0)
						return b
					}()},
				},
			},
			FirstImageRecord: 1,
			recordCount:      10,
		}
		pdbDb := &PdbDb{
			PdbRecords: []PdbRecord{
				{},
				{
					Offset: 0,
					Length: uint32(len(pngData)),
					DataSlice: func(n uint32) ([]byte, error) {
						return pngData[:n], nil
					},
					Data: func() ([]byte, error) {
						return pngData, nil
					},
				},
			},
		}
		result := reader.cover(model.NewPathBlob(imgPath), pdbDb, mobi)
		if result == nil {
			t.Fatalf("cover() = nil, want Resource")
		}
		if result.Name != "cover.png" {
			t.Errorf("Name = %q, want %q", result.Name, "cover.png")
		}
		if result.MediaType != "image/png" {
			t.Errorf("MediaType = %q, want %q", result.MediaType, "image/png")
		}
	})
}

func TestMobiReaderSupportsPreservesPosition(t *testing.T) {
	tmpDir := t.TempDir()
	reader := NewMobiReader(model.DefaultConfig())

	validPath := filepath.Join(tmpDir, "valid.mobi")
	validData := make([]byte, 60)
	validData = append(validData, []byte("BOOKMOBI")...)
	if err := os.WriteFile(validPath, validData, 0644); err != nil {
		t.Fatalf("failed to write valid MOBI file: %v", err)
	}

	f, err := os.Open(validPath)
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Seek(10, io.SeekCurrent); err != nil {
		t.Fatalf("failed to seek: %v", err)
	}

	b, err := model.NewFileBlob(f)
	if err != nil {
		t.Fatalf("NewFileBlob() error: %v", err)
	}
	if !reader.Supports(b) {
		t.Error("Supports() = false at non-zero position, want true")
	}

	posAfter, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		t.Fatalf("failed to get current position: %v", err)
	}
	if posAfter != 10 {
		t.Errorf("file position changed: before=10 after=%d", posAfter)
	}
}

func TestMobiReaderSupports(t *testing.T) {
	tmpDir := t.TempDir()
	reader := NewMobiReader(model.DefaultConfig())

	validPath := filepath.Join(tmpDir, "valid.mobi")
	validData := make([]byte, 60)
	validData = append(validData, []byte("BOOKMOBI")...)
	if err := os.WriteFile(validPath, validData, 0644); err != nil {
		t.Fatalf("failed to write valid MOBI file: %v", err)
	}

	if !reader.Supports(model.NewPathBlob(validPath)) {
		t.Error("Supports() = false for valid MOBI path blob, want true")
	}
}
