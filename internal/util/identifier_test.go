package util

import "testing"

func TestCanonicalScheme(t *testing.T) {
	tests := []struct {
		scheme string
		want   string
	}{
		{"", ""},
		{"isbn", "ISBN"},
		{"ISBN-13", "ISBN"},
		{"doi", "DOI"},
		{"uuid", "UUID"},
		{"asin", "ASIN"},
		{"issn", "ISSN"},
		{"url", "URL"},
		{"uri", "URL"},
		{"calibre", "calibre"},
		{"Proprietary", "Proprietary"},
	}

	for _, tt := range tests {
		if got := CanonicalScheme(tt.scheme); got != tt.want {
			t.Errorf("CanonicalScheme(%q) = %q, want %q", tt.scheme, got, tt.want)
		}
	}
}

func TestSplitIdentifier(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		explicit   string
		wantScheme string
		wantValue  string
	}{
		{"empty", "", "", "", ""},
		{"trims plain value", "  12345  ", "", "", "12345"},
		{"uuid prefix", "uuid:0618", "", "UUID", "0618"},
		{"urn uuid prefix", "urn:uuid:0618", "", "UUID", "0618"},
		{"uppercase urn prefix", "URN:ISBN:978", "", "ISBN", "978"},
		{"calibre prefix", "calibre:0618", "", "calibre", "0618"},
		{"isbn prefix", "isbn:978", "", "ISBN", "978"},
		{"doi prefix", "doi:10.1000/xyz", "", "DOI", "10.1000/xyz"},
		{"url", "http://example.com/a:b", "", "URL", "http://example.com/a:b"},
		{"https url", "https://example.com", "", "URL", "https://example.com"},
		{"ftp url", "ftp://example.com", "", "URL", "ftp://example.com"},
		{"http uri without slashes", "http:foo", "", "URL", "http:foo"},
		{"mailto", "mailto:user@example.com", "", "mailto", "user@example.com"},
		{"unknown prefix", "ABC:123", "", "ABC", "123"},
		{"digit prefix", "1abc:123", "", "", "1abc:123"},
		{"space in prefix", "foo bar:baz", "", "", "foo bar:baz"},
		{"slash in prefix", "foo/bar:baz", "", "", "foo/bar:baz"},
		{"urn without nid", "urn:", "", "", "urn:"},
		{"urn without rest", "urn:isbn", "", "", "urn:isbn"},
		{"empty urn nid", "urn::foo", "", "", "urn::foo"},
		{"plain identifier", "9780306406157", "", "", "9780306406157"},
		{"explicit isbn urn", "urn:isbn:978", "ISBN", "ISBN", "978"},
		{"explicit isbn prefix", "isbn:978", "ISBN", "ISBN", "978"},
		{"explicit uuid prefix", "uuid:0618", "UUID", "UUID", "0618"},
		{"explicit calibre prefix", "calibre:0618", "calibre", "calibre", "0618"},
		{"explicit unknown prefix", "Proprietary:ABC", "Proprietary", "Proprietary", "ABC"},
		{"explicit mismatched prefix", "calibre:978", "ISBN", "ISBN", "calibre:978"},
		{"explicit url", "http://example.com", "ISBN", "ISBN", "http://example.com"},
		{"explicit isbn no prefix", "978", "ISBN", "ISBN", "978"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme, value := SplitIdentifier(tt.value, tt.explicit)
			if scheme != tt.wantScheme || value != tt.wantValue {
				t.Errorf("SplitIdentifier(%q, %q) = (%q, %q), want (%q, %q)",
					tt.value, tt.explicit, scheme, value, tt.wantScheme, tt.wantValue)
			}
		})
	}
}
