package mediatype

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"whitespace only", "   ", ""},
		{"lower unchanged", "text/plain", "text/plain"},
		{"uppercase", "TEXT/PLAIN", "text/plain"},
		{"mixed case", "Application/XHTML+XML", "application/xhtml+xml"},
		{"charset stripped", "text/html; charset=utf-8", "text/html"},
		{"charset no space", "text/html;charset=UTF-8", "text/html"},
		{"multiple params", "text/plain; charset=utf-8; boundary=something", "text/plain"},
		{"whitespace around", "  text/html  ; charset=utf-8  ", "text/html"},
		{"tab whitespace", "\tTEXT/HTML\t", "text/html"},
		{"semicolon in whitespace", "text/html ; charset=utf-8", "text/html"},
		{"already normalized with param", "application/xhtml+xml; charset=utf-8", "application/xhtml+xml"},
		{"no param with spaces", "  application/xhtml+xml  ", "application/xhtml+xml"},
		{"mobi html", "Application/X-Mobipocket-HTML; charset=utf-8", "application/x-mobipocket-html"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Normalize(tt.input); got != tt.want {
				t.Fatalf("Normalize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestConstants(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"PlainText", PlainText, "text/plain"},
		{"HTML", HTML, "text/html"},
		{"XHTML", XHTML, "application/xhtml+xml"},
		{"MobiHTML", MobiHTML, "application/x-mobipocket-html"},
		{"JPEG", JPEG, "image/jpeg"},
		{"PNG", PNG, "image/png"},
		{"GIF", GIF, "image/gif"},
		{"SVG", SVG, "image/svg+xml"},
		{"OctetStream", OctetStream, "application/octet-stream"},
		{"CSS", CSS, "text/css"},
		{"NCX", NCX, "application/x-dtbncx+xml"},
		{"OPF", OPF, "application/oebps-package+xml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("constant %s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}
