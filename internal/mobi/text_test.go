package mobi

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/pkg/errs"
	"github.com/f0d0r/margaret-ebook-library/pkg/model"
)

func TestExtractTextNone(t *testing.T) {
	pdbDb := &PdbDb{PdbRecords: []PdbRecord{
		createMockPdbRecord(make([]byte, 100)),
		createMockPdbRecord([]byte("<html>hello world</html>")),
		createMockPdbRecord([]byte("<p>second</p>#")),
	}}
	mobi := &Mobi{
		Compression:     CompressionNone,
		FirstTextRecord: 1,
		TextRecordCount: 2,
	}
	got, err := extractText(pdbDb, mobi, 1<<20)
	if err != nil {
		t.Fatalf("extractText() error: %v", err)
	}
	want := "<html>hello world</html><p>second</p>"
	if string(got) != want {
		t.Errorf("extractText() = %q, want %q", got, want)
	}
}

func TestExtractTextPalmDoc(t *testing.T) {
	text := "<html>hello world</html>"
	pdbDb := &PdbDb{PdbRecords: []PdbRecord{
		createMockPdbRecord(make([]byte, 100)),
		createMockPdbRecord(palmDocEncode([]byte(text))),
	}}
	mobi := &Mobi{
		Compression:     CompressionPalmDOC,
		FirstTextRecord: 1,
		TextRecordCount: 1,
	}
	got, err := extractText(pdbDb, mobi, 1<<20)
	if err != nil {
		t.Fatalf("extractText() error: %v", err)
	}
	if string(got) != text {
		t.Errorf("extractText() = %q, want %q", got, text)
	}
}

func TestExtractTextHUFF(t *testing.T) {
	pdbDb := &PdbDb{PdbRecords: []PdbRecord{
		createMockPdbRecord(make([]byte, 100)),
		createMockPdbRecord(bytes.Repeat([]byte{0xFF}, 8)),
		createMockPdbRecord(buildTestHuff()),
		createMockPdbRecord(buildTestCdic()),
	}}
	mobi := &Mobi{
		Compression:         CompressionHUFF,
		FirstTextRecord:     1,
		TextRecordCount:     1,
		HuffmanRecordOffset: 2,
		HuffmanRecordCount:  2,
	}
	got, err := extractText(pdbDb, mobi, 1<<20)
	if err != nil {
		t.Fatalf("extractText() error: %v", err)
	}
	want := bytes.Repeat([]byte("Test"), 8)
	if !bytes.Equal(got, want) {
		t.Errorf("extractText() = %q, want %q", got, want)
	}
}

func TestExtractTextTrailingData(t *testing.T) {
	// The text record ends with a backward-encoded trailing entry: 4 bytes of
	// data plus a single size byte. A single-byte size of 5 is encoded as
	// 0x85 (bit 8 set on the only, and therefore highest-order, byte).
	// extraFlags bit 1 set means one varint trailing entry.
	rec := append([]byte("<html>hello</html>"), 0xde, 0xad, 0xbe, 0xef, 0x85)
	pdbDb := &PdbDb{PdbRecords: []PdbRecord{
		createMockPdbRecord(make([]byte, 100)),
		createMockPdbRecord(rec),
	}}
	mobi := &Mobi{
		Compression:      CompressionNone,
		FirstTextRecord:  1,
		TextRecordCount:  1,
		ExtraRecordFlags: 2,
	}
	got, err := extractText(pdbDb, mobi, 1<<20)
	if err != nil {
		t.Fatalf("extractText() error: %v", err)
	}
	if string(got) != "<html>hello</html>" {
		t.Errorf("extractText() = %q, want %q", got, "<html>hello</html>")
	}
}

func TestExtractTextMultibyteTrailingData(t *testing.T) {
	// Bit 0 (multibyte) trailing entry: 1 byte of overlap data plus the count
	// byte. (count & 0x3) + 1 bytes are stripped, i.e. 2 bytes here.
	rec := append([]byte("<html>hello</html>"), 0x00, 0x01)
	pdbDb := &PdbDb{PdbRecords: []PdbRecord{
		createMockPdbRecord(make([]byte, 100)),
		createMockPdbRecord(rec),
	}}
	mobi := &Mobi{
		Compression:      CompressionNone,
		FirstTextRecord:  1,
		TextRecordCount:  1,
		ExtraRecordFlags: 1,
	}
	got, err := extractText(pdbDb, mobi, 1<<20)
	if err != nil {
		t.Fatalf("extractText() error: %v", err)
	}
	if string(got) != "<html>hello</html>" {
		t.Errorf("extractText() = %q, want %q", got, "<html>hello</html>")
	}
}

func TestExtractTextStripsCP1252ControlBytes(t *testing.T) {
	pdbDb := &PdbDb{PdbRecords: []PdbRecord{
		createMockPdbRecord(make([]byte, 100)),
		createMockPdbRecord([]byte("<html>a\x1eb\x02c</html>#")),
	}}
	mobi := &Mobi{
		Compression:     CompressionNone,
		FirstTextRecord: 1,
		TextRecordCount: 1,
		TextEncoding:    CP1252,
	}
	got, err := extractText(pdbDb, mobi, 1<<20)
	if err != nil {
		t.Fatalf("extractText() error: %v", err)
	}
	if string(got) != "<html>abc</html>" {
		t.Errorf("extractText() = %q, want %q", got, "<html>abc</html>")
	}
}

func TestExtractTextRemovesNulBytes(t *testing.T) {
	pdbDb := &PdbDb{PdbRecords: []PdbRecord{
		createMockPdbRecord(make([]byte, 100)),
		createMockPdbRecord([]byte("<html>\x00body</html>")),
	}}
	mobi := &Mobi{
		Compression:     CompressionNone,
		FirstTextRecord: 1,
		TextRecordCount: 1,
	}
	got, err := extractText(pdbDb, mobi, 1<<20)
	if err != nil {
		t.Fatalf("extractText() error: %v", err)
	}
	if string(got) != "<html>body</html>" {
		t.Errorf("extractText() = %q, want %q", got, "<html>body</html>")
	}
}

func TestExtractTextLimitExceeded(t *testing.T) {
	pdbDb := &PdbDb{PdbRecords: []PdbRecord{
		createMockPdbRecord(make([]byte, 100)),
		createMockPdbRecord([]byte("<html>hello world</html>")),
	}}
	mobi := &Mobi{
		Compression:     CompressionNone,
		FirstTextRecord: 1,
		TextRecordCount: 1,
	}
	_, err := extractText(pdbDb, mobi, 5)
	if !errors.Is(err, errs.ErrLimitExceeded) {
		t.Errorf("extractText() error = %v, want ErrLimitExceeded", err)
	}
}

func TestExtractTextUnknownCompression(t *testing.T) {
	pdbDb := &PdbDb{PdbRecords: []PdbRecord{
		createMockPdbRecord(make([]byte, 100)),
	}}
	mobi := &Mobi{
		Compression:     CompressionType(999),
		FirstTextRecord: 1,
		TextRecordCount: 1,
	}
	if _, err := extractText(pdbDb, mobi, 1<<20); err == nil {
		t.Error("extractText() expected error for unknown compression, got nil")
	}
}

func TestMobiReaderContent(t *testing.T) {
	reader := NewMobiReader(model.DefaultConfig())

	t.Run("no text records returns nil", func(t *testing.T) {
		pdbDb := &PdbDb{PdbRecords: []PdbRecord{createMockPdbRecord(make([]byte, 100))}}
		mobi := &Mobi{TextRecordCount: 0}
		if got := reader.content(pdbDb, mobi); got != nil {
			t.Errorf("content() = %v, want nil", got)
		}
	})

	t.Run("builds lazy resource", func(t *testing.T) {
		pdbDb := &PdbDb{PdbRecords: []PdbRecord{
			createMockPdbRecord(make([]byte, 100)),
			createMockPdbRecord([]byte("<html>hello</html>#")),
		}}
		mobi := &Mobi{
			Compression:     CompressionNone,
			FirstTextRecord: 1,
			TextRecordCount: 1,
			TextLength:      uint32(len("<html>hello</html>")),
		}
		resources := reader.content(pdbDb, mobi)
		if len(resources) != 1 {
			t.Fatalf("content() = %d resources, want 1", len(resources))
		}
		res := resources[0]
		if res.Name != "index.html" {
			t.Errorf("Name = %q, want %q", res.Name, "index.html")
		}
		if res.MediaType != "application/x-mobipocket-html" {
			t.Errorf("MediaType = %q, want %q", res.MediaType, "application/x-mobipocket-html")
		}
		if res.Size != int64(len("<html>hello</html>")) {
			t.Errorf("Size = %d, want %d", res.Size, len("<html>hello</html>"))
		}
		rc, err := res.Open()
		if err != nil {
			t.Fatalf("Open() error: %v", err)
		}
		defer func() { _ = rc.Close() }()
		data, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("ReadAll() error: %v", err)
		}
		if string(data) != "<html>hello</html>" {
			t.Errorf("Open() = %q, want %q", data, "<html>hello</html>")
		}
	})
}

func TestSizeofTrailingEntry(t *testing.T) {
	tests := []struct {
		name  string
		data  []byte
		start int
		want  int
	}{
		// Backward-encoded Mobipocket varint: the byte closest to the end of
		// the record holds the least significant 7 bits, and a byte with bit
		// 0x80 set is the highest-order byte, terminating the read.
		{"single byte entry", []byte("abc\x85"), 4, 5},
		{"two byte entry", []byte("\x81\x48"), 2, 200}, // 0x48 | 0x01<<7
		{"three byte entry", []byte("\x84\x22\x11"), 3, 0x11111},
		{"max single byte", []byte("\xff"), 1, 127},
		{"psize zero returns zero", []byte("abc"), 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sizeofTrailingEntry(tt.data, tt.start)
			if got != tt.want {
				t.Errorf("sizeofTrailingEntry() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSizeofTrailingEntries(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		extraFlags uint32
		want       int
	}{
		{"no flags", []byte("hello"), 0, 0},
		{"single varint entry bit 1", []byte("hello world\xde\xad\xbe\xef\x85"), 2, 5},
		{"multibyte entry bit 0", []byte("hello\x05"), 1, 2},
		// Two varint entries, read from the end backwards: entry 1 (bit 1)
		// is "\xcc\x82" (data byte + size 2 encoded as 0x82), entry 2
		// (bit 2) is "\xaa\xbb\x83" (two data bytes + size 3 encoded as 0x83).
		{"two entries bits 1+2", []byte("xxxxx\xaa\xbb\x83\xcc\x82"), 6, 5},
		{"trailing larger than data clamped", []byte("hi\x85"), 2, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sizeofTrailingEntries(tt.data, tt.extraFlags)
			if got != tt.want {
				t.Errorf("sizeofTrailingEntries() = %d, want %d", got, tt.want)
			}
		})
	}
}
