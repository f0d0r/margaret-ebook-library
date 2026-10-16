package chm

import (
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
)

func readFixture(t *testing.T, fx fixture) book.Book {
	t.Helper()
	r := NewChmReader(chmTestConfig())
	b, err := r.Read(book.NewBytesBlob(buildFixture(fx)))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if b.FileType() != book.CHM {
		t.Fatalf("FileType = %q, want chm", b.FileType())
	}
	if b.Version() != "3" {
		t.Fatalf("Version = %q, want 3", b.Version())
	}
	return b
}

func TestMetadataFromSystem(t *testing.T) {
	html := `<html><head><title>HTML Title</title>` +
		`<meta name="author" content="Jane Doe; John Smith ">` +
		`<meta name="Description" content="A test book.">` +
		`</head><body></body></html>`
	// 0xE9 is é in windows-1250 (Hungarian LCID 0x040E).
	title1250 := []byte("Caf\xe9 Test\x00")
	b := readFixture(t, fixture{files: map[string][]byte{
		"/#SYSTEM": buildSystem(3, []sysEntry{
			{sysTitle, title1250},
			{sysDefaultTopic, []byte("/default.html\x00")},
			{sysInfo, sysInfoEntry(0x040E)},
		}),
		"/default.html": []byte(html),
	}})
	meta := b.Metadata()
	if meta.Title != "Café Test" {
		t.Errorf("Title = %q, want %q", meta.Title, "Café Test")
	}
	if len(meta.Authors) != 2 || meta.Authors[0] != "Jane Doe" || meta.Authors[1] != "John Smith" {
		t.Errorf("Authors = %q, want [Jane Doe John Smith]", meta.Authors)
	}
	if meta.Description != "A test book." {
		t.Errorf("Description = %q, want %q", meta.Description, "A test book.")
	}
	if len(meta.Languages) != 1 || meta.Languages[0] != "hu" {
		t.Errorf("Languages = %q, want [hu]", meta.Languages)
	}
}

func TestMetadataTitleFallbacks(t *testing.T) {
	html := `<html><head><title>
		Spaced   Title
	</title></head><body></body></html>`
	stringsRaw := append([]byte{0}, []byte("Topics Title\x00")...)
	topics := make([]byte, 16)
	putU32(topics[4:], 1) // title at #STRINGS offset 1

	b := readFixture(t, fixture{files: map[string][]byte{
		"/#SYSTEM": buildSystem(3, []sysEntry{
			{sysDefaultTopic, []byte("/default.html\x00")},
			{sysInfo, sysInfoEntry(1033)},
		}),
		"/default.html": []byte(html),
		"/#STRINGS":     stringsRaw,
		"/#TOPICS":      topics,
	}})
	if got := b.Metadata().Title; got != "Spaced Title" {
		t.Errorf("Title = %q, want HTML fallback %q", got, "Spaced Title")
	}

	bare := `<html><body>no head</body></html>`
	b = readFixture(t, fixture{files: map[string][]byte{
		"/#SYSTEM": buildSystem(3, []sysEntry{
			{sysDefaultTopic, []byte("/default.html\x00")},
			{sysInfo, sysInfoEntry(1033)},
		}),
		"/default.html": []byte(bare),
		"/#STRINGS":     stringsRaw,
		"/#TOPICS":      topics,
	}})
	if got := b.Metadata().Title; got != "Topics Title" {
		t.Errorf("Title = %q, want topics fallback %q", got, "Topics Title")
	}
	if got := b.Metadata().Authors; got != nil {
		t.Errorf("Authors = %q, want nil", got)
	}
	if got := b.Metadata().Description; got != "" {
		t.Errorf("Description = %q, want empty", got)
	}
}

func TestMetadataNoSystem(t *testing.T) {
	b := readFixture(t, fixture{files: map[string][]byte{
		"/default.html": []byte(`<html><head><title>T</title></head></html>`),
	}})
	if got := b.Metadata(); got.Title != "" || len(got.Languages) != 0 {
		t.Errorf("Metadata = %+v, want empty", got)
	}
}

func TestMetadataUnknownLCID(t *testing.T) {
	b := readFixture(t, fixture{files: map[string][]byte{
		"/#SYSTEM": buildSystem(3, []sysEntry{
			{sysTitle, []byte("T\x00")},
			{sysInfo, sysInfoEntry(0)},
		}),
	}})
	if got := b.Metadata().Languages; len(got) != 0 {
		t.Errorf("Languages = %q, want empty", got)
	}
}

func TestLcidTables(t *testing.T) {
	if got := lcidLanguage(0x040E); got != "hu" {
		t.Errorf("lcidLanguage(hu) = %q", got)
	}
	if got := lcidLanguage(1033); got != "en" {
		t.Errorf("lcidLanguage(1033) = %q", got)
	}
	if got := lcidLanguage(0x0411); got != "ja" {
		t.Errorf("lcidLanguage(ja) = %q", got)
	}
	if got := lcidLanguage(0); got != "" {
		t.Errorf("lcidLanguage(0) = %q, want empty", got)
	}
	if got := lcidCharset(0x040E); got != "windows-1250" {
		t.Errorf("lcidCharset(hu) = %q", got)
	}
	if got := lcidCharset(0x0411); got != "shift_jis" {
		t.Errorf("lcidCharset(ja) = %q", got)
	}
	if got := lcidCharset(1033); got != "windows-1252" {
		t.Errorf("lcidCharset(1033) = %q", got)
	}
}

func putU32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}
