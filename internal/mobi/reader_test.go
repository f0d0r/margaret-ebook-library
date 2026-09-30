package mobi

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
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
		{"offset past file end returns empty (calibre parity)", 100, 5, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := readRecordData(book.NewPathBlob(path), tt.offset, tt.length, config.DefaultConfig().MaxRecordSize, int64(len(content)))
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
	reader := NewMobiReader(config.DefaultConfig())

	t.Run("CoverRecordIdx returns 0", func(t *testing.T) {
		mobi := &Mobi{}
		result := reader.cover(book.NewPathBlob("/fake/path"), &PdbDb{}, mobi)
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
		result := reader.cover(book.NewPathBlob("/fake/path"), pdbDb, mobi)
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
		result := reader.cover(book.NewPathBlob("/fake/path"), pdbDb, mobi)
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
		result := reader.cover(book.NewPathBlob("/fake/path"), pdbDb, mobi)
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
		result := reader.cover(book.NewPathBlob("/fake/path"), pdbDb, mobi)
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
		result := reader.cover(book.NewPathBlob("/fake/path"), pdbDb, mobi)
		if result == nil {
			t.Fatalf("cover() = nil, want resource with Open error")
		}
		if _, err := result.Open(); err == nil {
			t.Errorf("Open() = nil, want ErrLimitExceeded")
		}
	})

	t.Run("config override rejects cover", func(t *testing.T) {
		cfg := config.DefaultConfig()
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
		result := cfgReader.cover(book.NewPathBlob("/fake/path"), pdbDb, mobi)
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
		result := reader.cover(book.NewPathBlob(imgPath), pdbDb, mobi)
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
		result := reader.cover(book.NewPathBlob(imgPath), pdbDb, mobi)
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
	reader := NewMobiReader(config.DefaultConfig())

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

	b, err := book.NewFileBlob(f)
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
	reader := NewMobiReader(config.DefaultConfig())

	validPath := filepath.Join(tmpDir, "valid.mobi")
	validData := make([]byte, 60)
	validData = append(validData, []byte("BOOKMOBI")...)
	if err := os.WriteFile(validPath, validData, 0644); err != nil {
		t.Fatalf("failed to write valid MOBI file: %v", err)
	}

	if !reader.Supports(book.NewPathBlob(validPath)) {
		t.Error("Supports() = false for valid MOBI path blob, want true")
	}
}

// KF8/MOBI8 Tests
// ================

// Helper to create a minimal KF8 MOBI file structure
func makeTestKF8Blob(t *testing.T, kf8Config KF8TestConfig) []byte {
	t.Helper()
	
	// Build PDB header for KF8
	const pdbHeaderSize = 78
	numRecords := 2 + len(kf8Config.ExtraRecords) // header + text + extra
	if kf8Config.HasFDST {
		numRecords++
	}
	if kf8Config.HasSKEL {
		numRecords++
	}
	if kf8Config.HasDIV {
		numRecords++
	}
	
	header := make([]byte, pdbHeaderSize)
	copy(header[0:], "TestKF8")
	binary.BigEndian.PutUint16(header[32:34], 0)
	binary.BigEndian.PutUint16(header[34:36], 0)
	binary.BigEndian.PutUint32(header[36:40], 2082844800+1000) // creation time
	binary.BigEndian.PutUint32(header[40:44], 2082844800+1000+86400) // modification time
	binary.BigEndian.PutUint32(header[44:48], 0)
	binary.BigEndian.PutUint32(header[48:52], 1)
	binary.BigEndian.PutUint32(header[52:56], 0)
	binary.BigEndian.PutUint32(header[56:60], 0)
	copy(header[60:64], "BOOK")
	copy(header[64:68], "MOBI")
	binary.BigEndian.PutUint32(header[68:72], 1)
	binary.BigEndian.PutUint32(header[72:76], 0)
	binary.BigEndian.PutUint16(header[76:78], uint16(numRecords))
	
	buf := append([]byte{}, header...)
	
	// Build record info list
		recordOffsets := make([]uint32, numRecords)
		currentOffset := uint32(pdbHeaderSize + numRecords*8)

		for i := 0; i < numRecords; i++ {
			recordOffsets[i] = currentOffset
			ri := make([]byte, 8)
			binary.BigEndian.PutUint32(ri[0:4], currentOffset)
			ri[4] = 0
			copy(ri[5:8], []byte{0, 0, byte(i + 1)})
			buf = append(buf, ri...)

			// Compute size of this record
			switch i {
			case 0: // MOBI header
				currentOffset += uint32(kf8Config.MobiHeaderSize)
			case 1: // Text record
				currentOffset += uint32(len(kf8Config.TextData))
			default: // Extra records (FDST, SKEL, DIV in order)
				// Determine which extra record this is
				extraIdx := i - 2
				extraRecordCount := 0
				if kf8Config.HasFDST {
					if extraRecordCount == extraIdx {
						// FDST record - from ExtraRecords if provided
						if len(kf8Config.ExtraRecords) > 0 {
							currentOffset += uint32(len(kf8Config.ExtraRecords[0]))
						} else {
							// No FDST data provided, but flag is set - use minimal size
							currentOffset += 20
						}
						extraRecordCount++
						continue
					}
					extraRecordCount++
				}
				if kf8Config.HasSKEL {
					if extraRecordCount == extraIdx {
						// SKEL record size
						skelSize := 56 // INDX header only (count=0)
						currentOffset += uint32(skelSize)
						extraRecordCount++
						continue
					}
					extraRecordCount++
				}
				if kf8Config.HasDIV {
					if extraRecordCount == extraIdx {
						// DIV record size - just estimate
						currentOffset += 100
						extraRecordCount++
						continue
					}
					extraRecordCount++
				}
				// Fallback for ExtraRecords
				if extraIdx < len(kf8Config.ExtraRecords) {
					currentOffset += uint32(len(kf8Config.ExtraRecords[extraIdx]))
				}
			}
		}
	
	// Build MOBI header
	mobiHeader := make([]byte, kf8Config.MobiHeaderSize)
	copy(mobiHeader[16:20], "MOBI")
	binary.BigEndian.PutUint32(mobiHeader[20:24], 264) // header length (must be >= 0xF8+16 for KF8 indices)
	binary.BigEndian.PutUint16(mobiHeader[0:2], 1) // PalmDOC compression
	binary.BigEndian.PutUint32(mobiHeader[4:8], uint32(len(kf8Config.TextData))) // text length
	binary.BigEndian.PutUint16(mobiHeader[8:10], 1) // text record count
	binary.BigEndian.PutUint16(mobiHeader[10:12], 4096) // max record size
	binary.BigEndian.PutUint32(mobiHeader[24:28], 248) // mobiType = MobiTypeKF8 (248)
	binary.BigEndian.PutUint32(mobiHeader[28:32], 65001) // text encoding (UTF-8)
	binary.BigEndian.PutUint32(mobiHeader[36:40], 8) // mobiVersion = 8 (KF8)

		if kf8Config.WithDRM {
			binary.BigEndian.PutUint16(mobiHeader[12:14], 1) // encryption type
			binary.BigEndian.PutUint32(mobiHeader[180:184], 1) // DRM offset
		} else {
			// Explicitly set no-DRM values since headerLength >= 184 will cause readMobiHeader to read these fields
			binary.BigEndian.PutUint16(mobiHeader[12:14], 0) // EncryptionNone
			binary.BigEndian.PutUint32(mobiHeader[168:172], 0xFFFFFFFF) // DRMOffset = no DRM
			binary.BigEndian.PutUint32(mobiHeader[172:176], 0xFFFFFFFF) // DRMCount
			binary.BigEndian.PutUint32(mobiHeader[176:180], 0xFFFFFFFF) // DRMSize
			binary.BigEndian.PutUint32(mobiHeader[180:184], 0xFFFFFFFF) // DRMFlags
		}

		// KF8 specific indices - calculate based on actual record positions
		// Record 0: MOBI header, Record 1: Text data, then ExtraRecords in order
		fdstIdx := NullIndex
		skelIdx := NullIndex
		divIdx := NullIndex

		// Simple approach: if HasFDST, it's the first extra record (index 2)
		// If HasSKEL, it's the next extra record after FDST
		// If HasDIV, it's after SKEL
		currentExtraIdx := uint32(2)
		if kf8Config.HasFDST {
				fdstIdx = currentExtraIdx
				binary.BigEndian.PutUint32(mobiHeader[192:196], fdstIdx) // 0xC0
				binary.BigEndian.PutUint32(mobiHeader[196:200], 2) // 0xC4 - FdstCount (must be > 1 to not be NullIndex)
				currentExtraIdx++
			} else {
				binary.BigEndian.PutUint32(mobiHeader[192:196], NullIndex)
			}

		if kf8Config.HasSKEL {
			skelIdx = currentExtraIdx
			binary.BigEndian.PutUint32(mobiHeader[252:256], skelIdx) // 0xFC
			currentExtraIdx++
		} else {
			binary.BigEndian.PutUint32(mobiHeader[252:256], NullIndex)
		}

		if kf8Config.HasDIV {
			divIdx = currentExtraIdx
			binary.BigEndian.PutUint32(mobiHeader[248:252], divIdx) // 0xF8
		} else {
			binary.BigEndian.PutUint32(mobiHeader[248:252], NullIndex)
		}
	
	buf = append(buf, mobiHeader...)
	
	// Add text data
		buf = append(buf, kf8Config.TextData...)

		// Add extra records (FDST, SKEL, DIV, etc.)
		for _, extraData := range kf8Config.ExtraRecords {
			buf = append(buf, extraData...)
		}

		// Add SKEL record if requested but not in ExtraRecords
				if kf8Config.HasSKEL {
					// Minimal SKEL record: INDX header with zero entries (no IDXT records to follow)
					skelData := make([]byte, 56) // INDX header (56 bytes)
					copy(skelData[0:4], "INDX")
					binary.BigEndian.PutUint32(skelData[4:8], 56) // HeaderLength = 56 (just the header)
					binary.BigEndian.PutUint32(skelData[8:12], 0)  // count=0 (no IDXT entries)
					binary.BigEndian.PutUint32(skelData[12:16], 56) // start=56 (past header)
					binary.BigEndian.PutUint32(skelData[16:20], 1)  // codec=utf-8
					binary.BigEndian.PutUint32(skelData[20:24], 0xE4) // total=228
					// No TAGX section since count=0
					buf = append(buf, skelData...)
				}

				return buf
	}

type KF8TestConfig struct {
	MobiHeaderSize int
	TextData       []byte
	ExtraRecords   [][]byte
	HasFDST        bool
	HasSKEL        bool  
	HasDIV         bool
	WithDRM        bool
}

// Helper to create FDST record
func makeFDSTRecord(flows [][2]int) []byte {
	data := make([]byte, 12+len(flows)*8)
	copy(data[0:4], "FDST")
	binary.BigEndian.PutUint32(data[4:8], 12) // section start
	binary.BigEndian.PutUint32(data[8:12], uint32(len(flows))) // num sections
	
	for i, flow := range flows {
		binary.BigEndian.PutUint32(data[12+i*8:12+i*8+4], uint32(flow[0]))
		binary.BigEndian.PutUint32(data[12+i*8+4:12+i*8+8], uint32(flow[1]))
	}
	return data
}

// Helper to create malformed FDST (for panic testing)
func makeMalformedFDSTRecord() []byte {
	data := make([]byte, 12)
	copy(data[0:4], "FDST") 
	binary.BigEndian.PutUint32(data[4:8], 12) // section start
	binary.BigEndian.PutUint32(data[8:12], 999) // invalid huge num sections
	// No actual section data - will cause out of bounds access
	return data
}

func TestKF8StandaloneBasicParsing(t *testing.T) {
	reader := NewMobiReader(config.DefaultConfig())

	testHTML := []byte("<html><body><h1>KF8 Test</h1><p>Hello KF8 world!</p></body></html>")

	testConfig := KF8TestConfig{
		MobiHeaderSize: 300,
		TextData:       testHTML,
		ExtraRecords:   [][]byte{},
		HasFDST:        false,
		HasSKEL:        true,  // Need at least SKEL or DIV for isMobi8 detection
		HasDIV:         false,
		WithDRM:        false,
	}
	
	blob := makeTestKF8Blob(t, testConfig)
	
	pdbDb, err := ReadPdbDb(book.NewBytesBlob(blob), config.DefaultConfig().MaxRecordSize)
	if err != nil {
		t.Fatalf("ReadPdbDb() error: %v", err)
	}
	
	mobiDoc, err := ReadMobi(pdbDb, config.DefaultConfig().MaxExthRecords) 
	if err != nil {
		t.Fatalf("ReadMobi() error: %v", err)
	}
	
	// Should detect as KF8
	if !isMobi8(mobiDoc) {
		t.Error("isMobi8() = false, want true for KF8")
	}
	
	// Test content extraction
	resources := reader.content(pdbDb, mobiDoc)
	if len(resources) == 0 {
		t.Fatal("content() returned no resources")
	}
	
	// For standalone KF8 without FDST/SKEL/DIV, should fallback to single resource
	if len(resources) != 1 {
		t.Errorf("content() returned %d resources, want 1 for basic KF8", len(resources))
	}
	
	rc, err := resources[0].Open()
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer rc.Close()
	
	content, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	
	if !bytes.Contains(content, []byte("KF8 Test")) {
		t.Errorf("content does not contain expected text, got: %s", content)
	}
}

func TestKF8WithFDSTFlowTable(t *testing.T) {
	testHTML := []byte("<html><body><div>Flow 1</div><div>Flow 2</div></body></html>")
	flows := [][2]int{{0, 30}, {30, len(testHTML)}}
	fdstData := makeFDSTRecord(flows)
	
	testConfig := KF8TestConfig{
		MobiHeaderSize: 300,
		TextData:       testHTML,
		ExtraRecords:   [][]byte{fdstData},
		HasFDST:        true,
		HasSKEL:        false,
		HasDIV:         false,
		WithDRM:        false,
	}
	
	blob := makeTestKF8Blob(t, testConfig)
	
	pdbDb, err := ReadPdbDb(book.NewBytesBlob(blob), config.DefaultConfig().MaxRecordSize)
	if err != nil {
		t.Fatalf("ReadPdbDb() error: %v", err)
	}
	
	mobiDoc, err := ReadMobi(pdbDb, config.DefaultConfig().MaxExthRecords)
	if err != nil {
		t.Fatalf("ReadMobi() error: %v", err)
	}
	
	// Test FDST parsing
		sections, err := loadSections(pdbDb)
		if err != nil {
			t.Fatalf("loadSections() error: %v", err)
		}

		// Debug: log sections
		for i, sec := range sections {
			ln := 20
			if len(sec) < ln {
				ln = len(sec)
			}
			t.Logf("Section %d: len=%d, first %d: %v", i, len(sec), ln, sec[:ln])
		}

		// For standalone KF8, indices in header are absolute, pass all sections
		// Debug: check mobiDoc indices
		t.Logf("mobiDoc.FdstIdx=%d, mobiDoc.SkelIdx=%d, mobiDoc.DivIdx=%d", mobiDoc.FdstIdx, mobiDoc.SkelIdx, mobiDoc.DivIdx)
		flowTable, _, _, err := readMobi8Indices(sections, mobiDoc, "utf-8")
		if err != nil {
			t.Fatalf("readMobi8Indices() error: %v", err)
		}
	
	if len(flowTable) != 2 {
		t.Errorf("flowTable length = %d, want 2", len(flowTable))
	}
	
	if flowTable[0][0] != 0 || flowTable[0][1] != 30 {
		t.Errorf("flowTable[0] = %v, want [0 30]", flowTable[0])
	}
	
	if flowTable[1][0] != 30 || flowTable[1][1] != len(testHTML) {
		t.Errorf("flowTable[1] = %v, want [30 %d]", flowTable[1], len(testHTML))
	}
}

func TestKF8DRMDetection(t *testing.T) {
	reader := NewMobiReader(config.DefaultConfig())
	
	testHTML := []byte("<html><body>DRM protected content</body></html>")
	
	testConfig := KF8TestConfig{
		MobiHeaderSize: 300,
		TextData:       testHTML,
		ExtraRecords:   [][]byte{},
		HasFDST:        false,
		HasSKEL:        false,
		HasDIV:         false,
		WithDRM:        true, // Enable DRM
	}
	
	blob := makeTestKF8Blob(t, testConfig)
	
	_, err := reader.Read(book.NewBytesBlob(blob))
	if err == nil {
		t.Fatal("Read() should have failed for DRM-protected KF8")
	}
	
	if !strings.Contains(err.Error(), "DRM protected") {
		t.Errorf("Read() error = %v, want DRM protection error", err)
	}
}

func TestKF8PanicProtection(t *testing.T) {
	reader := NewMobiReader(config.DefaultConfig())
	
	tests := []struct {
		name           string
		malformedBlob  func() []byte
		expectedError  string
	}{
		{
			name: "malformed FDST record",
			malformedBlob: func() []byte {
				malformedFDST := makeMalformedFDSTRecord()
				testConfig := KF8TestConfig{
					MobiHeaderSize: 300,
					TextData:       []byte("<html><body>test</body></html>"),
					ExtraRecords:   [][]byte{malformedFDST},
					HasFDST:        true,
					HasSKEL:        false,
					HasDIV:         false,
					WithDRM:        false,
				}
				return makeTestKF8Blob(t, testConfig)
			},
			expectedError: "FDST",
		},
		{
			name: "corrupted text data",
			malformedBlob: func() []byte {
				testConfig := KF8TestConfig{
					MobiHeaderSize: 300,
					TextData:       []byte{0xFF, 0xFF, 0xFF}, // Invalid compressed data
					ExtraRecords:   [][]byte{},
					HasFDST:        false,
					HasSKEL:        false,
					HasDIV:         false,
					WithDRM:        false,
				}
				return makeTestKF8Blob(t, testConfig)
			},
			expectedError: "", // Should not panic, but may return error
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Unexpected panic in KF8 parsing: %v", r)
				}
			}()
			
			blob := tt.malformedBlob()
			
			// This should not panic, even with malformed data
			_, err := reader.Read(book.NewBytesBlob(blob))
			
			// We expect an error for malformed data, but not a panic
			if tt.expectedError != "" && err != nil {
				if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Read() error = %v, want error containing %q", err, tt.expectedError)
				}
			}
		})
	}
}

func TestKF8BuildMobi8Parts(t *testing.T) {
	rawML := []byte("<html><body><div id='part1'>Content 1</div><div id='part2'>Content 2</div></body></html>")
	
	// Test with empty flow table (should default to single flow)
	parts, partInfos, err := buildMobi8Parts(rawML, nil, nil, nil)
	if err != nil {
		t.Fatalf("buildMobi8Parts() error: %v", err)
	}
	
	if len(parts) != 1 {
		t.Errorf("parts length = %d, want 1", len(parts))
	}
	
	if len(partInfos) != 1 {
		t.Errorf("partInfos length = %d, want 1", len(partInfos))
	}
	
	if !bytes.Equal(parts[0], rawML) {
		t.Errorf("parts[0] = %s, want %s", parts[0], rawML)
	}
}

func TestKF8LoadSections(t *testing.T) {
	// Test panic protection in loadSections
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("loadSections() panicked: %v", r)
		}
	}()
	
	// Create a minimal PDB with one record
	blob := makeTestKF8Blob(t, KF8TestConfig{
		MobiHeaderSize: 300,
		TextData:       []byte("test"),
		ExtraRecords:   [][]byte{},
		HasFDST:        false,
		HasSKEL:        false,
		HasDIV:         false,
		WithDRM:        false,
	})
	
	pdbDb, err := ReadPdbDb(book.NewBytesBlob(blob), config.DefaultConfig().MaxRecordSize)
	if err != nil {
		t.Fatalf("ReadPdbDb() error: %v", err)
	}
	
	sections, err := loadSections(pdbDb)
	if err != nil {
		t.Fatalf("loadSections() error: %v", err)
	}
	
	if len(sections) != len(pdbDb.PdbRecords) {
		t.Errorf("sections length = %d, want %d", len(sections), len(pdbDb.PdbRecords))
	}
}

func TestKF8ExtractMobi8Raw(t *testing.T) {
	// Test panic protection in extractMobi8Raw
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("extractMobi8Raw() panicked: %v", r)
		}
	}()
	
	testHTML := []byte("<html><body>KF8 content</body></html>")
	blob := makeTestKF8Blob(t, KF8TestConfig{
		MobiHeaderSize: 300,
		TextData:       testHTML,
		ExtraRecords:   [][]byte{},
		HasFDST:        false,
		HasSKEL:        false,
		HasDIV:         false,
		WithDRM:        false,
	})
	
	pdbDb, err := ReadPdbDb(book.NewBytesBlob(blob), config.DefaultConfig().MaxRecordSize)
	if err != nil {
		t.Fatalf("ReadPdbDb() error: %v", err)
	}
	
	mobiDoc, err := ReadMobi(pdbDb, config.DefaultConfig().MaxExthRecords)
	if err != nil {
		t.Fatalf("ReadMobi() error: %v", err)
	}
	
	rawML, kf8Mobi, offset, err := extractMobi8Raw(pdbDb, mobiDoc, -1)
	if err != nil {
		t.Fatalf("extractMobi8Raw() error: %v", err)
	}
	
	if kf8Mobi == nil {
		t.Error("kf8Mobi is nil")
	}
	
	if offset != 1 {
		t.Errorf("offset = %d, want 1", offset)
	}
	
	if len(rawML) == 0 {
		t.Error("rawML is empty")
	}
}
