package lrf

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
)

func buildLrf(t *testing.T, version uint16, xmlData []byte) []byte {
	t.Helper()

	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	if _, err := zw.Write(xmlData); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	headerEnd := lrfHeaderSize
	if version <= 800 {
		headerEnd = 0x53
	}
	buf := make([]byte, max(headerEnd, lrfHeaderSize))
	copy(buf, lrfMagic)
	binary.LittleEndian.PutUint16(buf[offVersion:], version)
	binary.LittleEndian.PutUint16(buf[offCompressedInfoSize:], uint16(compressed.Len()+4))
	if version > 800 {
		binary.LittleEndian.PutUint32(buf[offUncompressedInfo:], uint32(len(xmlData)))
	}
	return append(buf[:headerEnd], compressed.Bytes()...)
}

func readBook(t *testing.T, data []byte) (book.Metadata, string) {
	t.Helper()
	r := NewLrfReader(config.DefaultConfig())
	b, err := r.Read(book.NewBytesBlob(data))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	return b.Metadata(), b.Version()
}

func TestReadMetadata(t *testing.T) {
	xmlData := []byte("\xEF\xBB\xBF<Info version=\"1.1\"><BookInfo>" +
		"<Title>  The Title </Title>" +
		"<Author>Ann Lee &amp; Bob &amp;&amp; Co, and Carol with Dan</Author>" +
		"<Category>Fiction, Fantasy; Adventure</Category>" +
		"<Category>Drama</Category>" +
		"<Classification>FIC009000; Epic</Classification>" +
		"<FreeText>Summary</FreeText>" +
		"</BookInfo><DocInfo><Language> hu </Language></DocInfo>" +
		"<Keyword> magic </Keyword><Keyword>dragons</Keyword><Keyword>Drama</Keyword></Info>")

	md, version := readBook(t, buildLrf(t, 1000, xmlData))
	if version != "1000" {
		t.Errorf("Version() = %q, want 1000", version)
	}
	if md.Title != "The Title" {
		t.Errorf("Title = %q, want %q", md.Title, "The Title")
	}
	if got, want := strings.Join(md.Authors, "|"), "Ann Lee|Bob & Co|Carol|Dan"; got != want {
		t.Errorf("Authors = %q, want %q", got, want)
	}
	if md.Description != "Summary" {
		t.Errorf("Description = %q, want %q", md.Description, "Summary")
	}
	if got, want := strings.Join(md.Languages, "|"), "hu"; got != want {
		t.Errorf("Languages = %q, want %q", got, want)
	}
	wantSubjects := []string{"Fiction", "Fantasy", "Adventure", "Drama", "FIC009000", "Epic", "magic", "dragons"}
	if !reflect.DeepEqual(md.Subjects, wantSubjects) {
		t.Errorf("Subjects = %#v, want %#v", md.Subjects, wantSubjects)
	}
}

func TestReadMetadataSubjects(t *testing.T) {
	tests := []struct {
		name    string
		xmlData string
		want    []string
	}{
		{
			name: "all sources in source order",
			xmlData: "<Info><BookInfo><Category>Fantasy</Category>" +
				"<Classification>FIC009000</Classification></BookInfo>" +
				"<DocInfo/><Keyword>magic</Keyword><Keyword>dragons</Keyword></Info>",
			want: []string{"Fantasy", "FIC009000", "magic", "dragons"},
		},
		{
			name: "multiple categories preserve order",
			xmlData: "<Info><BookInfo><Category>Fantasy</Category><Category>Adventure</Category>" +
				"</BookInfo></Info>",
			want: []string{"Fantasy", "Adventure"},
		},
		{
			name:    "category split on commas and semicolons",
			xmlData: "<Info><BookInfo><Category> Fiction, Fantasy; Adventure </Category></BookInfo></Info>",
			want:    []string{"Fiction", "Fantasy", "Adventure"},
		},
		{
			name:    "classification split on commas and semicolons",
			xmlData: "<Info><BookInfo><Classification> A, B; C </Classification></BookInfo></Info>",
			want:    []string{"A", "B", "C"},
		},
		{
			name:    "keywords are kept whole",
			xmlData: "<Info><BookInfo/><DocInfo/><Keyword> science fiction, fantasy </Keyword></Info>",
			want:    []string{"science fiction, fantasy"},
		},
		{
			name: "empty values are dropped",
			xmlData: "<Info><BookInfo><Category> , ; </Category><Classification> </Classification>" +
				"</BookInfo><DocInfo/><Keyword> </Keyword></Info>",
			want: nil,
		},
		{
			name: "duplicates are deduplicated case-insensitively",
			xmlData: "<Info><BookInfo><Category>Fantasy; fantasy, FANTASY</Category></BookInfo>" +
				"<DocInfo/><Keyword>fanTASY</Keyword></Info>",
			want: []string{"Fantasy"},
		},
		{
			name: "label and free text are ignored",
			xmlData: "<Info><BookInfo><Label>label</Label><FreeText>summary</FreeText>" +
				"</BookInfo></Info>",
			want: nil,
		},
		{
			name:    "unknown category is kept",
			xmlData: "<Info><BookInfo><Category>Unknown</Category></BookInfo></Info>",
			want:    []string{"Unknown"},
		},
		{
			name:    "missing values",
			xmlData: "<Info><BookInfo/><DocInfo/></Info>",
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md, _ := readBook(t, buildLrf(t, 1000, []byte(tt.xmlData)))
			if !reflect.DeepEqual(md.Subjects, tt.want) {
				t.Errorf("Subjects = %#v, want %#v", md.Subjects, tt.want)
			}
		})
	}
}

func TestReadMetadataLegacyVersion(t *testing.T) {
	xmlData := []byte("<Info version=\"1.0\"><BookInfo><Title>Legacy</Title><Author>Solo</Author>" +
		"</BookInfo></Info>")

	md, version := readBook(t, buildLrf(t, 999, xmlData))
	if version != "999" {
		t.Errorf("Version() = %q, want 999", version)
	}
	if md.Title != "Legacy" {
		t.Errorf("Title = %q, want %q", md.Title, "Legacy")
	}
	if got, want := strings.Join(md.Authors, "|"), "Solo"; got != want {
		t.Errorf("Authors = %q, want %q", got, want)
	}
}

func TestReadMetadataUTF16(t *testing.T) {
	xmlText := "<?xml version=\"1.0\" encoding=\"utf-16\"?>" +
		"<Info><BookInfo><Title>UTF16 Title</Title></BookInfo></Info>"
	xmlData := encodeUTF16LE(t, xmlText)

	md, _ := readBook(t, buildLrf(t, 1000, xmlData))
	if md.Title != "UTF16 Title" {
		t.Errorf("Title = %q, want %q", md.Title, "UTF16 Title")
	}
}

func TestReadMetadataMissingElements(t *testing.T) {
	md, _ := readBook(t, buildLrf(t, 1000, []byte("<Info><BookInfo/><DocInfo/></Info>")))
	if md.Title != "" || len(md.Authors) != 0 || md.Description != "" || len(md.Languages) != 0 || len(md.Subjects) != 0 {
		t.Errorf("Metadata = %+v, want zero value", md)
	}
}

func TestReadMetadataNoInfoBlock(t *testing.T) {
	buf := make([]byte, lrfHeaderSize)
	copy(buf, lrfMagic)
	binary.LittleEndian.PutUint16(buf[offVersion:], 1000)

	md, _ := readBook(t, buf)
	if md.Title != "" || len(md.Authors) != 0 {
		t.Errorf("Metadata = %+v, want zero value", md)
	}
}

func TestReadMetadataErrors(t *testing.T) {
	t.Run("malformed xml", func(t *testing.T) {
		r := NewLrfReader(config.DefaultConfig())
		_, err := r.Read(book.NewBytesBlob(buildLrf(t, 1000, []byte("<Info><BookInfo>"))))
		if !errors.Is(err, book.ErrCorrupt) {
			t.Errorf("Read() error = %v, want book.ErrCorrupt", err)
		}
	})

	t.Run("bad compressed size", func(t *testing.T) {
		data := buildLrf(t, 1000, []byte("<Info/>"))
		binary.LittleEndian.PutUint16(data[offCompressedInfoSize:], 3)

		r := NewLrfReader(config.DefaultConfig())
		_, err := r.Read(book.NewBytesBlob(data))
		if !errors.Is(err, book.ErrCorrupt) {
			t.Errorf("Read() error = %v, want book.ErrCorrupt", err)
		}
	})

	t.Run("uncompressed size mismatch", func(t *testing.T) {
		data := buildLrf(t, 1000, []byte("<Info/>"))
		binary.LittleEndian.PutUint32(data[offUncompressedInfo:], 42)

		r := NewLrfReader(config.DefaultConfig())
		_, err := r.Read(book.NewBytesBlob(data))
		if !errors.Is(err, book.ErrCorrupt) {
			t.Errorf("Read() error = %v, want book.ErrCorrupt", err)
		}
	})
}

func TestSplitAuthors(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"A", "A"},
		{"A & B", "A|B"},
		{"A && B", "A & B"},
		{"A and B", "A|B"},
		{"A, and B", "A|B"},
		{"A with B", "A|B"},
		{"A, with B", "A|B"},
		{"A AND B", "A|B"},
		{"A & ", "A"},
		{" & B", "B"},
	}
	for _, tt := range tests {
		got := strings.Join(splitAuthors(tt.in), "|")
		if got != tt.want {
			t.Errorf("splitAuthors(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func encodeUTF16LE(t *testing.T, s string) []byte {
	t.Helper()
	buf := []byte{0xFF, 0xFE}
	for _, u := range utf16.Encode([]rune(s)) {
		buf = binary.LittleEndian.AppendUint16(buf, u)
	}
	return buf
}
