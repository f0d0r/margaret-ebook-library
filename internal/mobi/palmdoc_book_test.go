package mobi

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

const palmDocBookHTMLPayload = `<HTML><HEAD><metadata><dc-metadata>` +
	`<dc:Title>Test Palm Book</dc:Title>` +
	`<dc:Creator>Jane Author</dc:Creator>` +
	`<dc:Language>en</dc:Language>` +
	`</dc-metadata></metadata></HEAD><BODY><P>Hello PalmDOC</P></BODY></HTML>`

const palmDocBookPlainPayload = "Just some plain text for PalmDOC.\nSecond line here."

type palmDocBookOpt struct {
	compression     uint16
	encryption      uint16
	textRecordCount *uint16
	chunkSize       int
}

// buildPalmDocBookFile returns a minimal TEXtREAd PDB holding payload split
// into chunks. Record 0 is the 16-byte PalmDOC header plus the first chunk,
// the rest follow as full records. textRecordCount defaults to total-1 like
// real-world files (one short of the record count including the header).
// Compression uses the real palmDocEncode round-trip helper when set to
// PalmDOC so the compressed path is exercised.
func buildPalmDocBookFile(name string, payload []byte, opt palmDocBookOpt) []byte {
	compression := uint16(CompressionNone)
	if opt.compression != 0 {
		compression = opt.compression
	}
	chunkSize := opt.chunkSize
	if chunkSize <= 0 {
		chunkSize = 64
	}
	var chunks [][]byte
	for len(payload) > 0 {
		n := min(chunkSize, len(payload))
		chunks = append(chunks, payload[:n])
		payload = payload[n:]
	}
	if len(chunks) == 0 {
		chunks = [][]byte{{}}
	}
	total := len(chunks)
	textRecordCount := uint16(total - 1)
	if opt.textRecordCount != nil {
		textRecordCount = *opt.textRecordCount
	}

	encode := func(c []byte) []byte {
		if compression == uint16(CompressionPalmDOC) || compression == uint16(CompressionPalmDOCAlt) {
			return palmDocEncode(c)
		}
		return c
	}

	rec0head := make([]byte, PALM_DOC_HEADER_SIZE)
	binary.BigEndian.PutUint16(rec0head[0:2], compression)
	binary.BigEndian.PutUint16(rec0head[8:10], textRecordCount)
	binary.BigEndian.PutUint16(rec0head[10:12], 4096)
	binary.BigEndian.PutUint16(rec0head[12:14], opt.encryption)

	records := make([][]byte, total)
	records[0] = append(rec0head, encode(chunks[0])...)
	for i := 1; i < total; i++ {
		records[i] = encode(chunks[i])
	}
	var full []byte
	for _, c := range chunks {
		full = append(full, c...)
	}
	binary.BigEndian.PutUint32(records[0][4:8], uint32(len(full)))

	return assemblePdb(name, "TEXt", "REAd", records)
}

func TestReadPalmDocBookHeader(t *testing.T) {
	raw := buildPalmDocBookFile("MyPalm", []byte("hello"), palmDocBookOpt{})
	pdb, err := ReadPdbDb(book.NewBytesBlob(raw), config.DefaultConfig().MaxRecordSize)
	if err != nil {
		t.Fatalf("ReadPdbDb() error: %v", err)
	}
	m, err := readPalmDocHeader(pdb)
	if err != nil {
		t.Fatalf("readPalmDocHeader() error: %v", err)
	}
	if m.Type != MobiTypePalmDoc {
		t.Errorf("Type = %d, want %d", m.Type, MobiTypePalmDoc)
	}
	if m.TextEncoding != CP1252 || m.Codec != "cp1252" {
		t.Errorf("encoding = %d/%q, want cp1252", m.TextEncoding, m.Codec)
	}
	if m.ExtraRecordFlags != 0 {
		t.Errorf("ExtraRecordFlags = %d, want 0", m.ExtraRecordFlags)
	}
	if m.MobiVersion != 1 || m.Version() != "1" {
		t.Errorf("Version = %q, want 1", m.Version())
	}
}

func TestIsPalmDocBookHTML(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"upper html", []byte("<HTML><HEAD>"), true},
		{"lower html with padding", append([]byte{0x0e, 0x0e, 0x0e}, []byte("<html>")...), true},
		{"leading whitespace", []byte("  \n <HEAD><TITLE>x"), true},
		{"bom", append([]byte{0xEF, 0xBB, 0xBF}, []byte("<Html>")...), true},
		{"plain", []byte("Just plain text"), false},
		{"prose mention", []byte("I like html soup"), false},
		{"empty", nil, false},
	}
	for _, tt := range tests {
		if got := isPalmDocHTML(tt.data); got != tt.want {
			t.Errorf("%s: isPalmDocHTML() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestExtractPalmDocBookTextBoundary(t *testing.T) {
	payload := []byte(strings.Repeat("A", 50) + "</BODY></HTML>")
	raw := buildPalmDocBookFile("Bound", payload, palmDocBookOpt{chunkSize: 32})
	pdb, err := ReadPdbDb(book.NewBytesBlob(raw), config.DefaultConfig().MaxRecordSize)
	if err != nil {
		t.Fatalf("ReadPdbDb() error: %v", err)
	}
	m, err := readPalmDocHeader(pdb)
	if err != nil {
		t.Fatalf("readPalmDocHeader() error: %v", err)
	}
	// Simulate the real-world off-by-one: one below the true count.
	m.TextRecordCount--
	got, err := extractPalmDocText(pdb, m, config.DefaultConfig().MaxResourceSize)
	if err != nil {
		t.Fatalf("extractPalmDocText() error: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("got %d bytes, want full %d bytes", len(got), len(payload))
	}
	if !bytes.HasSuffix(got, []byte("</BODY></HTML>")) {
		t.Errorf("missing closing tags: %q", got)
	}
}

func TestPalmDocBookReaderHTML(t *testing.T) {
	raw := buildPalmDocBookFile("HtmlBook", []byte(palmDocBookHTMLPayload), palmDocBookOpt{compression: 2})
	r := NewMobiReader(config.DefaultConfig())
	blob := book.NewBytesBlob(raw)
	if !r.Supports(blob) {
		t.Fatal("Supports() = false for TEXtREAd, want true")
	}
	b, err := r.Read(blob)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if b.FileType() != book.PALMDOC {
		t.Errorf("FileType() = %q, want palmdoc", b.FileType())
	}
	if b.Version() != "1" {
		t.Errorf("Version() = %q, want 1", b.Version())
	}
	if got := b.Metadata().Title; got != "Test Palm Book" {
		t.Errorf("Title = %q, want Test Palm Book", got)
	}
	if got := b.Metadata().Authors; len(got) != 1 || got[0] != "Jane Author" {
		t.Errorf("Authors = %q, want [Jane Author]", got)
	}
	all := b.Resources().All()
	if len(all) != 1 {
		t.Fatalf("len(All) = %d, want 1", len(all))
	}
	if all[0].MediaType != mediatype.HTML {
		t.Errorf("MediaType = %q, want text/html", all[0].MediaType)
	}
	data, err := all[0].Data()
	if err != nil {
		t.Fatalf("Data() error: %v", err)
	}
	if !bytes.Contains(data, []byte("</BODY></HTML>")) && !bytes.Contains(data, []byte("</BODY>")) {
		t.Errorf("content missing body close: %q", data)
	}
	rc, err := all[0].OpenAs(context.Background(), mediatype.PlainText)
	if err != nil {
		t.Fatalf("OpenAs() error: %v", err)
	}
	text, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	if !strings.Contains(string(text), "Hello PalmDOC") {
		t.Errorf("plain text missing content: %q", text)
	}
}

func TestPalmDocBookReaderPlain(t *testing.T) {
	raw := buildPalmDocBookFile("PlainName", []byte(palmDocBookPlainPayload), palmDocBookOpt{})
	r := NewMobiReader(config.DefaultConfig())
	b, err := r.Read(book.NewBytesBlob(raw))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if b.FileType() != book.PALMDOC {
		t.Errorf("FileType() = %q, want palmdoc", b.FileType())
	}
	if got := b.Metadata().Title; got != "PlainName" {
		t.Errorf("Title = %q, want PlainName", got)
	}
	all := b.Resources().All()
	if len(all) != 1 {
		t.Fatalf("len(All) = %d, want 1", len(all))
	}
	if all[0].MediaType != mediatype.PlainText {
		t.Errorf("MediaType = %q, want text/plain", all[0].MediaType)
	}
	data, err := all[0].Data()
	if err != nil {
		t.Fatalf("Data() error: %v", err)
	}
	if string(data) != palmDocBookPlainPayload {
		t.Errorf("Data() = %q, want payload", data)
	}
	// Plain text skips the transformer via the from==to early return.
	rc, err := all[0].OpenAs(context.Background(), mediatype.PlainText)
	if err != nil {
		t.Fatalf("OpenAs() error: %v", err)
	}
	text, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(text) != palmDocBookPlainPayload {
		t.Errorf("OpenAs() = %q, want payload", text)
	}
}

func TestPalmDocBookReaderDRM(t *testing.T) {
	raw := buildPalmDocBookFile("Locked", []byte("secret"), palmDocBookOpt{encryption: 1})
	r := NewMobiReader(config.DefaultConfig())
	_, err := r.Read(book.NewBytesBlob(raw))
	if !errors.Is(err, book.ErrDRM) {
		t.Errorf("Read() err = %v, want ErrDRM", err)
	}
}

func TestPalmDocBookReaderLimit(t *testing.T) {
	raw := buildPalmDocBookFile("Big", []byte(strings.Repeat("B", 1000)), palmDocBookOpt{})
	cfg := config.DefaultConfig()
	cfg.MaxResourceSize = 10
	r := NewMobiReader(cfg)
	b, err := r.Read(book.NewBytesBlob(raw))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if _, err := b.Resources().All()[0].Data(); !errors.Is(err, book.ErrLimitExceeded) {
		t.Errorf("Data() err = %v, want ErrLimitExceeded", err)
	}
}

func TestMobiReaderSupportsBothIdents(t *testing.T) {
	r := NewMobiReader(config.DefaultConfig())
	mk := func(ident string) book.Blob {
		buf := make([]byte, 68)
		copy(buf[60:68], ident)
		return book.NewBytesBlob(buf)
	}
	if !r.Supports(mk("BOOKMOBI")) {
		t.Error("Supports(BOOKMOBI) = false, want true")
	}
	if !r.Supports(mk("TEXtREAd")) {
		t.Error("Supports(TEXtREAd) = false, want true")
	}
	if r.Supports(mk("XXXXXXXX")) {
		t.Error("Supports(unknown) = true, want false")
	}
}

func TestPalmDocBookReaderLeadingPadding(t *testing.T) {
	// aesop11p.prc regression: record 0 text starts with 0x0E padding
	// (plus a NUL) before <HTML>. Title and author must still be found.
	payload := append(bytes.Repeat([]byte{0x0e}, 100), append([]byte{0x00}, []byte(palmDocBookHTMLPayload)...)...)
	raw := buildPalmDocBookFile("PaddedBook", payload, palmDocBookOpt{compression: 2})
	r := NewMobiReader(config.DefaultConfig())
	b, err := r.Read(book.NewBytesBlob(raw))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if got := b.Metadata().Title; got != "Test Palm Book" {
		t.Errorf("Title = %q, want Test Palm Book", got)
	}
	if got := b.Metadata().Authors; len(got) != 1 || got[0] != "Jane Author" {
		t.Errorf("Authors = %q, want [Jane Author]", got)
	}
	if got := b.Resources().All()[0].MediaType; got != mediatype.HTML {
		t.Errorf("MediaType = %q, want text/html", got)
	}
}
