package mobi

import (
	"encoding/binary"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/pkg/model"
)

func TestDecodeString(t *testing.T) {
	tests := []struct {
		name         string
		data         []byte
		textEncoding TextEncodingType
		want         string
	}{
		{
			name:         "Nil data",
			data:         nil,
			textEncoding: UTF8,
			want:         "",
		},
		{
			name:         "Empty data",
			data:         []byte{},
			textEncoding: UTF8,
			want:         "",
		},
		{
			name:         "Valid UTF-8",
			data:         []byte("Hello World"),
			textEncoding: UTF8,
			want:         "Hello World",
		},
		{
			name:         "UTF-8 with special chars",
			data:         []byte("Café & Crème"),
			textEncoding: UTF8,
			want:         "Café & Crème",
		},
		{
			name:         "Valid UTF-8 with emoji",
			data:         []byte("Book 📚"),
			textEncoding: UTF8,
			want:         "Book 📚",
		},
		{
			name:         "CP1252 encoding (will attempt decode)",
			data:         []byte("Hello"),
			textEncoding: CP1252,
			want:         "Hello", // ASCII is valid UTF-8, so returns as-is
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeString(tt.data, tt.textEncoding)
			if got != tt.want {
				t.Errorf("decodeString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMobiHasEXTH(t *testing.T) {
	tests := []struct {
		name     string
		exthFlag uint32
		want     bool
	}{
		{
			name:     "EXTH flag set (0x40)",
			exthFlag: 0x40,
			want:     true,
		},
		{
			name:     "EXTH flag set with other bits",
			exthFlag: 0x40 | 0x80,
			want:     true,
		},
		{
			name:     "EXTH flag not set",
			exthFlag: 0x00,
			want:     false,
		},
		{
			name:     "Different bit set, not EXTH",
			exthFlag: 0x80,
			want:     false,
		},
		{
			name:     "All bits set",
			exthFlag: 0xFFFFFFFF,
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mobi := &Mobi{EXTHFlags: tt.exthFlag}
			got := mobi.HasEXTH()
			if got != tt.want {
				t.Errorf("HasEXTH() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMobiHasDRM(t *testing.T) {
	tests := []struct {
		name      string
		drmOffset uint32
		want      bool
	}{
		{
			name:      "DRM protected (offset != 0xFFFFFFFF)",
			drmOffset: 1000,
			want:      true,
		},
		{
			name:      "DRM protected (offset = 0)",
			drmOffset: 0,
			want:      true,
		},
		{
			name:      "No DRM (offset = 0xFFFFFFFF)",
			drmOffset: 0xFFFFFFFF,
			want:      false,
		},
		{
			name:      "DRM protected (high offset)",
			drmOffset: 0xFFFFFFFE,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mobi := &Mobi{DRMOffset: tt.drmOffset}
			got := mobi.HasDRM()
			if got != tt.want {
				t.Errorf("HasDRM() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMobiTitle(t *testing.T) {
	tests := []struct {
		name      string
		mobiTitle string
		exth      *Exth
		want      string
	}{
		{
			name:      "Only MOBI header title",
			mobiTitle: "Book Title",
			exth:      nil,
			want:      "Book Title",
		},
		{
			name:      "MOBI header title with nil EXTH",
			mobiTitle: "Main Title",
			exth:      nil,
			want:      "Main Title",
		},
		{
			name:      "Prefer EXTH updated title over header",
			mobiTitle: "Header Title",
			exth: &Exth{
				Records:      map[uint32][][]byte{UPDATED_TITLE: {[]byte("Updated Title")}},
				textEncoding: UTF8,
			},
			want: "Updated Title",
		},
		{
			name:      "Fall back to header when EXTH title empty",
			mobiTitle: "Fallback Title",
			exth: &Exth{
				Records:      map[uint32][][]byte{},
				textEncoding: UTF8,
			},
			want: "Fallback Title",
		},
		{
			name:      "EXTH title takes precedence",
			mobiTitle: "Original",
			exth: &Exth{
				Records:      map[uint32][][]byte{UPDATED_TITLE: {[]byte("New Title")}},
				textEncoding: UTF8,
			},
			want: "New Title",
		},
		{
			name:      "Empty both titles",
			mobiTitle: "",
			exth: &Exth{
				Records:      map[uint32][][]byte{},
				textEncoding: UTF8,
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mobi := &Mobi{
				title: tt.mobiTitle,
				EXTH:  tt.exth,
			}
			got := mobi.Title()
			if got != tt.want {
				t.Errorf("Title() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMobiAuthors(t *testing.T) {
	tests := []struct {
		name string
		kf8  *Mobi
		exth *Exth
		want []string
	}{
		{
			name: "No EXTH, no KF8",
			exth: nil,
			want: nil,
		},
		{
			name: "EXTH with authors",
			exth: &Exth{
				Records:      map[uint32][][]byte{AUTHOR: {[]byte("Author One"), []byte("Author Two")}},
				textEncoding: UTF8,
			},
			want: []string{"Author One", "Author Two"},
		},
		{
			name: "Empty EXTH authors",
			exth: &Exth{
				Records:      map[uint32][][]byte{},
				textEncoding: UTF8,
			},
			want: nil,
		},
		{
			name: "KF8 authors preferred over EXTH",
			kf8: &Mobi{
				EXTH: &Exth{
					Records:      map[uint32][][]byte{AUTHOR: {[]byte("KF8 Author")}},
					textEncoding: UTF8,
				},
			},
			exth: &Exth{
				Records:      map[uint32][][]byte{AUTHOR: {[]byte("EXTH Author")}},
				textEncoding: UTF8,
			},
			want: []string{"KF8 Author"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mobi := &Mobi{
				EXTH: tt.exth,
				KF8:  tt.kf8,
			}
			got := mobi.Authors()
			if len(got) == 0 && tt.want == nil {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("Authors() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("Authors()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestMobiDescription(t *testing.T) {
	tests := []struct {
		name string
		kf8  *Mobi
		exth *Exth
		want string
	}{
		{
			name: "No EXTH, no KF8",
			exth: nil,
			want: "",
		},
		{
			name: "EXTH with description",
			exth: &Exth{
				Records:      map[uint32][][]byte{DESCRIPTION: {[]byte("A great book.")}},
				textEncoding: UTF8,
			},
			want: "A great book.",
		},
		{
			name: "Empty EXTH description",
			exth: &Exth{
				Records:      map[uint32][][]byte{},
				textEncoding: UTF8,
			},
			want: "",
		},
		{
			name: "KF8 description preferred over EXTH",
			kf8: &Mobi{
				EXTH: &Exth{
					Records:      map[uint32][][]byte{DESCRIPTION: {[]byte("KF8 desc")}},
					textEncoding: UTF8,
				},
			},
			exth: &Exth{
				Records:      map[uint32][][]byte{DESCRIPTION: {[]byte("EXTH desc")}},
				textEncoding: UTF8,
			},
			want: "KF8 desc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mobi := &Mobi{
				EXTH: tt.exth,
				KF8:  tt.kf8,
			}
			got := mobi.Description()
			if got != tt.want {
				t.Errorf("Description() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadMobiValidHeader(t *testing.T) {
	// Create a minimal valid MOBI record (96+ bytes)
	data := buildValidMobiData()

	pdbDb := &PdbDb{
		PdbRecords: []PdbRecord{
			createMockPdbRecord(data),
		},
	}

	mobi, err := ReadMobi(pdbDb, model.DefaultConfig().MaxExthRecords)
	if err != nil {
		t.Fatalf("ReadMobi() error: %v", err)
	}

	if mobi.Identifier != "MOBI" {
		t.Errorf("Identifier = %q, want %q", mobi.Identifier, "MOBI")
	}
	if mobi.TextEncoding != UTF8 {
		t.Errorf("TextEncoding = %d, want %d", mobi.TextEncoding, UTF8)
	}
	if mobi.Type != MobiTypeBook {
		t.Errorf("Type = %d, want %d", mobi.Type, MobiTypeBook)
	}
}

func TestReadMobiNoRecords(t *testing.T) {
	pdbDb := &PdbDb{
		PdbRecords: []PdbRecord{},
	}

	_, err := ReadMobi(pdbDb, model.DefaultConfig().MaxExthRecords)
	if err == nil {
		t.Errorf("ReadMobi() expected error for empty record list, got nil")
	}
}

func TestReadMobiTooShort(t *testing.T) {
	pdbDb := &PdbDb{
		PdbRecords: []PdbRecord{
			createMockPdbRecord(make([]byte, 50)), // Too short
		},
	}

	_, err := ReadMobi(pdbDb, model.DefaultConfig().MaxExthRecords)
	if err == nil {
		t.Errorf("ReadMobi() expected error for short record, got nil")
	}
}

func TestReadMobiExtendedHeader(t *testing.T) {
	// Create extended MOBI header (>= 184 bytes)
	data := make([]byte, 184)
	copy(data[16:20], "MOBI")
	binary.BigEndian.PutUint32(data[20:24], 232) // HeaderLength
	binary.BigEndian.PutUint16(data[0:2], uint16(CompressionNone))
	binary.BigEndian.PutUint32(data[4:8], 1000) // TextLength
	binary.BigEndian.PutUint16(data[8:10], 1)   // TextRecordCount
	binary.BigEndian.PutUint16(data[10:12], 4096)
	binary.BigEndian.PutUint32(data[28:32], uint32(UTF8))
	binary.BigEndian.PutUint32(data[24:28], uint32(MobiTypeBook))

	// Extended header fields
	binary.BigEndian.PutUint32(data[96:100], 1033) // InputLanguage (US English)
	// Don't set EXTHFlags (0x40) to avoid EXTH parsing with insufficient data

	pdbDb := &PdbDb{
		PdbRecords: []PdbRecord{
			createMockPdbRecord(data),
		},
	}

	mobi, err := ReadMobi(pdbDb, model.DefaultConfig().MaxExthRecords)
	if err != nil {
		t.Fatalf("ReadMobi() error: %v", err)
	}

	if mobi.InputLanguage != 1033 {
		t.Errorf("InputLanguage = %d, want %d", mobi.InputLanguage, 1033)
	}
}

func TestReadMobiFullHeader(t *testing.T) {
	// Create full MOBI header (>= 248 bytes)
	data := make([]byte, 250)
	copy(data[16:20], "MOBI")
	binary.BigEndian.PutUint32(data[20:24], 248) // HeaderLength
	binary.BigEndian.PutUint16(data[0:2], uint16(CompressionNone))
	binary.BigEndian.PutUint32(data[4:8], 5000) // TextLength
	binary.BigEndian.PutUint16(data[8:10], 1)
	binary.BigEndian.PutUint16(data[10:12], 4096)
	binary.BigEndian.PutUint32(data[28:32], uint32(UTF8))
	binary.BigEndian.PutUint32(data[24:28], uint32(MobiTypeKF8))

	// Extended header fields
	binary.BigEndian.PutUint32(data[192:196], 5000) // FirstTextRecord
	binary.BigEndian.PutUint32(data[200:204], 10)   // FCISRecordOffset
	binary.BigEndian.PutUint32(data[244:248], 5000) // IndxRecordOffset

	pdbDb := &PdbDb{
		PdbRecords: []PdbRecord{
			createMockPdbRecord(data),
		},
	}

	mobi, err := ReadMobi(pdbDb, model.DefaultConfig().MaxExthRecords)
	if err != nil {
		t.Fatalf("ReadMobi() error: %v", err)
	}

	if mobi.IndxRecordOffset != 5000 {
		t.Errorf("IndxRecordOffset = %d, want %d", mobi.IndxRecordOffset, 5000)
	}
	if mobi.Type != MobiTypeKF8 {
		t.Errorf("Type = %d, want %d", mobi.Type, MobiTypeKF8)
	}
}

func TestReadMobiDRMDefaults(t *testing.T) {
	// Create minimal MOBI record
	data := buildValidMobiData()

	pdbDb := &PdbDb{
		PdbRecords: []PdbRecord{
			createMockPdbRecord(data),
		},
	}

	mobi, err := ReadMobi(pdbDb, model.DefaultConfig().MaxExthRecords)
	if err != nil {
		t.Fatalf("ReadMobi() error: %v", err)
	}

	// Check DRM defaults (0xFFFFFFFF means no DRM)
	if mobi.DRMOffset != 0xFFFFFFFF {
		t.Errorf("DRMOffset default = 0x%x, want 0x%x", mobi.DRMOffset, 0xFFFFFFFF)
	}
	if mobi.HasDRM() {
		t.Errorf("HasDRM() = true, want false for default")
	}
}

func TestReadMobiCompressionTypes(t *testing.T) {
	tests := []struct {
		name        string
		compression CompressionType
	}{
		{"CompressionNone", CompressionNone},
		{"CompressionPalmDOC", CompressionPalmDOC},
		{"CompressionHUFF", CompressionHUFF},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := buildValidMobiData()
			binary.BigEndian.PutUint16(data[0:2], uint16(tt.compression))

			pdbDb := &PdbDb{
				PdbRecords: []PdbRecord{
					createMockPdbRecord(data),
				},
			}

			mobi, err := ReadMobi(pdbDb, model.DefaultConfig().MaxExthRecords)
			if err != nil {
				t.Fatalf("ReadMobi() error: %v", err)
			}

			if mobi.Compression != tt.compression {
				t.Errorf("Compression = %d, want %d", mobi.Compression, tt.compression)
			}
		})
	}
}

func TestMobiLocaleCode(t *testing.T) {
	tests := []struct {
		name   string
		locale uint32
		want   string
	}{
		{
			name:   "Unknown LCID",
			locale: 0x0000,
			want:   "",
		},
		{
			name:   "Hungarian",
			locale: 0x040E,
			want:   "hu",
		},
		{
			name:   "US English",
			locale: 0x0409,
			want:   "en-US",
		},
		{
			name:   "UK English",
			locale: 0x0809,
			want:   "en-GB",
		},
		{
			name:   "German Germany",
			locale: 0x0407,
			want:   "de-DE",
		},
		{
			name:   "French Canada",
			locale: 0x0C0C,
			want:   "fr-CA",
		},
		{
			name:   "Arabic UAE",
			locale: 0x3801,
			want:   "ar-AE",
		},
		{
			name:   "Spanish Mexico",
			locale: 0x080A,
			want:   "es-MX",
		},
		{
			name:   "Spanish Spain (International Sort)",
			locale: 0x0C0A,
			want:   "es-ES",
		},
		{
			name:   "Chinese Simplified",
			locale: 0x0004,
			want:   "zh-Hans",
		},
		{
			name:   "Chinese Traditional",
			locale: 0x7C04,
			want:   "zh-Hant",
		},
		{
			name:   "Serbian Cyrillic Serbia",
			locale: 0x0C1A,
			want:   "sr-Cyrl-RS",
		},
		{
			name:   "Upper bits masked out (0xZZZZ0409)",
			locale: 0xDEAD0409,
			want:   "en-US",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mobi := &Mobi{Locale: tt.locale}
			got := mobi.localeCode()
			if got != tt.want {
				t.Errorf("localeCode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMobiLanguage(t *testing.T) {
	tests := []struct {
		name   string
		kf8    *Mobi
		exth   *Exth
		locale uint32
		want   string
	}{
		{
			name:   "Falls back to localeCode when no KF8 or EXTH",
			locale: 0x0809, // UK English
			want:   "en-GB",
		},
		{
			name:   "Unknown locale returns empty",
			locale: 0x0000,
			want:   "",
		},
		{
			name: "EXTH language overrides localeCode",
			exth: &Exth{
				Records:      map[uint32][][]byte{LANGUAGE: {[]byte("fr")}},
				textEncoding: UTF8,
			},
			locale: 0x0409, // en-US
			want:   "fr",
		},
		{
			name: "EXTH empty language does not override localeCode",
			exth: &Exth{
				Records:      map[uint32][][]byte{},
				textEncoding: UTF8,
			},
			locale: 0x040E, // hu
			want:   "hu",
		},
		{
			name: "KF8 language overrides EXTH and localeCode",
			kf8: &Mobi{
				EXTH: &Exth{
					Records:      map[uint32][][]byte{LANGUAGE: {[]byte("de")}},
					textEncoding: UTF8,
				},
			},
			exth: &Exth{
				Records:      map[uint32][][]byte{LANGUAGE: {[]byte("fr")}},
				textEncoding: UTF8,
			},
			locale: 0x0409, // en-US
			want:   "de",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mobi := &Mobi{
				KF8:    tt.kf8,
				EXTH:   tt.exth,
				Locale: tt.locale,
			}
			got := mobi.Language()
			if got != tt.want {
				t.Errorf("Language() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMobiCoverRecordIdx(t *testing.T) {
	uint32be := func(v uint32) []byte {
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, v)
		return b
	}

	tests := []struct {
		name string
		mobi *Mobi
		want uint32
	}{
		{
			name: "no EXTH returns 0",
			mobi: &Mobi{},
			want: 0,
		},
		{
			name: "EXTH but FirstImageRecord=0 returns 0",
			mobi: &Mobi{
				EXTH:             &Exth{Records: map[uint32][][]byte{}},
				FirstImageRecord: 0,
				recordCount:      10,
			},
			want: 0,
		},
		{
			name: "EXTH but FirstImageRecord=0xFFFF returns 0",
			mobi: &Mobi{
				EXTH:             &Exth{Records: map[uint32][][]byte{}},
				FirstImageRecord: 0xFFFF,
				recordCount:      10,
			},
			want: 0,
		},
		{
			name: "EXTH but FirstImageRecord=0xFFFFFFFF returns 0",
			mobi: &Mobi{
				EXTH:             &Exth{Records: map[uint32][][]byte{}},
				FirstImageRecord: 0xFFFFFFFF,
				recordCount:      10,
			},
			want: 0,
		},
		{
			name: "valid EXTH + FirstImageRecord + CoverOffset",
			mobi: &Mobi{
				EXTH: &Exth{
					Records: map[uint32][][]byte{
						COVER_OFFSET: {uint32be(3)},
					},
				},
				FirstImageRecord: 2,
				recordCount:      10,
			},
			want: 5,
		},
		{
			name: "absIdx >= recordCount returns 0",
			mobi: &Mobi{
				EXTH: &Exth{
					Records: map[uint32][][]byte{
						COVER_OFFSET: {uint32be(10)},
					},
				},
				FirstImageRecord: 8,
				recordCount:      10,
			},
			want: 0,
		},
		{
			name: "KF8 path returns combined index",
			mobi: &Mobi{
				EXTH: &Exth{
					Records: map[uint32][][]byte{
						KF8_HEADER_INDEX: {uint32be(5)},
					},
				},
				KF8: &Mobi{
					EXTH: &Exth{
						Records: map[uint32][][]byte{
							COVER_OFFSET: {uint32be(2)},
						},
					},
					FirstImageRecord: 1,
					recordCount:      10,
				},
				recordCount: 20,
			},
			want: 8,
		},
		{
			name: "KF8 path returns 0 falls through to EXTH path",
			mobi: &Mobi{
				EXTH: &Exth{
					Records: map[uint32][][]byte{
						KF8_HEADER_INDEX: {uint32be(5)},
						COVER_OFFSET:     {uint32be(3)},
					},
				},
				KF8: &Mobi{
					recordCount: 0,
				},
				FirstImageRecord: 2,
				recordCount:      20,
			},
			want: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.mobi.CoverRecordIdx()
			if got != tt.want {
				t.Errorf("CoverRecordIdx() = %d, want %d", got, tt.want)
			}
		})
	}
}

// Helper functions

func createMockPdbRecord(data []byte) PdbRecord {
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	return PdbRecord{
		Offset: 0,
		Length: uint32(len(data)),
		Data: func() ([]byte, error) {
			return dataCopy, nil
		},
	}
}

func buildValidMobiData() []byte {
	data := make([]byte, 100)

	// Required fields (bytes 0-95)
	binary.BigEndian.PutUint16(data[0:2], uint16(CompressionNone))
	binary.BigEndian.PutUint32(data[4:8], 1000)   // TextLength
	binary.BigEndian.PutUint16(data[8:10], 1)     // TextRecordCount
	binary.BigEndian.PutUint16(data[10:12], 4096) // MaxTextRecordSize
	binary.BigEndian.PutUint16(data[12:14], uint16(EncryptionNone))
	copy(data[16:20], "MOBI")                    // Identifier
	binary.BigEndian.PutUint32(data[20:24], 232) // HeaderLength
	binary.BigEndian.PutUint32(data[24:28], uint32(MobiTypeBook))
	binary.BigEndian.PutUint32(data[28:32], uint32(UTF8)) // TextEncoding
	binary.BigEndian.PutUint32(data[36:40], 6)            // MobiVersion
	binary.BigEndian.PutUint32(data[80:84], 100)          // FirstNonTextRecord
	binary.BigEndian.PutUint32(data[84:88], 0)            // FullNameOffset
	binary.BigEndian.PutUint32(data[88:92], 0)            // FullNameLength
	binary.BigEndian.PutUint32(data[92:96], 1033)         // Locale (US English)

	return data
}
