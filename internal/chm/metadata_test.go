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

func TestMetadataPublisherDateISBN(t *testing.T) {
	wrap := func(head, body string) string {
		return `<html><head>` + head + `</head><body>` + body + `</body></html>`
	}
	tests := []struct {
		name      string
		html      string
		publisher string
		date      string
		isbn      string
	}{
		{
			name: "declared meta",
			html: wrap(`<meta name="publisher" content="Acme Press">`+
				`<meta name="isbn" content="978-0-306-40615-7">`+
				`<meta name="date" content="2020-01-01">`, ""),
			publisher: "Acme Press",
			date:      "2020-01-01",
			isbn:      "9780306406157",
		},
		{
			name: "declared dublin core",
			html: wrap(`<meta name="DC.publisher" content="DC Press">`+
				`<meta name="DC.identifier" content="urn:isbn:9780306406157">`+
				`<meta name="DC.date" content="1999">`, ""),
			publisher: "DC Press",
			date:      "1999",
			isbn:      "9780306406157",
		},
		{
			name:      "span heuristics",
			html:      wrap("", `<span class="imprint">Imprint Press</span><span class="isbn">ISBN 0-306-40615-2</span>`),
			publisher: "Imprint Press",
			isbn:      "0306406152",
		},
		{
			name: "table heuristics",
			html: wrap("", `<table>`+
				`<tr><td>Publisher</td><td>Table Press</td></tr>`+
				`<tr><td>ISBN</td><td>978-0-306-40615-7</td></tr>`+
				`</table>`),
			publisher: "Table Press",
			isbn:      "9780306406157",
		},
		{
			name:      "single cell table",
			html:      wrap("", `<table><tr><td>Publisher: Colon Press</td></tr><tr><td>ISBN: 0-306-40615-2</td></tr></table>`),
			publisher: "Colon Press",
			isbn:      "0306406152",
		},
		{
			name: "declared beats heuristics",
			html: wrap(`<meta name="publisher" content="Meta Press">`+
				`<meta name="isbn" content="978-0-306-40615-7">`,
				`<span class="imprint">Span Press</span><span class="isbn">0-306-40615-2</span>`),
			publisher: "Meta Press",
			isbn:      "9780306406157",
		},
		{
			name:      "invalid isbn ignored",
			html:      wrap(`<meta name="isbn" content="not an isbn">`, ""),
			publisher: "",
			date:      "",
			isbn:      "",
		},
		{
			name: "invalid declared falls back to valid heuristic",
			html: wrap(`<meta name="isbn" content="12345">`,
				`<span class="isbn">978-0-306-40615-7</span>`),
			isbn: "9780306406157",
		},
		{
			name: "isbn surrounded by text",
			html: wrap(`<meta name="isbn" content="ISBN: 978-0-306-40615-7 (pbk.)">`, ""),
			isbn: "9780306406157",
		},
		{
			name: "isbn10 trailing x",
			html: wrap("", `<span class="isbn">0-8044-2957-X</span>`),
			isbn: "080442957X",
		},
		{
			name: "isbn unrelated digits ignored",
			html: wrap("", `<span class="isbn">call 555-1234</span>`),
		},
		{
			name: "generic identifier not digit stripped",
			html: wrap(`<meta name="DC.identifier" content="ID 978-0-306-40615-7 (pbk)">`, ""),
		},
		{
			name:      "publisher label stripped",
			html:      wrap(`<meta name="publisher" content="Publisher: Labelled Press">`, ""),
			publisher: "Labelled Press",
		},
		{
			name: "date heuristic ignored",
			html: wrap("", `<span class="cwdate">2020</span>`+
				`<table><tr><td>Pub Date</td><td>2019</td></tr></table>`),
			date: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := readFixture(t, fixture{files: map[string][]byte{
				"/#SYSTEM": buildSystem(3, []sysEntry{
					{sysDefaultTopic, []byte("/default.html\x00")},
					{sysInfo, sysInfoEntry(1033)},
				}),
				"/default.html": []byte(tt.html),
			}})
			md := b.Metadata()
			if md.Publisher != tt.publisher {
				t.Errorf("Publisher = %q, want %q", md.Publisher, tt.publisher)
			}
			if md.PublicationDate != tt.date {
				t.Errorf("PublicationDate = %q, want %q", md.PublicationDate, tt.date)
			}
			if tt.isbn == "" {
				if len(md.Identifiers) != 0 {
					t.Errorf("Identifiers = %+v, want empty", md.Identifiers)
				}
				return
			}
			if len(md.Identifiers) != 1 || md.Identifiers[0].Scheme != "ISBN" || md.Identifiers[0].Value != tt.isbn {
				t.Errorf("Identifiers = %+v, want ISBN %q", md.Identifiers, tt.isbn)
			}
		})
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
