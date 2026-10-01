package lit

import (
	"bytes"
	"testing"
)

func TestEscapeReserved(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"a&b", "a&amp;b"},
		{"&amp;", "&amp;"},
		{"&#65;", "&#65;"},
		{"&#x41;", "&#x41;"},
		{"&foo;", "&foo;"},
		{"& foo", "&amp; foo"},
		{"a<<b", "a&lt;b"},
		{"<<!--x-->", "<!--x-->"},
		{"a>>b", "a&gt;b"},
		{"ab>>", "ab>"},
		{"x&y<<z>>w", "x&amp;y&lt;z&gt;w"},
	} {
		if got := string(escapeReserved([]byte(tt.in))); got != tt.want {
			t.Errorf("escapeReserved(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestResolveHref(t *testing.T) {
	manifest := map[string]string{"c1": "chap.html"}
	if got := resolveHref("\x02c1", manifest, ""); got != "chap.html" {
		t.Errorf("resolve = %q, want chap.html", got)
	}
	if got := resolveHref("\x02missing", manifest, ""); got != "missing" {
		t.Errorf("resolve = %q, want missing", got)
	}
	if got := resolveHref("\x02c1#frag", manifest, ""); got != "chap.html#frag" {
		t.Errorf("resolve = %q, want chap.html#frag", got)
	}
	if got := resolveHref("\x02c1", nil, ""); got != "c1" {
		t.Errorf("resolve = %q, want c1", got)
	}
}

func TestResolveHrefDir(t *testing.T) {
	manifest := map[string]string{"c1": "OEBPS/Text/a.html", "cov": "cover.jpeg"}
	for _, tt := range []struct{ href, dir, want string }{
		{"\x02c1", "OEBPS/Text", "a.html"},
		{"\x02c1", "", "OEBPS/Text/a.html"},
		{"\x02cov", "OEBPS/Text", "../../cover.jpeg"},
		{"\x02cov", "", "cover.jpeg"},
		{"\x02c1#s", "OEBPS/Text", "a.html#s"},
		{"\x02missing", "OEBPS/Text", "../../missing"},
	} {
		if got := resolveHref(tt.href, manifest, tt.dir); got != tt.want {
			t.Errorf("resolveHref(%q, dir %q) = %q, want %q", tt.href, tt.dir, got, tt.want)
		}
	}
}

func TestDecodeUnbinaryTiny(t *testing.T) {
	// <package><metadata><dc:Title>T</dc:Title></metadata></package>
	raw := []byte{
		0x00, 0x01, 0x01, 0x00,
		0x00, 0x01, 0x14, 0x00,
		0x00, 0x01, 0x02, 0x00, 'T', 0x00, 0x02, 0x00,
		0x00, 0x02, 0x00,
		0x00, 0x02, 0x00,
	}
	got, err := decodeUnbinary(raw, &opfTables, nil, nil, "")
	if err != nil {
		t.Fatalf("decodeUnbinary: %v", err)
	}
	want := `<package><metadata><dc:Title>T</dc:Title></metadata></package>`
	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDecodeUnbinaryCorrupt(t *testing.T) {
	for name, raw := range map[string][]byte{
		"unknown tag":    {0x00, 0x01, 0x04, 0x00},
		"length exceeds": {0x00, 0x01, 0x01, 0x06, 0x20},
		"bad utf8":       {0xFF},
	} {
		if _, err := decodeUnbinary(raw, &opfTables, nil, nil, ""); err == nil {
			t.Errorf("%s: expected error, got nil (out=%q)", name, mustDecode(raw))
		}
	}
}

func TestDecodeUnbinaryToleratedClose(t *testing.T) {
	// Trailing closes with nothing open are skipped (MS-era files carry
	// them); the decoded prefix up to that point is kept.
	raw := []byte{
		0x00, 0x01, 0x01, 0x00,
		0x00, 0x02, 0x00,
		0x00, 0x02, 0x00,
	}
	got, err := decodeUnbinary(raw, &opfTables, nil, nil, "")
	if err != nil {
		t.Fatalf("decodeUnbinary: %v", err)
	}
	if string(got) != "<package></package>" {
		t.Errorf("got %q, want <package></package>", got)
	}
}

func mustDecode(raw []byte) string {
	out, _ := decodeUnbinary(raw, &opfTables, nil, nil, "")
	return string(bytes.TrimSpace(out))
}

func TestDecodeUnbinaryAtoms(t *testing.T) {
	// flags OPENING|ATOM, tag 1, end of attrs, close record.
	raw := []byte{0x00, 0x11, 0x01, 0x00, 0x00, 0x02, 0x00}
	atoms := &unbinAtoms{tags: map[int]string{1: "foo"}}
	got, err := decodeUnbinary(raw, &htmlTables, nil, atoms, "")
	if err != nil {
		t.Fatalf("decodeUnbinary: %v", err)
	}
	if string(got) != "<foo></foo>" {
		t.Errorf("got %q, want <foo></foo>", got)
	}
	if _, err := decodeUnbinary(raw, &htmlTables, nil, nil, ""); err == nil {
		t.Errorf("expected error without atoms, got nil")
	}
}
