package mobi

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/f0d0r/margaret-ebook-library/pkg/model"
)

func TestParseAttributes(t *testing.T) {
	tests := []struct {
		name           string
		rawAttributes  uint16
		wantReadOnly   bool
		wantDirty      bool
		wantBackup     bool
		wantAllowNewer bool
		wantResetAfter bool
		wantNoBeam     bool
	}{
		{
			name:          "No attributes set",
			rawAttributes: 0x0000,
			wantReadOnly:  false,
			wantDirty:     false,
			wantBackup:    false,
		},
		{
			name:          "ReadOnly attribute",
			rawAttributes: AttrReadOnly,
			wantReadOnly:  true,
			wantDirty:     false,
			wantBackup:    false,
		},
		{
			name:          "Dirty and Backup attributes",
			rawAttributes: AttrDirtyAppInfo | AttrBackup,
			wantReadOnly:  false,
			wantDirty:     true,
			wantBackup:    true,
		},
		{
			name:           "AllowNewer and ResetAfter",
			rawAttributes:  AttrAllowNewer | AttrResetAfter,
			wantAllowNewer: true,
			wantResetAfter: true,
		},
		{
			name:           "All attributes set",
			rawAttributes:  0xFFFF,
			wantReadOnly:   true,
			wantDirty:      true,
			wantBackup:     true,
			wantAllowNewer: true,
			wantResetAfter: true,
			wantNoBeam:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAttributes(tt.rawAttributes)

			if got.ReadOnly != tt.wantReadOnly {
				t.Errorf("ReadOnly = %v, want %v", got.ReadOnly, tt.wantReadOnly)
			}
			if got.Dirty != tt.wantDirty {
				t.Errorf("Dirty = %v, want %v", got.Dirty, tt.wantDirty)
			}
			if got.Backup != tt.wantBackup {
				t.Errorf("Backup = %v, want %v", got.Backup, tt.wantBackup)
			}
			if got.AllowNewer != tt.wantAllowNewer {
				t.Errorf("AllowNewer = %v, want %v", got.AllowNewer, tt.wantAllowNewer)
			}
			if got.ResetAfter != tt.wantResetAfter {
				t.Errorf("ResetAfter = %v, want %v", got.ResetAfter, tt.wantResetAfter)
			}
			if got.NoBeam != tt.wantNoBeam {
				t.Errorf("NoBeam = %v, want %v", got.NoBeam, tt.wantNoBeam)
			}
		})
	}
}

func TestParsePalmDate(t *testing.T) {
	tests := []struct {
		name     string
		rawDate  uint32
		wantZero bool
		checkFn  func(t *testing.T, got time.Time)
	}{
		{
			name:     "Zero date",
			rawDate:  0,
			wantZero: true,
		},
		{
			name:     "Valid Palm date",
			rawDate:  uint32(PALM_EPOCH_OFFSET + 1000),
			wantZero: false,
			checkFn: func(t *testing.T, got time.Time) {
				if got.Unix() != 1000 {
					t.Errorf("Unix timestamp = %d, want 1000", got.Unix())
				}
			},
		},
		{
			name:     "PALM epoch start",
			rawDate:  uint32(PALM_EPOCH_OFFSET),
			wantZero: false,
			checkFn: func(t *testing.T, got time.Time) {
				if got.Unix() != 0 {
					t.Errorf("Unix timestamp = %d, want 0", got.Unix())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePalmDate(tt.rawDate)

			if tt.wantZero {
				if !got.IsZero() {
					t.Errorf("Expected zero time, got %v", got)
				}
			} else {
				if got.IsZero() {
					t.Errorf("Expected non-zero time, got zero")
				}
				if tt.checkFn != nil {
					tt.checkFn(t, got)
				}
			}
		})
	}
}

func TestParseRecordAttributes(t *testing.T) {
	tests := []struct {
		name         string
		raw          uint8
		wantSecret   bool
		wantBusy     bool
		wantDirty    bool
		wantDelete   bool
		wantCategory uint8
	}{
		{
			name:         "No attributes",
			raw:          0x00,
			wantSecret:   false,
			wantBusy:     false,
			wantDirty:    false,
			wantDelete:   false,
			wantCategory: 0,
		},
		{
			name:       "Secret attribute",
			raw:        RecordAttrSecret,
			wantSecret: true,
		},
		{
			name:     "Busy attribute",
			raw:      RecordAttrBusy,
			wantBusy: true,
		},
		{
			name:      "Dirty attribute",
			raw:       RecordAttrDirty,
			wantDirty: true,
		},
		{
			name:       "Delete attribute",
			raw:        RecordAttrDelete,
			wantDelete: true,
		},
		{
			name:         "Category mask (lower 4 bits)",
			raw:          0x05, // Category 5
			wantCategory: 5,
		},
		{
			name:         "All attributes with category",
			raw:          0xFF, // All bits set
			wantSecret:   true,
			wantBusy:     true,
			wantDirty:    true,
			wantDelete:   true,
			wantCategory: 0x0F, // Max category (15)
		},
		{
			name:         "Mixed attributes",
			raw:          RecordAttrSecret | RecordAttrDirty | 0x03, // Secret + Dirty + Category 3
			wantSecret:   true,
			wantDirty:    true,
			wantCategory: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRecordAttributes(tt.raw)

			if got.Secret != tt.wantSecret {
				t.Errorf("Secret = %v, want %v", got.Secret, tt.wantSecret)
			}
			if got.Busy != tt.wantBusy {
				t.Errorf("Busy = %v, want %v", got.Busy, tt.wantBusy)
			}
			if got.Dirty != tt.wantDirty {
				t.Errorf("Dirty = %v, want %v", got.Dirty, tt.wantDirty)
			}
			if got.Delete != tt.wantDelete {
				t.Errorf("Delete = %v, want %v", got.Delete, tt.wantDelete)
			}
			if got.Category != tt.wantCategory {
				t.Errorf("Category = %d, want %d", got.Category, tt.wantCategory)
			}
		})
	}
}

func TestParseRecordInfo(t *testing.T) {
	tests := []struct {
		name       string
		raw        []byte
		wantOffset uint32
		wantID     uint32
		wantSecret bool
	}{
		{
			name:       "Valid record info",
			raw:        []byte{0x00, 0x00, 0x01, 0x00, 0x10, 0x00, 0x00, 0x01}, // offset=256, secret=true, id=1
			wantOffset: 256,
			wantID:     1,
			wantSecret: true,
		},
		{
			name:       "Zero offset and ID",
			raw:        []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			wantOffset: 0,
			wantID:     0,
			wantSecret: false,
		},
		{
			name:       "High offset and ID",
			raw:        []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0xFF, 0xFF, 0xFF},
			wantOffset: 0xFFFFFFFF,
			wantID:     0xFFFFFF,
			wantSecret: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRecordInfo(tt.raw)
			if err != nil {
				t.Fatalf("parseRecordInfo() error: %v", err)
			}

			if got.Offset != tt.wantOffset {
				t.Errorf("Offset = %d, want %d", got.Offset, tt.wantOffset)
			}
			if got.UniqueId != tt.wantID {
				t.Errorf("UniqueId = %d, want %d", got.UniqueId, tt.wantID)
			}
			if got.Attributes.Secret != tt.wantSecret {
				t.Errorf("Secret = %v, want %v", got.Attributes.Secret, tt.wantSecret)
			}
		})
	}
}

func TestReadPdbDbValidFile(t *testing.T) {
	// Create a valid minimal PDB file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.pdb")

	pdbData := buildValidPdbFile()
	if err := os.WriteFile(testFile, pdbData, 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	f, err := os.Open(testFile)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer func() { _ = f.Close() }()

	pdb, err := ReadPdbDb(blobForFile(t, f), model.DefaultConfig().MaxRecordSize)
	if err != nil {
		t.Fatalf("ReadPdbDb() error: %v", err)
	}

	if pdb.Name != "TestBook" {
		t.Errorf("Name = %q, want %q", pdb.Name, "TestBook")
	}
	if pdb.Type != "BOOK" {
		t.Errorf("Type = %q, want %q", pdb.Type, "BOOK")
	}
	if pdb.Creator != "MOBI" {
		t.Errorf("Creator = %q, want %q", pdb.Creator, "MOBI")
	}
	if pdb.NumberOfRecords != 1 {
		t.Errorf("NumberOfRecords = %d, want %d", pdb.NumberOfRecords, 1)
	}
}

func TestReadPdbDbNonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := ReadPdbDb(model.NewPathBlob(filepath.Join(tmpDir, "missing.pdb")), model.DefaultConfig().MaxRecordSize)
	if err == nil {
		t.Errorf("ReadPdbDb() expected error for non-existent file, got nil")
	}
}

func TestReadPdbDbTruncatedHeader(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "truncated.pdb")

	// Write less than PDB_HEADER_SIZE bytes
	truncatedData := make([]byte, PDB_HEADER_SIZE-10)
	if err := os.WriteFile(testFile, truncatedData, 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	f, err := os.Open(testFile)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer func() { _ = f.Close() }()

	_, err = ReadPdbDb(blobForFile(t, f), model.DefaultConfig().MaxRecordSize)
	if err == nil {
		t.Errorf("ReadPdbDb() expected error for truncated header, got nil")
	}
}

func TestReadPdbDbAttributes(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "attrs.pdb")

	// Create PDB with specific attributes
	pdbData := buildValidPdbFile()

	// Set ReadOnly and Backup attributes in header
	header := pdbData[:PDB_HEADER_SIZE]
	attributes := AttrReadOnly | AttrBackup
	binary.BigEndian.PutUint16(header[32:34], attributes)

	if err := os.WriteFile(testFile, pdbData, 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	f, err := os.Open(testFile)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer func() { _ = f.Close() }()

	pdb, err := ReadPdbDb(blobForFile(t, f), model.DefaultConfig().MaxRecordSize)
	if err != nil {
		t.Fatalf("ReadPdbDb() error: %v", err)
	}

	if !pdb.Attributes.ReadOnly {
		t.Errorf("ReadOnly = false, want true")
	}
	if !pdb.Attributes.Backup {
		t.Errorf("Backup = false, want true")
	}
	if pdb.Attributes.Dirty {
		t.Errorf("Dirty = true, want false")
	}
}

func TestReadPdbDbDates(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "dates.pdb")

	pdbData := buildValidPdbFile()
	header := pdbData[:PDB_HEADER_SIZE]

	// Set creation date
	createdRaw := uint32(PALM_EPOCH_OFFSET + 86400) // 1 day after epoch
	binary.BigEndian.PutUint32(header[36:40], createdRaw)

	if err := os.WriteFile(testFile, pdbData, 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	f, err := os.Open(testFile)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer func() { _ = f.Close() }()

	pdb, err := ReadPdbDb(blobForFile(t, f), model.DefaultConfig().MaxRecordSize)
	if err != nil {
		t.Fatalf("ReadPdbDb() error: %v", err)
	}

	if pdb.CreatedAt.Unix() != 86400 {
		t.Errorf("CreatedAt Unix timestamp = %d, want 86400", pdb.CreatedAt.Unix())
	}
}

func TestLazyReadRecord(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "lazy_record.pdb")

	// Create a file with specific record data
	recordData := []byte("This is record data")
	offset := uint32(100)

	fileData := make([]byte, offset+uint32(len(recordData)))
	copy(fileData[offset:], recordData)

	if err := os.WriteFile(testFile, fileData, 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	// Open and read the record
	f, err := os.Open(testFile)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer func() { _ = f.Close() }()

	got, err := readRecordData(blobForFile(t, f), offset, uint32(len(recordData)), model.DefaultConfig().MaxRecordSize)
	if err != nil {
		t.Fatalf("readRecordData() error: %v", err)
	}

	if !bytes.Equal(got, recordData) {
		t.Errorf("Got %q, want %q", got, recordData)
	}
}

func TestReadRecordData(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "record_data.pdb")

	// Create a file with specific record data
	recordData := []byte("This is record data")
	offset := uint32(100)

	fileData := make([]byte, offset+uint32(len(recordData)))
	copy(fileData[offset:], recordData)

	if err := os.WriteFile(testFile, fileData, 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	// Open and read the record
	f, err := os.Open(testFile)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer func() { _ = f.Close() }()

	got, err := readRecordData(blobForFile(t, f), offset, uint32(len(recordData)), model.DefaultConfig().MaxRecordSize)
	if err != nil {
		t.Fatalf("readRecordData() error: %v", err)
	}

	if !bytes.Equal(got, recordData) {
		t.Errorf("Got %q, want %q", got, recordData)
	}
}

func TestPdbRecordGetData(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "record_data.pdb")

	pdbData := buildValidPdbFile()
	if err := os.WriteFile(testFile, pdbData, 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	f, err := os.Open(testFile)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer func() { _ = f.Close() }()

	pdb, err := ReadPdbDb(blobForFile(t, f), model.DefaultConfig().MaxRecordSize)
	if err != nil {
		t.Fatalf("ReadPdbDb() error: %v", err)
	}

	// Data should work for records
	if len(pdb.PdbRecords) > 0 {
		record := &pdb.PdbRecords[0]
		if record.Data == nil {
			t.Errorf("Data is nil")
			return
		}
		data, err := record.Data()
		if err != nil {
			t.Errorf("Data() error: %v", err)
		}
		if len(data) == 0 {
			t.Errorf("Data() returned empty data")
		}
	}
}

func TestReadPdbDbFileVersion(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "version.pdb")

	pdbData := buildValidPdbFile()
	header := pdbData[:PDB_HEADER_SIZE]

	// Set file version
	fileVersion := uint16(0x0102)
	binary.BigEndian.PutUint16(header[34:36], fileVersion)

	if err := os.WriteFile(testFile, pdbData, 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	f, err := os.Open(testFile)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer func() { _ = f.Close() }()

	pdb, err := ReadPdbDb(blobForFile(t, f), model.DefaultConfig().MaxRecordSize)
	if err != nil {
		t.Fatalf("ReadPdbDb() error: %v", err)
	}

	if pdb.FileVersion != fileVersion {
		t.Errorf("FileVersion = 0x%04x, want 0x%04x", pdb.FileVersion, fileVersion)
	}
}

func TestReadPdbDbMultipleRecords(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "multi_records.pdb")

	// Build a PDB file with multiple records
	pdbData := buildPdbFileWithRecords(3)

	if err := os.WriteFile(testFile, pdbData, 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	f, err := os.Open(testFile)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer func() { _ = f.Close() }()

	pdb, err := ReadPdbDb(blobForFile(t, f), model.DefaultConfig().MaxRecordSize)
	if err != nil {
		t.Fatalf("ReadPdbDb() error: %v", err)
	}

	if pdb.NumberOfRecords != 3 {
		t.Errorf("NumberOfRecords = %d, want 3", pdb.NumberOfRecords)
	}
	if len(pdb.PdbRecords) != 3 {
		t.Errorf("len(PdbRecords) = %d, want 3", len(pdb.PdbRecords))
	}
}

// Helper functions

func blobForFile(t *testing.T, f *os.File) model.Blob {
	t.Helper()
	b, err := model.NewFileBlob(f)
	if err != nil {
		t.Fatalf("NewFileBlob() error: %v", err)
	}
	return b
}

func buildValidPdbFile() []byte {
	var buf bytes.Buffer

	// Create header
	header := make([]byte, PDB_HEADER_SIZE)

	// Name (32 bytes)
	copy(header[0:], "TestBook")

	// Attributes (2 bytes)
	binary.BigEndian.PutUint16(header[32:34], 0x0000)

	// File version (2 bytes)
	binary.BigEndian.PutUint16(header[34:36], 0x0000)

	// Created, Updated, Backup dates (12 bytes)
	createdDate := uint32(PALM_EPOCH_OFFSET + 1000)
	binary.BigEndian.PutUint32(header[36:40], createdDate)
	binary.BigEndian.PutUint32(header[40:44], createdDate+86400)
	binary.BigEndian.PutUint32(header[44:48], 0)

	// Modification number (4 bytes)
	binary.BigEndian.PutUint32(header[48:52], 1)

	// AppInfoOffset, SortInfoOffset (8 bytes)
	binary.BigEndian.PutUint32(header[52:56], 0)
	binary.BigEndian.PutUint32(header[56:60], 0)

	// Type (4 bytes)
	copy(header[60:64], "BOOK")

	// Creator (4 bytes)
	copy(header[64:68], "MOBI")

	// UniqueIdSeed (4 bytes)
	binary.BigEndian.PutUint32(header[68:72], 1)

	// NextRecordListId (4 bytes)
	binary.BigEndian.PutUint32(header[72:76], 0)

	// NumberOfRecords (2 bytes)
	binary.BigEndian.PutUint16(header[76:78], 1)

	buf.Write(header)

	// Record info (8 bytes per record)
	recordInfo := make([]byte, 8)
	recordOffset := uint32(PDB_HEADER_SIZE + 8) // After header and record info
	binary.BigEndian.PutUint32(recordInfo[0:4], recordOffset)
	recordInfo[4] = 0x00 // Attributes
	copy(recordInfo[5:8], []byte{0x00, 0x00, 0x01})

	buf.Write(recordInfo)

	// Record data
	recordData := []byte("Test record data")
	buf.Write(recordData)

	return buf.Bytes()
}

func buildPdbFileWithRecords(numRecords uint16) []byte {
	var buf bytes.Buffer

	// Create header
	header := make([]byte, PDB_HEADER_SIZE)

	copy(header[0:], "MultiRecordBook")
	binary.BigEndian.PutUint16(header[32:34], 0x0000)
	binary.BigEndian.PutUint16(header[34:36], 0x0000)

	createdDate := uint32(PALM_EPOCH_OFFSET + 1000)
	binary.BigEndian.PutUint32(header[36:40], createdDate)
	binary.BigEndian.PutUint32(header[40:44], createdDate+86400)
	binary.BigEndian.PutUint32(header[44:48], 0)

	binary.BigEndian.PutUint32(header[48:52], 1)
	binary.BigEndian.PutUint32(header[52:56], 0)
	binary.BigEndian.PutUint32(header[56:60], 0)
	copy(header[60:64], "BOOK")
	copy(header[64:68], "MOBI")
	binary.BigEndian.PutUint32(header[68:72], 1)
	binary.BigEndian.PutUint32(header[72:76], 0)
	binary.BigEndian.PutUint16(header[76:78], numRecords)

	buf.Write(header)

	// Record info for each record
	recordStartOffset := uint32(PDB_HEADER_SIZE + int(numRecords)*8)
	recordSize := uint32(100)

	for i := 0; i < int(numRecords); i++ {
		recordInfo := make([]byte, 8)
		offset := recordStartOffset + uint32(i)*recordSize
		binary.BigEndian.PutUint32(recordInfo[0:4], offset)
		recordInfo[4] = 0x00
		// Write unique ID in bytes 5-7
		idVal := uint32(i)
		recordInfo[5] = byte((idVal >> 16) & 0xFF)
		recordInfo[6] = byte((idVal >> 8) & 0xFF)
		recordInfo[7] = byte(idVal & 0xFF)
		buf.Write(recordInfo)
	}

	// Record data
	for i := 0; i < int(numRecords); i++ {
		recordData := make([]byte, recordSize)
		copy(recordData, []byte("Record data"))
		buf.Write(recordData)
	}

	return buf.Bytes()
}
