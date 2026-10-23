package mobi

import (
	"encoding/binary"
	"reflect"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/internal/config"
)

func TestReadExth(t *testing.T) {
	tests := []struct {
		name          string
		offset        uint32
		data          []byte
		textEncoding  TextEncodingType
		wantErr       error
		wantIdent     string
		wantHeaderLen uint32
		wantRecCount  uint32
	}{
		{
			name:    "Invalid offset",
			offset:  1000,
			data:    make([]byte, 50),
			wantErr: errInvalidOffset,
		},
		{
			name:          "Valid EXTH header with no records",
			offset:        0,
			data:          buildExthData(0),
			textEncoding:  UTF8,
			wantIdent:     "EXTH",
			wantHeaderLen: 12,
			wantRecCount:  0,
		},
		{
			name:          "Valid EXTH with one record",
			offset:        0,
			data:          buildExthDataWithRecords(map[uint32]string{100: "test"}),
			textEncoding:  UTF8,
			wantIdent:     "EXTH",
			wantHeaderLen: 12,
			wantRecCount:  1,
		},
		{
			name:         "Valid EXTH with multiple records",
			offset:       0,
			data:         buildExthDataWithRecords(map[uint32]string{100: "title", 101: "author", 103: "desc"}),
			textEncoding: UTF8,
			wantIdent:    "EXTH",
			wantRecCount: 3,
		},
		{
			name:         "Offset at valid position",
			offset:       10,
			data:         append(make([]byte, 10), buildExthData(0)...),
			textEncoding: CP1252,
			wantIdent:    "EXTH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadExth(tt.offset, tt.data, tt.textEncoding, config.DefaultConfig().MaxExthRecords)

			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("ReadExth() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("ReadExth() unexpected error: %v", err)
			}

			if got.Identifier != tt.wantIdent {
				t.Errorf("Identifier = %q, want %q", got.Identifier, tt.wantIdent)
			}
			if got.RecordCount != tt.wantRecCount {
				t.Errorf("RecordCount = %d, want %d", got.RecordCount, tt.wantRecCount)
			}
		})
	}
}

func TestExthUpdatedTitle(t *testing.T) {
	tests := []struct {
		name         string
		records      map[uint32]string
		textEncoding TextEncodingType
		wantTitle    string
	}{
		{
			name:         "No updated title record",
			records:      map[uint32]string{100: "other"},
			textEncoding: UTF8,
			wantTitle:    "",
		},
		{
			name:         "UTF-8 title",
			records:      map[uint32]string{UPDATED_TITLE: "My Book Title"},
			textEncoding: UTF8,
			wantTitle:    "My Book Title",
		},
		{
			name:         "Empty title",
			records:      map[uint32]string{UPDATED_TITLE: ""},
			textEncoding: UTF8,
			wantTitle:    "",
		},
		{
			name:         "Title with special characters UTF-8",
			records:      map[uint32]string{UPDATED_TITLE: "Café & Crème"},
			textEncoding: UTF8,
			wantTitle:    "Café & Crème",
		},
		{
			name:         "Multiple records with updated title",
			records:      map[uint32]string{100: "author", UPDATED_TITLE: "The Title", 102: "publisher"},
			textEncoding: UTF8,
			wantTitle:    "The Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exth := &Exth{
				Identifier:   "EXTH",
				HeaderLength: 12,
				RecordCount:  uint32(len(tt.records)),
				Records:      make(map[uint32][][]byte),
				textEncoding: tt.textEncoding,
			}

			for recType, data := range tt.records {
				exth.Records[recType] = [][]byte{[]byte(data)}
			}

			got := exth.UpdatedTitle()
			if got != tt.wantTitle {
				t.Errorf("UpdatedTitle() = %q, want %q", got, tt.wantTitle)
			}
		})
	}
}

func TestExthCP1252Encoding(t *testing.T) {
	// Test CP1252 encoding with special character
	// 0x92 in CP1252 is the right single quotation mark (')
	exth := &Exth{
		Identifier:   "EXTH",
		HeaderLength: 12,
		RecordCount:  1,
		Records: map[uint32][][]byte{
			UPDATED_TITLE: {{0x54, 0x6f, 0x6d, 0x92, 0x73}}, // "Tom's" with CP1252 apostrophe
		},
		textEncoding: CP1252,
	}

	got := exth.UpdatedTitle()
	// CP1252 0x92 should decode properly
	if len(got) == 0 {
		t.Errorf("UpdatedTitle() returned empty string for CP1252 encoded data")
	}
}

func TestExthDescription(t *testing.T) {
	tests := []struct {
		name         string
		records      map[uint32]string
		textEncoding TextEncodingType
		want         string
	}{
		{
			name:         "No description record",
			records:      map[uint32]string{100: "other"},
			textEncoding: UTF8,
			want:         "",
		},
		{
			name:         "UTF-8 description",
			records:      map[uint32]string{DESCRIPTION: "A great book about Go programming."},
			textEncoding: UTF8,
			want:         "A great book about Go programming.",
		},
		{
			name:         "Empty description",
			records:      map[uint32]string{DESCRIPTION: ""},
			textEncoding: UTF8,
			want:         "",
		},
		{
			name:         "Description with special characters",
			records:      map[uint32]string{DESCRIPTION: "Café & Crème — a story"},
			textEncoding: UTF8,
			want:         "Café & Crème — a story",
		},
		{
			name:         "Multiple records with description",
			records:      map[uint32]string{100: "author", DESCRIPTION: "The book description", 102: "publisher"},
			textEncoding: UTF8,
			want:         "The book description",
		},
		{
			name:         "Description with CP1252 encoding",
			records:      map[uint32]string{DESCRIPTION: string([]byte{0x49, 0x74, 0x92, 0x73, 0x20, 0x67, 0x72, 0x65, 0x61, 0x74})}, // "It's great" with CP1252 right single quote (0x92)
			textEncoding: CP1252,
			want:         "It\u2019s great",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exth := &Exth{
				Identifier:   "EXTH",
				HeaderLength: 12,
				RecordCount:  uint32(len(tt.records)),
				Records:      make(map[uint32][][]byte),
				textEncoding: tt.textEncoding,
			}

			for recType, data := range tt.records {
				exth.Records[recType] = [][]byte{[]byte(data)}
			}

			got := exth.Description()
			if got != tt.want {
				t.Errorf("Description() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExthAuthors(t *testing.T) {
	tests := []struct {
		name         string
		records      map[uint32]string
		textEncoding TextEncodingType
		want         []string
	}{
		{
			name:         "No author record",
			records:      map[uint32]string{101: "other"},
			textEncoding: UTF8,
			want:         nil,
		},
		{
			name:         "Single author",
			records:      map[uint32]string{AUTHOR: "J.R.R. Tolkien"},
			textEncoding: UTF8,
			want:         []string{"J.R.R. Tolkien"},
		},
		{
			name:         "Author with special characters",
			records:      map[uint32]string{AUTHOR: "José Saramago"},
			textEncoding: UTF8,
			want:         []string{"José Saramago"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exth := &Exth{
				Identifier:   "EXTH",
				HeaderLength: 12,
				RecordCount:  uint32(len(tt.records)),
				Records:      make(map[uint32][][]byte),
				textEncoding: tt.textEncoding,
			}
			for recType, data := range tt.records {
				exth.Records[recType] = [][]byte{[]byte(data)}
			}

			got := exth.Authors()
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

func TestExthSubjects(t *testing.T) {
	tests := []struct {
		name         string
		records      map[uint32][][]byte
		textEncoding TextEncodingType
		want         []string
	}{
		{
			name:         "No subject record",
			records:      map[uint32][][]byte{AUTHOR: {[]byte("Author")}},
			textEncoding: UTF8,
			want:         nil,
		},
		{
			name:         "Single subject",
			records:      map[uint32][][]byte{SUBJECT: {[]byte("Fantasy")}},
			textEncoding: UTF8,
			want:         []string{"Fantasy"},
		},
		{
			name: "Multiple records in source order",
			records: map[uint32][][]byte{
				SUBJECT: {[]byte("Fantasy"), []byte("Adventure")},
			},
			textEncoding: UTF8,
			want:         []string{"Fantasy", "Adventure"},
		},
		{
			name:         "Semicolons split",
			records:      map[uint32][][]byte{SUBJECT: {[]byte("Fantasy; Adventure")}},
			textEncoding: UTF8,
			want:         []string{"Fantasy", "Adventure"},
		},
		{
			name: "Records and semicolons preserve order",
			records: map[uint32][][]byte{
				SUBJECT: {[]byte("A; B"), []byte("C")},
			},
			textEncoding: UTF8,
			want:         []string{"A", "B", "C"},
		},
		{
			name:         "Parts trimmed and empty dropped",
			records:      map[uint32][][]byte{SUBJECT: {[]byte(" Science ; ; History ; ")}},
			textEncoding: UTF8,
			want:         []string{"Science", "History"},
		},
		{
			name:         "CP1252 subject",
			records:      map[uint32][][]byte{SUBJECT: {cp1252Bytes(t, "Café; History")}},
			textEncoding: CP1252,
			want:         []string{"Café", "History"},
		},
		{
			name:         "Duplicates preserved",
			records:      map[uint32][][]byte{SUBJECT: {[]byte("A; A"), []byte("A")}},
			textEncoding: UTF8,
			want:         []string{"A", "A", "A"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exth := &Exth{Records: tt.records, textEncoding: tt.textEncoding}
			if got := exth.Subjects(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Subjects() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestExthPublisher(t *testing.T) {
	tests := []struct {
		name         string
		records      map[uint32][][]byte
		textEncoding TextEncodingType
		want         string
	}{
		{
			name:         "No publisher record",
			records:      map[uint32][][]byte{AUTHOR: {[]byte("Author")}},
			textEncoding: UTF8,
			want:         "",
		},
		{
			name:         "UTF-8 publisher",
			records:      map[uint32][][]byte{PUBLISHER: {[]byte("Acme Press")}},
			textEncoding: UTF8,
			want:         "Acme Press",
		},
		{
			name:         "Whitespace trimmed",
			records:      map[uint32][][]byte{PUBLISHER: {[]byte("  Acme  Press  ")}},
			textEncoding: UTF8,
			want:         "Acme  Press",
		},
		{
			name:         "First non-empty wins",
			records:      map[uint32][][]byte{PUBLISHER: {[]byte(""), []byte("First"), []byte("Second")}},
			textEncoding: UTF8,
			want:         "First",
		},
		{
			name:         "CP1252 publisher",
			records:      map[uint32][][]byte{PUBLISHER: {cp1252Bytes(t, "Café Press")}},
			textEncoding: CP1252,
			want:         "Café Press",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exth := &Exth{Records: tt.records, textEncoding: tt.textEncoding}
			if got := exth.Publisher(); got != tt.want {
				t.Errorf("Publisher() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExthPublicationDate(t *testing.T) {
	tests := []struct {
		name         string
		records      map[uint32][][]byte
		textEncoding TextEncodingType
		want         string
	}{
		{
			name:         "No publication date record",
			records:      map[uint32][][]byte{AUTHOR: {[]byte("Author")}},
			textEncoding: UTF8,
			want:         "",
		},
		{
			name:         "ISO date",
			records:      map[uint32][][]byte{PUBLICATION_DATE: {[]byte("2000-01-01T00:00:00Z")}},
			textEncoding: UTF8,
			want:         "2000-01-01T00:00:00Z",
		},
		{
			name:         "Raw value preserved unparsed",
			records:      map[uint32][][]byte{PUBLICATION_DATE: {[]byte("circa 1999")}},
			textEncoding: UTF8,
			want:         "circa 1999",
		},
		{
			name:         "Whitespace trimmed",
			records:      map[uint32][][]byte{PUBLICATION_DATE: {[]byte("  1999  ")}},
			textEncoding: UTF8,
			want:         "1999",
		},
		{
			name:         "First non-empty wins",
			records:      map[uint32][][]byte{PUBLICATION_DATE: {[]byte(""), []byte("2001"), []byte("2002")}},
			textEncoding: UTF8,
			want:         "2001",
		},
		{
			name:         "CP1252 date",
			records:      map[uint32][][]byte{PUBLICATION_DATE: {cp1252Bytes(t, "1999. május")}},
			textEncoding: CP1252,
			want:         "1999. május",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exth := &Exth{Records: tt.records, textEncoding: tt.textEncoding}
			if got := exth.PublicationDate(); got != tt.want {
				t.Errorf("PublicationDate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExthRecordMap(t *testing.T) {
	data := buildExthDataWithRecords(map[uint32]string{
		100: "value1",
		101: "value2",
		102: "value3",
	})

	exth, err := ReadExth(0, data, UTF8, config.DefaultConfig().MaxExthRecords)
	if err != nil {
		t.Fatalf("ReadExth() error: %v", err)
	}

	tests := []struct {
		recordType uint32
		want       string
	}{
		{100, "value1"},
		{101, "value2"},
		{102, "value3"},
		{999, ""}, // Non-existent record
	}

	for _, tt := range tests {
		vals := exth.Records[tt.recordType]
		var got string
		if len(vals) > 0 {
			got = string(vals[0])
		}
		if got != tt.want {
			t.Errorf("Records[%d] = %q, want %q", tt.recordType, got, tt.want)
		}
	}
}

func TestExthLanguage(t *testing.T) {
	tests := []struct {
		name         string
		records      map[uint32]string
		textEncoding TextEncodingType
		want         string
	}{
		{
			name:         "No LANGUAGE record",
			records:      map[uint32]string{100: "other"},
			textEncoding: UTF8,
			want:         "",
		},
		{
			name:         "LANGUAGE record present",
			records:      map[uint32]string{LANGUAGE: "en"},
			textEncoding: UTF8,
			want:         "en",
		},
		{
			name:         "Language with special characters",
			records:      map[uint32]string{LANGUAGE: "pt-BR"},
			textEncoding: UTF8,
			want:         "pt-BR",
		},
		{
			name:         "Language with CP1252 encoding",
			records:      map[uint32]string{LANGUAGE: "fr"},
			textEncoding: CP1252,
			want:         "fr",
		},
		{
			name:         "Multiple records with LANGUAGE present",
			records:      map[uint32]string{100: "author", LANGUAGE: "hu", 103: "desc"},
			textEncoding: UTF8,
			want:         "hu",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exth := &Exth{
				Identifier:   "EXTH",
				HeaderLength: 12,
				RecordCount:  uint32(len(tt.records)),
				Records:      make(map[uint32][][]byte),
				textEncoding: tt.textEncoding,
			}
			for recType, data := range tt.records {
				exth.Records[recType] = [][]byte{[]byte(data)}
			}

			got := exth.Language()
			if got != tt.want {
				t.Errorf("Language() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExthCoverOffset(t *testing.T) {
	tests := []struct {
		name    string
		records map[uint32][][]byte
		want    uint32
	}{
		{
			name:    "No COVER_OFFSET record",
			records: map[uint32][][]byte{100: {[]byte("other")}},
			want:    0,
		},
		{
			name:    "Record with less than 4 bytes",
			records: map[uint32][][]byte{COVER_OFFSET: {{0x00, 0x01}}},
			want:    0,
		},
		{
			name:    "Valid COVER_OFFSET",
			records: map[uint32][][]byte{COVER_OFFSET: {{0x00, 0x00, 0x00, 0x05}}},
			want:    5,
		},
		{
			name:    "Large offset value",
			records: map[uint32][][]byte{COVER_OFFSET: {{0x00, 0x01, 0x00, 0x00}}},
			want:    65536,
		},
		{
			name:    "Multiple bytes beyond 4 are ignored",
			records: map[uint32][][]byte{COVER_OFFSET: {{0x00, 0x00, 0x00, 0x0A, 0xFF, 0xFF}}},
			want:    10,
		},
		{
			name:    "Empty data slice",
			records: map[uint32][][]byte{COVER_OFFSET: {}},
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exth := &Exth{
				Identifier:   "EXTH",
				HeaderLength: 12,
				RecordCount:  uint32(len(tt.records)),
				Records:      tt.records,
				textEncoding: UTF8,
			}
			got := exth.CoverOffset()
			if got != tt.want {
				t.Errorf("CoverOffset() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestExthThumbnailOffset(t *testing.T) {
	tests := []struct {
		name    string
		records map[uint32][][]byte
		want    uint32
	}{
		{
			name:    "No THUMBNAIL_OFFSET record",
			records: map[uint32][][]byte{100: {[]byte("other")}},
			want:    0,
		},
		{
			name:    "Record with less than 4 bytes",
			records: map[uint32][][]byte{THUMBNAIL_OFFSET: {{0x01}}},
			want:    0,
		},
		{
			name:    "Valid THUMBNAIL_OFFSET",
			records: map[uint32][][]byte{THUMBNAIL_OFFSET: {{0x00, 0x00, 0x00, 0x03}}},
			want:    3,
		},
		{
			name:    "Large offset value",
			records: map[uint32][][]byte{THUMBNAIL_OFFSET: {{0xFF, 0x00, 0x00, 0x00}}},
			want:    4278190080,
		},
		{
			name:    "Extra bytes beyond 4 are ignored",
			records: map[uint32][][]byte{THUMBNAIL_OFFSET: {{0x00, 0x00, 0x00, 0x07, 0xBB}}},
			want:    7,
		},
		{
			name:    "Empty data slice",
			records: map[uint32][][]byte{THUMBNAIL_OFFSET: {}},
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exth := &Exth{
				Identifier:   "EXTH",
				HeaderLength: 12,
				RecordCount:  uint32(len(tt.records)),
				Records:      tt.records,
				textEncoding: UTF8,
			}
			got := exth.ThumbnailOffset()
			if got != tt.want {
				t.Errorf("ThumbnailOffset() = %d, want %d", got, tt.want)
			}
		})
	}
}

// Helper functions

// buildExthData creates a minimal EXTH header with no records
func buildExthData(recordCount uint32) []byte {
	data := make([]byte, 12)
	copy(data[0:4], "EXTH")
	binary.BigEndian.PutUint32(data[4:8], 12) // HeaderLength
	binary.BigEndian.PutUint32(data[8:12], recordCount)
	return data
}

// buildExthDataWithRecords creates EXTH header with records
func buildExthDataWithRecords(records map[uint32]string) []byte {
	// Calculate total size
	headerSize := uint32(12)
	recordsSize := uint32(0)
	for _, value := range records {
		recordsSize += 8 + uint32(len(value)) // 4 bytes type + 4 bytes length + data
	}

	data := make([]byte, 0, headerSize+recordsSize)

	// Header
	header := make([]byte, 12)
	copy(header[0:4], "EXTH")
	binary.BigEndian.PutUint32(header[4:8], headerSize+recordsSize)
	binary.BigEndian.PutUint32(header[8:12], uint32(len(records)))
	data = append(data, header...)

	// Records
	for recordType, value := range records {
		recordData := make([]byte, 8+len(value))
		binary.BigEndian.PutUint32(recordData[0:4], recordType)
		binary.BigEndian.PutUint32(recordData[4:8], uint32(8+len(value)))
		copy(recordData[8:], value)
		data = append(data, recordData...)
	}

	return data
}
