package tools

import "testing"

func TestIsValidISBN(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"isbn10 plain", "0306406152", true},
		{"isbn10 hyphenated", "0-306-40615-2", true},
		{"isbn10 spaced", "0 306 40615 2", true},
		{"isbn10 check digit X", "080442957X", true},
		{"isbn10 lowercase x", "080442957x", true},
		{"isbn10 X with hyphens", "3-16-148410-X", true},
		{"isbn13 plain", "9780306406157", true},
		{"isbn13 hyphenated", "978-0-306-40615-7", true},
		{"isbn13 spaced", "978 0 306 40615 7", true},
		{"isbn13 979 prefix", "9798400000003", true},
		{"urn prefix", "urn:isbn:9780306406157", true},
		{"uppercase urn prefix", "URN:ISBN:978-0-306-40615-7", true},
		{"isbn label", "ISBN 978-0-306-40615-7", true},
		{"isbn13 label", "ISBN-13: 978-0-306-40615-7", true},
		{"isbn10 label", "isbn-10: 0-306-40615-2", true},
		{"isbn10 label", "isbn-10: 0-306  40615-2", true},
		{"urn with label", "urn:isbn:ISBN-10: 0-306-40615-2", true},
		{"en dash separators", "978–0–306–40615–7", true},
		{"non-breaking hyphen", "0‑306‑40615‑2", true},
		{"surrounding whitespace", " \t9780306406157\n ", true},
		{"bad isbn13 checksum", "9780306406158", false},
		{"bad isbn10 checksum", "0306406153", false},
		{"too short", "978030640615", false},
		{"too long", "97803064061570", false},
		{"invalid character", "978-0-306-40615-A", false},
		{"X in the middle", "0X06406152", false},
		{"X in isbn13", "978030640615X", false},
		{"empty", "", false},
		{"only prefix", "urn:isbn:", false},
		{"non-isbn ean13", "5901234123457", false},
		{"embedded in text", "the ISBN is 9780306406157", false},
		{"unicode digits", "９７８０３０６４０６１５７", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidISBN(tt.input); got != tt.valid {
				t.Fatalf("IsValidISBN(%q) = %v, want %v", tt.input, got, tt.valid)
			}
		})
	}
}

func TestParseISBN(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
		wantKd ISBNKind
	}{
		{"isbn10", "0-306-40615-2", "0306406152", ISBN10},
		{"isbn10 lowercase x", "080442957x", "080442957X", ISBN10},
		{"isbn13 urn", "urn:isbn:978-0-306-40615-7", "9780306406157", ISBN13},
		{"isbn13 979 label", "ISBN-13: 9798400000003", "9798400000003", ISBN13},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isbn, ok := ParseISBN(tt.input)
			if !ok {
				t.Fatalf("ParseISBN(%q) failed", tt.input)
			}
			if got := isbn.String(); got != tt.want {
				t.Fatalf("ParseISBN(%q) = %q, want %q", tt.input, got, tt.want)
			}
			if got := isbn.Kind(); got != tt.wantKd {
				t.Fatalf("ParseISBN(%q).Kind() = %d, want %d", tt.input, got, tt.wantKd)
			}
		})
	}
}

func TestParseISBNInvalid(t *testing.T) {
	isbn, ok := ParseISBN("9780306406158")
	if ok {
		t.Fatal("ParseISBN accepted invalid checksum")
	}
	if isbn.String() != "" {
		t.Fatalf("ParseISBN returned %q for invalid input", isbn.String())
	}
	if isbn.Kind() != 0 {
		t.Fatalf("ParseISBN returned kind %d for invalid input", isbn.Kind())
	}
}

func TestISBNToISBN13(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"0306406152", "9780306406157"},
		{"080442957X", "9780804429573"},
		{"316148410X", "9783161484100"},
		{"9780306406157", "9780306406157"},
		{"9798400000003", "9798400000003"},
	}

	for _, tt := range tests {
		isbn, ok := ParseISBN(tt.input)
		if !ok {
			t.Fatalf("ParseISBN(%q) failed", tt.input)
		}
		got, ok := isbn.ToISBN13()
		if !ok {
			t.Fatalf("%q.ToISBN13() failed", tt.input)
		}
		if got.String() != tt.want {
			t.Fatalf("%q.ToISBN13() = %q, want %q", tt.input, got.String(), tt.want)
		}
		if got.Kind() != ISBN13 {
			t.Fatalf("%q.ToISBN13().Kind() = %d, want %d", tt.input, got.Kind(), ISBN13)
		}
	}
}

func TestISBNToISBN10(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
		ok    bool
	}{
		{"isbn13", "9780306406157", "0306406152", true},
		{"isbn13 with X", "9780804429573", "080442957X", true},
		{"isbn13 with X result", "9783161484100", "316148410X", true},
		{"isbn10 unchanged", "0306406152", "0306406152", true},
		{"979 has no isbn10", "9798400000003", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isbn, ok := ParseISBN(tt.input)
			if !ok {
				t.Fatalf("ParseISBN(%q) failed", tt.input)
			}
			got, ok := isbn.ToISBN10()
			if ok != tt.ok {
				t.Fatalf("%q.ToISBN10() ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if got.String() != tt.want {
				t.Fatalf("%q.ToISBN10() = %q, want %q", tt.input, got.String(), tt.want)
			}
			if ok && got.Kind() != ISBN10 {
				t.Fatalf("%q.ToISBN10().Kind() = %d, want %d", tt.input, got.Kind(), ISBN10)
			}
		})
	}
}

func TestISBNRoundTrip(t *testing.T) {
	for _, input := range []string{"0306406152", "080442957X", "316148410X"} {
		ten, ok := ParseISBN(input)
		if !ok {
			t.Fatalf("ParseISBN(%q) failed", input)
		}
		thirteen, ok := ten.ToISBN13()
		if !ok {
			t.Fatalf("%q.ToISBN13() failed", input)
		}
		back, ok := thirteen.ToISBN10()
		if !ok {
			t.Fatalf("%q.ToISBN10() failed", thirteen)
		}
		if back.String() != ten.String() {
			t.Fatalf("round trip %q = %q", ten.String(), back.String())
		}
	}

	for _, input := range []string{"9780306406157", "9780804429573", "9783161484100"} {
		thirteen, ok := ParseISBN(input)
		if !ok {
			t.Fatalf("ParseISBN(%q) failed", input)
		}
		ten, ok := thirteen.ToISBN10()
		if !ok {
			t.Fatalf("%q.ToISBN10() failed", input)
		}
		back, ok := ten.ToISBN13()
		if !ok {
			t.Fatalf("%q.ToISBN13() failed", ten)
		}
		if back.String() != thirteen.String() {
			t.Fatalf("round trip %q = %q", thirteen.String(), back.String())
		}
	}
}

func TestZeroISBN(t *testing.T) {
	var isbn ISBN
	if isbn.String() != "" {
		t.Fatalf("zero ISBN.String() = %q", isbn.String())
	}
	if isbn.Kind() != 0 {
		t.Fatalf("zero ISBN.Kind() = %d", isbn.Kind())
	}
	if _, ok := isbn.ToISBN13(); ok {
		t.Fatal("zero ISBN.ToISBN13() succeeded")
	}
	if _, ok := isbn.ToISBN10(); ok {
		t.Fatal("zero ISBN.ToISBN10() succeeded")
	}
}
