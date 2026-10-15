package ereader

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

type header132Fields struct {
	compression, nonText, hasMetadata, metadata uint16
	imageFirst, imageCount                      uint16
	footnoteFirst, footnoteCount                uint16
	sidebarFirst, sidebarCount                  uint16
}

func header132(f header132Fields) []byte {
	raw := make([]byte, 132)
	binary.BigEndian.PutUint16(raw[0:2], f.compression)
	binary.BigEndian.PutUint16(raw[12:14], f.nonText)
	binary.BigEndian.PutUint16(raw[20:22], f.imageCount)
	binary.BigEndian.PutUint16(raw[24:26], f.hasMetadata)
	binary.BigEndian.PutUint16(raw[40:42], f.imageFirst)
	binary.BigEndian.PutUint16(raw[44:46], f.metadata)
	binary.BigEndian.PutUint16(raw[28:30], f.footnoteCount)
	binary.BigEndian.PutUint16(raw[48:50], f.footnoteFirst)
	binary.BigEndian.PutUint16(raw[30:32], f.sidebarCount)
	binary.BigEndian.PutUint16(raw[50:52], f.sidebarFirst)
	return raw
}

func header202(version, nonText, imageCount uint16) []byte {
	raw := make([]byte, 202)
	binary.BigEndian.PutUint16(raw[0:2], version)
	binary.BigEndian.PutUint16(raw[8:10], nonText)
	binary.BigEndian.PutUint16(raw[116:118], imageCount)
	return raw
}

func xorA5(data []byte) []byte {
	out := make([]byte, len(data))
	for i, c := range data {
		out[i] = c ^ 0xA5
	}
	return out
}

func zlibCompress(t *testing.T, s string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write([]byte(s)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func pngRecord(name string, payload []byte) []byte {
	raw := []byte{'P', 'N', 'G', ' '}
	nb := make([]byte, 32)
	copy(nb, name)
	raw = append(raw, nb...)
	for len(raw) < 62 {
		raw = append(raw, 0)
	}
	return append(raw, payload...)
}

var pngPayload = append([]byte("\x89PNG\r\n\x1a\n"), []byte("fakepngdata")...)

func TestReadContent202(t *testing.T) {
	r := NewEreaderReader(config.DefaultConfig())
	pml := "Hello \\bworld\\b\nBye"
	b := buildPDB(t, "Doc", "PNRdPPrs", [][]byte{
		header202(4, 2, 0),
		xorA5([]byte(pml)),
	})

	bk, err := r.Read(b)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	rs := bk.Resources()
	if rs.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", rs.Len())
	}
	res := rs.All()[0]
	if res.MediaType != mediatype.EreaderPML {
		t.Errorf("MediaType = %q, want %q", res.MediaType, mediatype.EreaderPML)
	}
	if res.Href != "index.pml" {
		t.Errorf("Href = %q, want index.pml", res.Href)
	}
	data, err := res.Data()
	if err != nil {
		t.Fatalf("Data() error: %v", err)
	}
	if string(data) != pml {
		t.Errorf("Data() = %q, want original PML %q", data, pml)
	}
	text, err := res.DataAs(context.Background(), mediatype.PlainText)
	if err != nil {
		t.Fatalf("DataAs() error: %v", err)
	}
	if string(text) != "Hello world\nBye" {
		t.Errorf("DataAs() = %q, want %q", text, "Hello world\nBye")
	}
	order := rs.ReadingOrder()
	if len(order) != 1 || !order[0].Linear {
		t.Errorf("ReadingOrder = %+v, want single linear item", order)
	}
}

func TestReadContent132Zlib(t *testing.T) {
	r := NewEreaderReader(config.DefaultConfig())
	plain := "Line one\nLine two"
	b := buildPDB(t, "Doc", "PNRdPPrs", [][]byte{
		header132(header132Fields{compression: 10, nonText: 2}),
		zlibCompress(t, plain),
	})

	bk, err := r.Read(b)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	data, err := bk.Resources().All()[0].Data()
	if err != nil {
		t.Fatalf("Data() error: %v", err)
	}
	if string(data) != plain {
		t.Errorf("Data() = %q, want %q", data, plain)
	}
}

func TestReadContent132PalmDoc(t *testing.T) {
	r := NewEreaderReader(config.DefaultConfig())
	b := buildPDB(t, "Doc", "PNRdPPrs", [][]byte{
		header132(header132Fields{compression: 2, nonText: 2}),
		[]byte("Hi \\ithere\\i"),
	})

	bk, err := r.Read(b)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	text, err := bk.Resources().All()[0].DataAs(context.Background(), mediatype.PlainText)
	if err != nil {
		t.Fatalf("DataAs() error: %v", err)
	}
	if string(text) != "Hi there" {
		t.Errorf("DataAs() = %q, want %q", text, "Hi there")
	}
}

func TestReadImagesAndCover(t *testing.T) {
	r := NewEreaderReader(config.DefaultConfig())
	b := buildPDB(t, "Doc", "PNRdPPrs", [][]byte{
		header202(4, 2, 2),
		xorA5([]byte("x")),
		pngRecord("cover.png", pngPayload),
		pngRecord("notes.png", pngPayload),
	})

	bk, err := r.Read(b)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	rs := bk.Resources()
	if rs.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", rs.Len())
	}
	cover, ok := rs.GetByHref("images/cover.png")
	if !ok {
		t.Fatalf("GetByHref(images/cover.png) missing")
	}
	if cover.MediaType != mediatype.PNG {
		t.Errorf("cover MediaType = %q, want %q", cover.MediaType, mediatype.PNG)
	}
	data, err := cover.Data()
	if err != nil {
		t.Fatalf("cover Data() error: %v", err)
	}
	if !bytes.Equal(data, pngPayload) {
		t.Errorf("cover Data() = %x, want payload %x", data, pngPayload)
	}
	cc, ok := rs.CoverImage()
	if !ok || cc != cover {
		t.Errorf("CoverImage() = %v, %v; want the cover.png resource", cc, ok)
	}
	if len(rs.ReadingOrder()) != 1 {
		t.Errorf("ReadingOrder len = %d, want 1 (images excluded)", len(rs.ReadingOrder()))
	}
}

func TestReadContentLimit(t *testing.T) {
	r := NewEreaderReader(config.Config{MaxResourceSize: 4, MaxRecordSize: 1 << 20})
	b := buildPDB(t, "Doc", "PNRdPPrs", [][]byte{
		header202(4, 2, 0),
		xorA5([]byte("way too long for the limit")),
	})

	bk, err := r.Read(b)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if _, err := bk.Resources().All()[0].Data(); !errors.Is(err, book.ErrLimitExceeded) {
		t.Errorf("Data() error = %v, want ErrLimitExceeded", err)
	}
}

func TestReadFootnotesNonlinear(t *testing.T) {
	r := NewEreaderReader(config.DefaultConfig())
	b := buildPDB(t, "Doc", "PNRdPPrs", [][]byte{
		header132(header132Fields{compression: 2, nonText: 2, footnoteFirst: 1, footnoteCount: 2}),
		[]byte("Body"),
		[]byte("Note"),
		[]byte("EXTRA"),
	})

	bk, err := r.Read(b)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	rs := bk.Resources()
	order := rs.ReadingOrder()
	if len(order) != 2 || !order[0].Linear || order[1].Linear {
		t.Errorf("ReadingOrder linear flags = %v, want [true false]", order)
	}
	notes, ok := rs.GetByHref("footnotes.pml")
	if !ok {
		t.Fatalf("GetByHref(footnotes.pml) missing")
	}
	data, err := notes.Data()
	if err != nil {
		t.Fatalf("footnotes Data() error: %v", err)
	}
	if string(data) != "Note" {
		t.Errorf("footnotes Data() = %q, want %q", data, "Note")
	}
}

func TestReadImageCountField(t *testing.T) {
	r := NewEreaderReader(config.DefaultConfig())
	b := buildPDB(t, "Doc", "PNRdPPrs", [][]byte{
		header132(header132Fields{compression: 2, nonText: 2, imageFirst: 2, imageCount: 1, metadata: 5}),
		[]byte("t"),
		pngRecord("a.png", pngPayload),
		pngRecord("decoy.png", pngPayload),
		[]byte("META"),
	})

	bk, err := r.Read(b)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	rs := bk.Resources()
	if rs.Len() != 2 {
		t.Fatalf("Len() = %d, want 2 (content + one image)", rs.Len())
	}
	if _, ok := rs.GetByHref("images/a.png"); !ok {
		t.Error("GetByHref(images/a.png) missing")
	}
	if _, ok := rs.GetByHref("images/decoy.png"); ok {
		t.Error("GetByHref(images/decoy.png) present, want it excluded by the image count")
	}
}

func TestReadMakebookImageCountBoundsScan(t *testing.T) {
	r := NewEreaderReader(config.DefaultConfig())
	b := buildPDB(t, "Doc", "PNRdPPrs", [][]byte{
		header202(4, 2, 1),
		xorA5([]byte("x")),
		pngRecord("a.png", pngPayload),
		pngRecord("late.png", pngPayload),
	})

	bk, err := r.Read(b)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	rs := bk.Resources()
	if rs.Len() != 2 {
		t.Fatalf("Len() = %d, want 2 (content + one image)", rs.Len())
	}
	if _, ok := rs.GetByHref("images/late.png"); ok {
		t.Error("GetByHref(images/late.png) present, want it excluded by the image count")
	}
}

func TestReadMakebookZeroImageCountSkipsScan(t *testing.T) {
	r := NewEreaderReader(config.DefaultConfig())
	b := buildPDB(t, "Doc", "PNRdPPrs", [][]byte{
		header202(4, 2, 0),
		xorA5([]byte("x")),
		pngRecord("stray.png", pngPayload),
	})

	bk, err := r.Read(b)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if rs := bk.Resources(); rs.Len() != 1 {
		t.Errorf("Len() = %d, want 1 (content only)", rs.Len())
	}
}

func TestReadCoverCaseInsensitive(t *testing.T) {
	r := NewEreaderReader(config.DefaultConfig())
	b := buildPDB(t, "Doc", "PNRdPPrs", [][]byte{
		header202(4, 2, 1),
		xorA5([]byte("x")),
		pngRecord("COVER.PNG", pngPayload),
	})

	bk, err := r.Read(b)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if _, ok := bk.Resources().CoverImage(); !ok {
		t.Error("CoverImage() missing, want COVER.PNG detected")
	}
}

func TestReadImageNameCP1252(t *testing.T) {
	r := NewEreaderReader(config.DefaultConfig())
	b := buildPDB(t, "Doc", "PNRdPPrs", [][]byte{
		header202(4, 2, 1),
		xorA5([]byte("x")),
		pngRecord("caf\xe9.png", pngPayload),
	})

	bk, err := r.Read(b)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if _, ok := bk.Resources().GetByHref("images/caf\u00e9.png"); !ok {
		t.Error("GetByHref(images/café.png) missing, want cp1252 name decoded")
	}
}
