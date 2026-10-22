package ereader

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
)

func buildPDB(t *testing.T, name, ident string, records [][]byte) book.Blob {
	t.Helper()

	header := make([]byte, 78)
	copy(header, name)
	copy(header[60:68], ident)
	binary.BigEndian.PutUint16(header[76:78], uint16(len(records)))

	var buf []byte
	buf = append(buf, header...)
	offset := uint32(78 + 8*len(records))
	for _, rec := range records {
		entry := make([]byte, 8)
		binary.BigEndian.PutUint32(entry[0:4], offset)
		buf = append(buf, entry...)
		offset += uint32(len(rec))
	}
	for _, rec := range records {
		buf = append(buf, rec...)
	}
	return book.NewBytesBlob(buf)
}

func dropbookHeader(compression, hasMetadata, metadataOffset uint16) []byte {
	raw := make([]byte, 132)
	binary.BigEndian.PutUint16(raw[0:2], compression)
	binary.BigEndian.PutUint16(raw[24:26], hasMetadata)
	binary.BigEndian.PutUint16(raw[44:46], metadataOffset)
	return raw
}

func TestReadDropbookMetadata(t *testing.T) {
	r := NewEreaderReader(config.DefaultConfig())
	rec0 := dropbookHeader(10, 1, 1)
	rec1 := []byte("Test Title\x00Test Auth\xe9r\x00\x00publisher\x009780306406157\x00")
	b := buildPDB(t, "Fallback Name", "PNRdPPrs", [][]byte{rec0, rec1})

	bk, err := r.Read(b)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if bk.FileType() != book.EREADER {
		t.Errorf("FileType() = %q, want %q", bk.FileType(), book.EREADER)
	}
	if bk.Version() != "132" {
		t.Errorf("Version() = %q, want %q", bk.Version(), "132")
	}
	m := bk.Metadata()
	if m.Title != "Test Title" {
		t.Errorf("Title = %q, want %q", m.Title, "Test Title")
	}
	if len(m.Authors) != 1 || m.Authors[0] != "Test Auth\xc3\xa9r" {
		t.Errorf("Authors = %q, want [Test Auth\xc3\xa9r]", m.Authors)
	}
	if m.Description != "" || len(m.Languages) != 0 {
		t.Errorf("Description/Languages = %q/%q, want empty", m.Description, m.Languages)
	}
	if isbn, ok := m.Identifier("ISBN"); !ok || isbn != "9780306406157" {
		t.Errorf("Identifier(ISBN) = %q, %v, want %q, true", isbn, ok, "9780306406157")
	}
}

func TestReadDropbookISBNKeptRaw(t *testing.T) {
	r := NewEreaderReader(config.DefaultConfig())
	rec0 := dropbookHeader(2, 1, 1)
	rec1 := []byte("Title\x00Author\x00Copyright\x00Publisher\x000-306-40615-2\x00")
	b := buildPDB(t, "Fallback Name", "PNRdPPrs", [][]byte{rec0, rec1})

	bk, err := r.Read(b)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	want := []book.Identifier{{Scheme: "ISBN", Value: "0-306-40615-2"}}
	if got := bk.Metadata().Identifiers; len(got) != 1 || got[0] != want[0] {
		t.Errorf("Identifiers = %+v, want %+v", got, want)
	}
}

func TestReadDropbookWithoutMetadata(t *testing.T) {
	r := NewEreaderReader(config.DefaultConfig())
	b := buildPDB(t, "PDB Name", "PNRdPPrs", [][]byte{dropbookHeader(10, 0, 0)})

	bk, err := r.Read(b)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	m := bk.Metadata()
	if m.Title != "PDB Name" {
		t.Errorf("Title = %q, want %q", m.Title, "PDB Name")
	}
	if len(m.Authors) != 0 {
		t.Errorf("Authors = %q, want empty", m.Authors)
	}
	if bk.Version() != "132" {
		t.Errorf("Version() = %q, want %q", bk.Version(), "132")
	}
}

func TestReadMakebookFallsBackToPDBName(t *testing.T) {
	r := NewEreaderReader(config.DefaultConfig())
	rec0 := make([]byte, 202)
	binary.BigEndian.PutUint16(rec0[0:2], 4)
	b := buildPDB(t, "Peanut Doc", "PNRdPPrs", [][]byte{rec0})

	bk, err := r.Read(b)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if m := bk.Metadata(); m.Title != "Peanut Doc" {
		t.Errorf("Title = %q, want %q", m.Title, "Peanut Doc")
	}
	if bk.Version() != "202" {
		t.Errorf("Version() = %q, want %q", bk.Version(), "202")
	}
}

func TestReadDRM(t *testing.T) {
	r := NewEreaderReader(config.DefaultConfig())
	b := buildPDB(t, "Locked", "PNRdPPrs", [][]byte{dropbookHeader(260, 0, 0)})

	if _, err := r.Read(b); !errors.Is(err, book.ErrDRM) {
		t.Errorf("Read() error = %v, want ErrDRM", err)
	}
}
