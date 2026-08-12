package mobi

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/pkg/model"
)

func TestMOBISupports(t *testing.T) {
	tmpDir := t.TempDir()
	reader := &MobiReader{}

	validMOBIPath := filepath.Join(tmpDir, "valid.mobi")
	validData := make([]byte, 60)
	validData = append(validData, []byte("BOOKMOBI")...)
	if err := os.WriteFile(validMOBIPath, validData, 0644); err != nil {
		t.Fatalf("failed to write valid MOBI file: %v", err)
	}

	invalidMOBIPath := filepath.Join(tmpDir, "invalid.mobi")
	invalidData := []byte("BOOKMOBI" + string(make([]byte, 60)))
	if err := os.WriteFile(invalidMOBIPath, invalidData, 0644); err != nil {
		t.Fatalf("failed to write invalid MOBI file: %v", err)
	}

	shortPath := filepath.Join(tmpDir, "short.mobi")
	if err := os.WriteFile(shortPath, []byte("BOOK"), 0644); err != nil {
		t.Fatalf("failed to write short MOBI file: %v", err)
	}

	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{"Valid MOBI file", validMOBIPath, true},
		{"Identifier in wrong position", invalidMOBIPath, false},
		{"Too short file", shortPath, false},
		{"Non-existing file", filepath.Join(tmpDir, "missing.mobi"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reader.Supports(tt.filePath); got != tt.want {
				t.Errorf("Supports() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMOBISupportsEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()
	reader := &MobiReader{}

	tests := []struct {
		name    string
		setupFn func() string
		want    bool
	}{
		{
			name: "Exactly 68 bytes with valid identifier",
			setupFn: func() string {
				path := filepath.Join(tmpDir, "exact68.mobi")
				data := make([]byte, 60)
				data = append(data, []byte("BOOKMOBI")...)
				if err := os.WriteFile(path, data, 0644); err != nil {
					t.Fatalf("WriteFile error: %v", err)
				}
				return path
			},
			want: true,
		},
		{
			name: "More than 68 bytes with valid identifier",
			setupFn: func() string {
				path := filepath.Join(tmpDir, "more68.mobi")
				data := make([]byte, 60)
				data = append(data, []byte("BOOKMOBI")...)
				data = append(data, []byte("extra data here")...)
				if err := os.WriteFile(path, data, 0644); err != nil {
					t.Fatalf("WriteFile error: %v", err)
				}
				return path
			},
			want: true,
		},
		{
			name: "67 bytes (one short)",
			setupFn: func() string {
				path := filepath.Join(tmpDir, "short67.mobi")
				data := make([]byte, 67)
				if err := os.WriteFile(path, data, 0644); err != nil {
					t.Fatalf("WriteFile error: %v", err)
				}
				return path
			},
			want: false,
		},
		{
			name: "BOOKMOBI at wrong offset",
			setupFn: func() string {
				path := filepath.Join(tmpDir, "wrong_offset.mobi")
				data := make([]byte, 100)
				copy(data[0:], "BOOKMOBI")
				if err := os.WriteFile(path, data, 0644); err != nil {
					t.Fatalf("WriteFile error: %v", err)
				}
				return path
			},
			want: false,
		},
		{
			name: "Similar but wrong identifier",
			setupFn: func() string {
				path := filepath.Join(tmpDir, "similar.mobi")
				data := make([]byte, 60)
				data = append(data, []byte("BOOKMOBI"[0:7])...) // Missing last character
				data = append(data, []byte("X")...)
				if err := os.WriteFile(path, data, 0644); err != nil {
					t.Fatalf("WriteFile error: %v", err)
				}
				return path
			},
			want: false,
		},
		{
			name: "Empty file",
			setupFn: func() string {
				path := filepath.Join(tmpDir, "empty.mobi")
				if err := os.WriteFile(path, []byte{}, 0644); err != nil {
					t.Fatalf("WriteFile error: %v", err)
				}
				return path
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := tt.setupFn()
			got := reader.Supports(filePath)
			if got != tt.want {
				t.Errorf("Supports() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMOBISupportsCaseInsensitivity(t *testing.T) {
	tmpDir := t.TempDir()
	reader := &MobiReader{}

	tests := []struct {
		name       string
		identifier string
		want       bool
	}{
		{"Lowercase", "bookmobi", false},
		{"Mixed case", "BookMobi", false},
		{"Uppercase", "BOOKMOBI", true},
		{"Partial uppercase", "BOOKMOBI"[:4] + "mobi", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, tt.name+".mobi")
			data := make([]byte, 60)
			data = append(data, []byte(tt.identifier)...)
			if err := os.WriteFile(path, data, 0644); err != nil {
				t.Fatalf("WriteFile error: %v", err)
			}

			got := reader.Supports(path)
			if got != tt.want {
				t.Errorf("Supports() with %q = %v, want %v", tt.identifier, got, tt.want)
			}
		})
	}
}

func TestMobiReaderReadMetadataInvalid(t *testing.T) {
	reader := &MobiReader{}

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"Non-existent file", "/nonexistent/path/file.mobi", true},
		{"Empty path", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := reader.ReadMetadata(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadMetadata() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestMobiReaderReadMetadataValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	reader := &MobiReader{}

	// Create a valid MOBI file
	pdbData := buildValidPdbFileForReader()
	testFile := filepath.Join(tmpDir, "test.mobi")
	if err := os.WriteFile(testFile, pdbData, 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	metadata, err := reader.ReadMetadata(testFile)
	if err != nil {
		t.Fatalf("ReadMetadata() error: %v", err)
	}

	if metadata == nil {
		t.Errorf("ReadMetadata() returned nil metadata")
	} else if metadata.FileType != model.MOBI {
		t.Errorf("FileType = %v, want %v", metadata.FileType, model.MOBI)
	}
}

func TestMobiReaderReadMetadataReturnsValidModel(t *testing.T) {
	tmpDir := t.TempDir()
	reader := &MobiReader{}

	// Create a valid MOBI file
	pdbData := buildValidPdbFileForReader()
	testFile := filepath.Join(tmpDir, "book.mobi")
	if err := os.WriteFile(testFile, pdbData, 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	metadata, err := reader.ReadMetadata(testFile)
	if err != nil {
		t.Fatalf("ReadMetadata() error: %v", err)
	}

	// Verify metadata structure
	if metadata.Title == "" {
		t.Logf("Title is empty (expected for minimal test file)")
	}
	if metadata.FileType != model.MOBI {
		t.Errorf("FileType = %v, want %v", metadata.FileType, model.MOBI)
	}
}

func TestMobiReaderLoadRecord(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "record.bin")
	content := []byte("0123456789ABCDEFGHIJ")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	reader := &MobiReader{}

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
			data, err := reader.loadRecord(path, tt.offset, tt.length)
			if tt.wantErr {
				if err == nil {
					t.Errorf("loadRecord() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("loadRecord() unexpected error: %v", err)
			}
			if string(data) != tt.want {
				t.Errorf("loadRecord() = %q, want %q", string(data), tt.want)
			}
		})
	}
}

func TestMobiReaderCover(t *testing.T) {
	tmpDir := t.TempDir()
	reader := &MobiReader{}

	t.Run("CoverRecordIdx returns 0", func(t *testing.T) {
		mobi := &Mobi{}
		result := reader.cover("/fake/path", &PdbDb{}, mobi)
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
		result := reader.cover("/fake/path", pdbDb, mobi)
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
		result := reader.cover("/fake/path", pdbDb, mobi)
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
		result := reader.cover("/fake/path", pdbDb, mobi)
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
		result := reader.cover("/fake/path", pdbDb, mobi)
		if result != nil {
			t.Errorf("cover() = %v, want nil", result)
		}
	})

	t.Run("oversized cover record returns nil", func(t *testing.T) {
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
					Length: MAX_EXPECTED_COVER_SIZE + 1,
				},
			},
		}
		result := reader.cover("/fake/path", pdbDb, mobi)
		if result != nil {
			t.Errorf("cover() = %v, want nil", result)
		}
	})

	t.Run("valid JPEG cover returns Resource with lazy Data", func(t *testing.T) {
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
		result := reader.cover(imgPath, pdbDb, mobi)
		if result == nil {
			t.Fatalf("cover() = nil, want Resource")
		}
		if result.Name != "cover.jpg" {
			t.Errorf("Name = %q, want %q", result.Name, "cover.jpg")
		}
		if result.MediaType != "image/jpeg" {
			t.Errorf("MediaType = %q, want %q", result.MediaType, "image/jpeg")
		}
		if result.Size != len(imgData) {
			t.Errorf("Size = %d, want %d", result.Size, len(imgData))
		}
		if result.Data == nil {
			t.Fatal("Data function is nil")
		}

		loaded, err := result.Data()
		if err != nil {
			t.Fatalf("Data() error: %v", err)
		}
		if string(loaded) != string(imgData) {
			t.Errorf("Data() = %x, want %x", loaded, imgData)
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
		result := reader.cover(imgPath, pdbDb, mobi)
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

// Helper function to build a valid PDB file for reader tests
func buildValidPdbFileForReader() []byte {
	var buf []byte

	// Create PDB header
	header := make([]byte, PDB_HEADER_SIZE)

	// Name
	copy(header[0:], "TestBook")

	// Attributes
	binary.BigEndian.PutUint16(header[32:34], 0x0000)

	// File version
	binary.BigEndian.PutUint16(header[34:36], 0x0000)

	// Created, Updated, Backup dates
	createdDate := uint32(PALM_EPOCH_OFFSET + 1000)
	binary.BigEndian.PutUint32(header[36:40], createdDate)
	binary.BigEndian.PutUint32(header[40:44], createdDate+86400)
	binary.BigEndian.PutUint32(header[44:48], 0)

	// Modification number
	binary.BigEndian.PutUint32(header[48:52], 1)

	// AppInfoOffset, SortInfoOffset
	binary.BigEndian.PutUint32(header[52:56], 0)
	binary.BigEndian.PutUint32(header[56:60], 0)

	// Type, Creator
	copy(header[60:64], "BOOK")
	copy(header[64:68], "MOBI")

	// UniqueIdSeed, NextRecordListId
	binary.BigEndian.PutUint32(header[68:72], 1)
	binary.BigEndian.PutUint32(header[72:76], 0)

	// NumberOfRecords
	binary.BigEndian.PutUint16(header[76:78], 1)

	buf = append(buf, header...)

	// Record info (8 bytes)
	recordInfo := make([]byte, 8)
	recordOffset := uint32(PDB_HEADER_SIZE + 8)
	binary.BigEndian.PutUint32(recordInfo[0:4], recordOffset)
	recordInfo[4] = 0x00
	copy(recordInfo[5:8], []byte{0x00, 0x00, 0x01})
	buf = append(buf, recordInfo...)

	// Create minimal MOBI record (100+ bytes for MOBI header)
	mobiData := make([]byte, 100)
	copy(mobiData[16:20], "MOBI")
	binary.BigEndian.PutUint32(mobiData[20:24], 232)
	binary.BigEndian.PutUint16(mobiData[0:2], uint16(CompressionNone))
	binary.BigEndian.PutUint32(mobiData[4:8], 1000)
	binary.BigEndian.PutUint16(mobiData[8:10], 1)
	binary.BigEndian.PutUint16(mobiData[10:12], 4096)
	binary.BigEndian.PutUint32(mobiData[28:32], uint32(UTF8))
	binary.BigEndian.PutUint32(mobiData[24:28], uint32(MobiTypeBook))

	buf = append(buf, mobiData...)

	return buf
}
