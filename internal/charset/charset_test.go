package charset

import (
	"testing"

	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"UTF-8", UTF8},
		{"utf8", UTF8},
		{" us-ascii ", UTF8},
		{"ASCII", UTF8},
		{"cp1252", "windows-1252"},
		{"CP1250", "windows-1250"},
		{"cp1251", "windows-1251"},
		{"Windows-1252", "windows-1252"},
		{"windows-1252", "windows-1252"},
		{"binary", Binary},
		{"UTF-16LE", "utf-16le"},
		{"utf-16", "utf-16"},
		{"shift_jis", "shift_jis"},
		{"cp", "cp"},
		{"cpabc", "cpabc"},
		{"cp866", "cp866"},
		{"cp932", "cp932"},
	}
	for _, tt := range tests {
		if got := Normalize(tt.in); got != tt.want {
			t.Errorf("Normalize(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestDefault(t *testing.T) {
	tests := []struct {
		mime, want string
	}{
		{mediatype.PlainText, UTF8},
		{mediatype.HTML, UTF8},
		{mediatype.XHTML, UTF8},
		{mediatype.CSS, UTF8},
		{mediatype.FB2Body, UTF8},
		{mediatype.LRFText, UTF8},
		{mediatype.SVG, UTF8},
		{"application/xml", UTF8},
		{"application/json", UTF8},
		{"application/smil+xml", UTF8},
		{"application/oebs-page-map+xml", UTF8},
		{"application/geo+json", UTF8},
		{mediatype.JPEG, Binary},
		{mediatype.PNG, Binary},
		{mediatype.OctetStream, Binary},
		{"", Binary},
	}
	for _, tt := range tests {
		if got := Default(tt.mime); got != tt.want {
			t.Errorf("Default(%q) = %q, want %q", tt.mime, got, tt.want)
		}
	}
}
