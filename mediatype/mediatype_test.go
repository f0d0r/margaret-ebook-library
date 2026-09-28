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
		{"JPG", JPG, "image/jpg"},
		{"PNG", PNG, "image/png"},
		{"GIF", GIF, "image/gif"},
		{"SVG", SVG, "image/svg+xml"},
		{"WEBP", WEBP, "image/webp"},
		{"BMP", BMP, "image/bmp"},
		{"MSBMP", MSBMP, "image/x-ms-bmp"},
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

func TestExtension(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
		ok    bool
	}{
		{"jpeg", "image/jpeg", "jpg", true},
		{"jpeg alias", "image/jpg", "jpg", true},
		{"png", "image/png", "png", true},
		{"gif", "image/gif", "gif", true},
		{"svg", "image/svg+xml", "svg", true},
		{"webp", "image/webp", "webp", true},
		{"bmp", "image/bmp", "bmp", true},
		{"bmp alias", "image/x-ms-bmp", "bmp", true},
		{"octet-stream", "application/octet-stream", "bin", true},
		{"uppercase", "IMAGE/JPEG", "jpg", true},
		{"parameters stripped", "image/png; charset=binary", "png", true},
		{"whitespace", "  image/gif  ", "gif", true},
		{"unknown", "application/pdf", "", false},
		{"empty", "", "", false},
		{"not a mime", "hello", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Extension(tt.input)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("Extension(%q) = (%q, %v), want (%q, %v)", tt.input, got, ok, tt.want, tt.ok)
			}
		})
	}
}
