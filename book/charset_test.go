package book

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/f0d0r/margaret-ebook-library/mediatype"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

func mustEncode(t *testing.T, enc interface {
	Bytes([]byte) ([]byte, error)
}, s string) []byte {
	t.Helper()
	b, err := enc.Bytes([]byte(s))
	if err != nil {
		t.Fatalf("encode %q: %v", s, err)
	}
	return b
}

func mustDecode(t *testing.T, dec interface {
	Bytes([]byte) ([]byte, error)
}, b []byte) string {
	t.Helper()
	s, err := dec.Bytes(b)
	if err != nil {
		t.Fatalf("decode %x: %v", b, err)
	}
	return string(s)
}

func openBytes(t *testing.T, data []byte, mediaType, charset string) *Resource {
	t.Helper()
	return &Resource{
		MediaType: mediaType,
		Charset:   charset,
		Open: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(data)), nil
		},
	}
}

func TestResourceEffectiveCharset(t *testing.T) {
	var nilRes *Resource
	if got := nilRes.effectiveCharset(); got != CharsetBinary {
		t.Errorf("nil effectiveCharset() = %q, want binary", got)
	}
	if got := (&Resource{MediaType: mediatype.HTML}).effectiveCharset(); got != CharsetUTF8 {
		t.Errorf("unset text effectiveCharset() = %q, want utf-8", got)
	}
	if got := (&Resource{MediaType: mediatype.JPEG}).effectiveCharset(); got != CharsetBinary {
		t.Errorf("unset image effectiveCharset() = %q, want binary", got)
	}
	if got := (&Resource{MediaType: mediatype.HTML, Charset: "CP1252"}).effectiveCharset(); got != "windows-1252" {
		t.Errorf("aliased effectiveCharset() = %q, want windows-1252", got)
	}
}

func TestNewResourceSet_CharsetBackstop(t *testing.T) {
	text := &Resource{Id: "t", MediaType: mediatype.HTML, Href: "t.html", ResolvedHref: "t.html"}
	img := &Resource{Id: "i", MediaType: mediatype.PNG, Href: "i.png", ResolvedHref: "i.png"}
	explicit := &Resource{Id: "e", MediaType: mediatype.HTML, Href: "e.html", ResolvedHref: "e.html", Charset: "CP1252"}
	rs := NewResourceSet(
		[]*Resource{text, img, explicit},
		[]ReadingOrderItem{{Resource: text, Linear: true}, {Resource: img}},
		img,
	)

	if text.Charset != CharsetUTF8 {
		t.Errorf("text Charset = %q, want utf-8", text.Charset)
	}
	if img.Charset != CharsetBinary {
		t.Errorf("image Charset = %q, want binary", img.Charset)
	}
	if explicit.Charset != "windows-1252" {
		t.Errorf("explicit Charset = %q, want windows-1252", explicit.Charset)
	}
	for _, r := range rs.All() {
		if r.Charset == "" {
			t.Errorf("resource %s left with empty Charset", r.Id)
		}
	}
	if ro := rs.ReadingOrder(); ro[1].Resource.Charset != CharsetBinary {
		t.Errorf("reading order image Charset = %q, want binary", ro[1].Resource.Charset)
	}
	if cover, ok := rs.CoverImage(); !ok || cover.Charset != CharsetBinary {
		t.Errorf("cover Charset = %q, want binary", cover.Charset)
	}
}

func TestOpenAs_DecodesStoredCharset(t *testing.T) {
	ctx := context.Background()
	raw := mustEncode(t, charmap.Windows1252.NewEncoder(), `<p>café Ünïcode</p>`)
	r := openBytes(t, raw, mediatype.HTML, "windows-1252")

	got, err := r.DataAs(ctx, mediatype.PlainText)
	if err != nil {
		t.Fatalf("DataAs: %v", err)
	}
	if string(got) != "café Ünïcode" {
		t.Errorf("got %q, want café Ünïcode", string(got))
	}
}

func TestOpenAs_SniffsDeclaredCharset(t *testing.T) {
	ctx := context.Background()
	raw := mustEncode(t, charmap.Windows1251.NewEncoder(),
		`<html><head><meta charset="windows-1251"></head><body><p>Привет</p></body></html>`)
	// Hand-built resource: no Charset recorded, the declaration must win.
	r := openBytes(t, raw, mediatype.HTML, "")

	got, err := r.DataAs(ctx, mediatype.PlainText)
	if err != nil {
		t.Fatalf("DataAs: %v", err)
	}
	if !strings.Contains(string(got), "Привет") {
		t.Errorf("got %q, want to contain Привет", string(got))
	}
}

func TestOpenAs_UTF8PassesThrough(t *testing.T) {
	ctx := context.Background()
	r := openBytes(t, []byte("<p>árvíztűrő</p>"), mediatype.HTML, CharsetUTF8)
	got, err := r.DataAs(ctx, mediatype.PlainText)
	if err != nil {
		t.Fatalf("DataAs: %v", err)
	}
	if string(got) != "árvíztűrő" {
		t.Errorf("got %q, want árvíztűrő", string(got))
	}
}

// utf16LEBytes encodes s as UTF-16LE, optionally prefixed with a BOM.
func utf16LEBytes(s string, bom bool) []byte {
	var out []byte
	if bom {
		out = append(out, 0xFF, 0xFE)
	}
	for _, u := range utf16.Encode([]rune(s)) {
		out = append(out, byte(u), byte(u>>8))
	}
	return out
}

func TestOpenAs_DecodesUTF16(t *testing.T) {
	ctx := context.Background()
	raw := utf16LEBytes("<p>ő árvíz</p>", true)
	r := openBytes(t, raw, mediatype.HTML, "utf-16le")

	got, err := r.DataAs(ctx, mediatype.PlainText)
	if err != nil {
		t.Fatalf("DataAs: %v", err)
	}
	if string(got) != "ő árvíz" {
		t.Errorf("got %q, want ő árvíz (BOM must be consumed)", string(got))
	}
}

func TestOpenAsWithCharset_EncodesUTF16(t *testing.T) {
	ctx := context.Background()
	r := openBytes(t, []byte("<p>ő</p>"), mediatype.HTML, CharsetUTF8)

	rc, err := r.OpenAsWithCharset(ctx, mediatype.HTML, "utf-16le")
	if err != nil {
		t.Fatalf("OpenAsWithCharset: %v", err)
	}
	got, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.HasPrefix(got, []byte{0xFF, 0xFE}) {
		t.Errorf("missing UTF-16LE BOM: % x", got)
	}
	dec := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder()
	text, err := dec.Bytes(got)
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if string(text) != "<p>ő</p>" {
		t.Errorf("roundtrip = %q, want <p>ő</p>", text)
	}
}

func TestOpenAs_IdentityReturnsRawBytes(t *testing.T) {
	ctx := context.Background()
	raw := mustEncode(t, charmap.Windows1252.NewEncoder(), `<p>café</p>`)
	r := openBytes(t, raw, mediatype.HTML, "windows-1252")

	rc, err := r.OpenAs(ctx, mediatype.HTML)
	if err != nil {
		t.Fatalf("OpenAs: %v", err)
	}
	got, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, raw) {
		t.Errorf("identity OpenAs altered bytes: got %x, want %x", got, raw)
	}
}

func TestOpenAsWithCharset_IdentityUTF8Transcodes(t *testing.T) {
	ctx := context.Background()
	raw := mustEncode(t, charmap.Windows1252.NewEncoder(), `<p>café</p>`)
	r := openBytes(t, raw, mediatype.HTML, "windows-1252")

	got, err := r.DataAsWithCharset(ctx, mediatype.HTML, "utf-8")
	if err != nil {
		t.Fatalf("DataAsWithCharset: %v", err)
	}
	if bytes.Equal(got, raw) {
		t.Fatal("explicit utf-8 request returned raw windows-1252 bytes")
	}
	if !utf8.Valid(got) {
		t.Errorf("output is not valid UTF-8: % x", got)
	}
	if string(got) != `<p>café</p>` {
		t.Errorf("got %q, want <p>café</p>", got)
	}

	// The stored charset itself: raw passthrough, no transcode.
	got, err = r.DataAsWithCharset(ctx, mediatype.HTML, "cp1252")
	if err != nil {
		t.Fatalf("DataAsWithCharset(cp1252): %v", err)
	}
	if !bytes.Equal(got, raw) {
		t.Errorf("stored-charset request altered bytes: got %x, want %x", got, raw)
	}
}

func TestOpenAsWithCharset_EncodesOutput(t *testing.T) {
	ctx := context.Background()
	r := openBytes(t, []byte("<p>ő</p>"), mediatype.HTML, CharsetUTF8)

	rc, err := r.OpenAsWithCharset(ctx, mediatype.HTML, "windows-1250")
	if err != nil {
		t.Fatalf("OpenAsWithCharset: %v", err)
	}
	got, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	want := mustEncode(t, charmap.Windows1250.NewEncoder(), "<p>ő</p>")
	if !bytes.Equal(got, want) {
		t.Errorf("got %x, want windows-1250 %x", got, want)
	}
	if s := mustDecode(t, charmap.Windows1250.NewDecoder(), got); s != "<p>ő</p>" {
		t.Errorf("roundtrip = %q, want <p>ő</p>", s)
	}
}

func TestOpenAsWithCharset_TranscodesBetweenCharmaps(t *testing.T) {
	ctx := context.Background()
	raw := mustEncode(t, charmap.Windows1252.NewEncoder(), `<p>café</p>`)
	r := openBytes(t, raw, mediatype.HTML, "windows-1252")

	rc, err := r.OpenAsWithCharset(ctx, mediatype.HTML, "iso-8859-2")
	if err != nil {
		t.Fatalf("OpenAsWithCharset: %v", err)
	}
	got, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, raw) {
		// é is 0xE9 in both charmaps: an identity passthrough is
		// acceptable here only because the byte value coincides; the
		// decode-back below proves the text survived.
		if s := mustDecode(t, charmap.ISO8859_2.NewDecoder(), got); s != "<p>café</p>" {
			t.Errorf("transcoded output decodes to %q, want <p>café</p>", s)
		}
	}
}

func TestOpenAsWithCharset_UnknownEncodingErrors(t *testing.T) {
	ctx := context.Background()
	r := openBytes(t, []byte("<p>x</p>"), mediatype.HTML, CharsetUTF8)

	_, err := r.OpenAsWithCharset(ctx, mediatype.HTML, "no-such-charset")
	if err == nil {
		t.Fatal("unknown output charset should error")
	}

	rc, err := r.OpenAsWithCharset(ctx, mediatype.HTML, "UTF-8")
	if err != nil {
		t.Fatalf("utf-8 alias should be accepted: %v", err)
	}
	_ = rc.Close()
}

func TestOpenAsWithCharset_BinaryPassthrough(t *testing.T) {
	ctx := context.Background()
	raw := []byte{0x89, 'P', 'N', 'G'}
	r := openBytes(t, raw, mediatype.PNG, CharsetBinary)

	rc, err := r.OpenAsWithCharset(ctx, mediatype.PNG, "windows-1250")
	if err != nil {
		t.Fatalf("OpenAsWithCharset: %v", err)
	}
	got, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, raw) {
		t.Errorf("binary content altered: got %x, want %x", got, raw)
	}
}

func TestResource_OpenAsWithCharset_NoTransformerError(t *testing.T) {
	ctx := context.Background()
	r := openBytes(t, []byte{0x89, 'P', 'N', 'G'}, mediatype.PNG, CharsetBinary)
	_, err := r.OpenAsWithCharset(ctx, mediatype.PlainText, "windows-1250")
	if !errors.Is(err, ErrNoTransformer) {
		t.Fatalf("want ErrNoTransformer, got %v", err)
	}
}

func TestResourceSet_OpenReadingOrderAsWithCharset(t *testing.T) {
	ctx := context.Background()
	raw1 := mustEncode(t, charmap.Windows1252.NewEncoder(), `<p>café</p>`)
	ch1 := openBytes(t, raw1, mediatype.HTML, "windows-1252")
	ch1.Id, ch1.Href, ch1.ResolvedHref = "c1", "c1.html", "c1.html"
	ch2 := openBytes(t, []byte("<p>tea</p>"), mediatype.HTML, CharsetUTF8)
	ch2.Id, ch2.Href, ch2.ResolvedHref = "c2", "c2.html", "c2.html"
	rs := NewResourceSet(
		[]*Resource{ch1, ch2},
		[]ReadingOrderItem{{Resource: ch1, Linear: true}, {Resource: ch2, Linear: true}},
		nil,
	)

	rc, err := rs.OpenReadingOrderAs(ctx, mediatype.PlainText)
	if err != nil {
		t.Fatalf("OpenReadingOrderAs: %v", err)
	}
	got, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != "cafétea" {
		t.Errorf("UTF-8 reading order = %q, want cafétea", string(got))
	}

	rc, err = rs.OpenReadingOrderAsWithCharset(ctx, mediatype.PlainText, "windows-1250")
	if err != nil {
		t.Fatalf("OpenReadingOrderAsWithCharset: %v", err)
	}
	got, err = io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if s := mustDecode(t, charmap.Windows1250.NewDecoder(), got); s != "cafétea" {
		t.Errorf("windows-1250 reading order decodes to %q, want cafétea", s)
	}
}
