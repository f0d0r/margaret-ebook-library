package mobi

import (
	"reflect"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
)

func TestExthIdentifiers(t *testing.T) {
	tests := []struct {
		name    string
		records map[uint32][][]byte
		want    []book.Identifier
	}{
		{
			name: "no records",
			want: nil,
		},
		{
			name:    "ISBN record",
			records: map[uint32][][]byte{ISBN: {[]byte("978-0-306-40615-7")}},
			want:    []book.Identifier{{Scheme: "ISBN", Value: "978-0-306-40615-7"}},
		},
		{
			name:    "ISBN urn prefix",
			records: map[uint32][][]byte{ISBN: {[]byte("urn:isbn:9780375704024")}},
			want:    []book.Identifier{{Scheme: "ISBN", Value: "9780375704024"}},
		},
		{
			name:    "multiple ISBN records",
			records: map[uint32][][]byte{ISBN: {[]byte("0306406152"), []byte("9780306406157")}},
			want: []book.Identifier{
				{Scheme: "ISBN", Value: "0306406152"},
				{Scheme: "ISBN", Value: "9780306406157"},
			},
		},
		{
			name:    "calibre source",
			records: map[uint32][][]byte{SOURCE: {[]byte("calibre:06182534-7d14-4ef9-a738-843ff09973d8")}},
			want:    []book.Identifier{{Scheme: "calibre", Value: "06182534-7d14-4ef9-a738-843ff09973d8"}},
		},
		{
			name:    "ISBN source",
			records: map[uint32][][]byte{SOURCE: {[]byte("urn:isbn:9780375704024")}},
			want:    []book.Identifier{{Scheme: "ISBN", Value: "9780375704024"}},
		},
		{
			name:    "arbitrary source is ignored",
			records: map[uint32][][]byte{SOURCE: {[]byte("Egy másik regény")}},
			want:    nil,
		},
		{
			name: "arbitrary source does not hide ASIN",
			records: map[uint32][][]byte{
				SOURCE: {[]byte("Egy másik regény")},
				ASIN:   {[]byte("B00TEST123")},
			},
			want: []book.Identifier{{Scheme: "ASIN", Value: "B00TEST123"}},
		},
		{
			name:    "ASIN record",
			records: map[uint32][][]byte{ASIN: {[]byte("B00TEST123")}},
			want:    []book.Identifier{{Scheme: "ASIN", Value: "B00TEST123"}},
		},
		{
			name:    "multiple ASIN records",
			records: map[uint32][][]byte{ASIN: {[]byte("B00TEST123"), []byte("B00TEST456")}},
			want: []book.Identifier{
				{Scheme: "ASIN", Value: "B00TEST123"},
				{Scheme: "ASIN", Value: "B00TEST456"},
			},
		},
		{
			name:    "prefixed ASIN value keeps its scheme",
			records: map[uint32][][]byte{ASIN: {[]byte("urn:uuid:06182534-7d14-4ef9-a738-843ff09973d8")}},
			want:    []book.Identifier{{Scheme: "UUID", Value: "06182534-7d14-4ef9-a738-843ff09973d8"}},
		},
		{
			name:    "ASIN 5xx is a fallback",
			records: map[uint32][][]byte{ASIN_5XX: {[]byte("B00TEST123")}},
			want:    []book.Identifier{{Scheme: "ASIN", Value: "B00TEST123"}},
		},
		{
			name: "ASIN record wins over ASIN 5xx",
			records: map[uint32][][]byte{
				ASIN:     {[]byte("B00TEST123")},
				ASIN_5XX: {[]byte("B00TEST456")},
			},
			want: []book.Identifier{{Scheme: "ASIN", Value: "B00TEST123"}},
		},
		{
			name: "duplicate ISBN values are collapsed",
			records: map[uint32][][]byte{
				ISBN:   {[]byte("9780375704024")},
				SOURCE: {[]byte("urn:isbn:9780375704024")},
			},
			want: []book.Identifier{{Scheme: "ISBN", Value: "9780375704024"}},
		},
		{
			name: "empty values are skipped",
			records: map[uint32][][]byte{
				ISBN:   {[]byte("   ")},
				SOURCE: {[]byte("calibre:")},
				ASIN:   {[]byte("")},
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exth := &Exth{Records: tt.records, textEncoding: UTF8}
			if got := exth.Identifiers(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Identifiers() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestMobiIdentifiers(t *testing.T) {
	exth := func(records map[uint32][][]byte) *Exth {
		return &Exth{Records: records, textEncoding: UTF8}
	}
	mobi6IDs := []book.Identifier{{Scheme: "ISBN", Value: "9780306406157"}}
	kf8IDs := []book.Identifier{{Scheme: "ASIN", Value: "B00KF8TEST"}}

	tests := []struct {
		name string
		mobi *Mobi
		want []book.Identifier
	}{
		{
			name: "no EXTH",
			mobi: &Mobi{},
			want: nil,
		},
		{
			name: "MOBI6 EXTH",
			mobi: &Mobi{EXTH: exth(map[uint32][][]byte{ISBN: {[]byte("9780306406157")}})},
			want: mobi6IDs,
		},
		{
			name: "KF8 EXTH preferred",
			mobi: &Mobi{
				KF8:  &Mobi{EXTH: exth(map[uint32][][]byte{ASIN: {[]byte("B00KF8TEST")}})},
				EXTH: exth(map[uint32][][]byte{ISBN: {[]byte("9780306406157")}}),
			},
			want: kf8IDs,
		},
		{
			name: "KF8 without identifiers falls back to MOBI6",
			mobi: &Mobi{
				KF8:  &Mobi{EXTH: exth(map[uint32][][]byte{SOURCE: {[]byte("Egy másik regény")}})},
				EXTH: exth(map[uint32][][]byte{ISBN: {[]byte("9780306406157")}}),
			},
			want: mobi6IDs,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mobi.Identifiers(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Identifiers() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestMobiReaderIdentifiers(t *testing.T) {
	text := []byte("<html><body>identifiers</body></html>")
	opts := defaultMobiHeaderOpts()
	opts.textLength = uint32(len(text))
	opts.textRecordCount = 1
	opts.exthRecords = map[uint32][]byte{
		ISBN:   []byte("978-0-306-40615-7"),
		SOURCE: []byte("calibre:06182534-7d14-4ef9-a738-843ff09973d8"),
		ASIN:   []byte("B00TEST123"),
	}

	raw := makePdbBlob(t, "Identifiers", []recordSpec{
		makeMobiHeaderRecord(opts),
		{data: text},
	})
	ebook, err := NewMobiReader(config.DefaultConfig()).Read(book.NewBytesBlob(raw))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	want := []book.Identifier{
		{Scheme: "ISBN", Value: "978-0-306-40615-7"},
		{Scheme: "calibre", Value: "06182534-7d14-4ef9-a738-843ff09973d8"},
		{Scheme: "ASIN", Value: "B00TEST123"},
	}
	if got := ebook.Metadata().Identifiers; !reflect.DeepEqual(got, want) {
		t.Errorf("Identifiers = %+v, want %+v", got, want)
	}
}
