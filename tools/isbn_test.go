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
		{"isbn13 979 prefix", "9798400012341", true},
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
		{"fullwidth digits", "９７８０３０６４０６１５７", true},
		{"arabic indic digits", "٩٧٨٠٣٠٦٤٠٦١٥٧", true},
		{"persian digits", "۹۷۸۰۳۰۶۴۰۶۱۵۷", true},
		{"placeholder known test value", "978-1-234567-89-7", false},
		{"placeholder all same digits", "978-0-000000-00-2", false},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidISBN(tt.input); got != tt.valid {
				t.Fatalf("IsValidISBN(%q) = %v, want %v", tt.input, got, tt.valid)
			}
		})
	}
}

func TestNormalizeISBN(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
		wantKd ISBNKind
	}{
		{"isbn10", "0-306-40615-2", "0306406152", ISBN10},
		{"isbn10 lowercase x", "080442957x", "080442957X", ISBN10},
		{"isbn13 urn", "urn:isbn:978-0-306-40615-7", "9780306406157", ISBN13},
		{"isbn13 979 label", "ISBN-13: 9798400012341", "9798400012341", ISBN13},
		{"arabic indic", "٩٧٨٠٣٠٦٤٠٦١٥٧", "9780306406157", ISBN13},
		{"persian", "۹۷۸۰۳۰۶۴۰۶۱۵۷", "9780306406157", ISBN13},
		{"full width", "９７８０３０６４０６１５７", "9780306406157", ISBN13},
		{"placeholder is still well formed", "978‑1‑234567‑89‑7", "9781234567897", ISBN13},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isbn, ok := NormalizeISBN(tt.input)
			if !ok {
				t.Fatalf("NormalizeISBN(%q) failed", tt.input)
			}
			if got := isbn.String(); got != tt.want {
				t.Fatalf("NormalizeISBN(%q) = %q, want %q", tt.input, got, tt.want)
			}
			if got := isbn.Kind(); got != tt.wantKd {
				t.Fatalf("NormalizeISBN(%q).Kind() = %d, want %d", tt.input, got, tt.wantKd)
			}
		})
	}
}

func TestNormalizeISBNInvalid(t *testing.T) {
	isbn, ok := NormalizeISBN("9780306406158")
	if ok {
		t.Fatal("NormalizeISBN accepted invalid checksum")
	}
	if isbn.String() != "" {
		t.Fatalf("NormalizeISBN returned %q for invalid input", isbn.String())
	}
	if isbn.Kind() != 0 {
		t.Fatalf("NormalizeISBN returned kind %d for invalid input", isbn.Kind())
	}
}

func TestValidateBookISBN(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
		ok    bool
	}{
		{"real isbn13", "978-0-306-40615-7", "9780306406157", true},
		{"real isbn10", "0-306-40615-2", "0306406152", true},
		{"unicode digits", "ISBN ٩٧٨٠٣٠٦٤٠٦١٥٧", "9780306406157", true},
		{"known test value", "978‑1‑234567‑89‑7", "", false},
		{"all same digits", "978-0-000000-00-2", "", false},
		{"invalid checksum", "9780306406158", "", false},
		{"empty", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isbn, ok := ValidateBookISBN(tt.input)
			if got := isbn.String(); got != tt.want || ok != tt.ok {
				t.Fatalf("ValidateBookISBN(%q) = (%q, %v), want (%q, %v)", tt.input, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestPlaceholderKind(t *testing.T) {
	tests := []struct {
		name string
		core string
		want PlaceholderKind
	}{
		{"real", "030640615", NotPlaceholder},
		{"all zero", "000000000", PlaceholderAllSameDigits},
		{"all one", "111111111", PlaceholderAllSameDigits},
		{"all eight", "888888888", PlaceholderAllSameDigits},
		{"ascending test value", "123456789", PlaceholderKnownTestValue},
		{"descending test value", "987654321", PlaceholderKnownTestValue},
		{"leading zero test value", "012345678", PlaceholderKnownTestValue},
		{"sequential rotation", "234567890", PlaceholderSequentialDigits},
		{"wrap sequential rotation", "890123456", PlaceholderSequentialDigits},
		{"descending rotation", "109876543", PlaceholderSequentialDigits},
		{"repeated period 2", "010101010", PlaceholderRepeatedPattern},
		{"repeated period 3", "123123123", PlaceholderKnownTestValue},
		{"repeated blocks", "111222333", PlaceholderKnownTestValue},
		{"long run", "111111234", PlaceholderRepeatedPattern},
		{"low entropy", "111101011", PlaceholderLowEntropy},
		{"low entropy leading zero", "011101010", PlaceholderLowEntropy},
		{"low entropy blocks", "111222121", PlaceholderLowEntropy},
		{"max int32", "214748364", PlaceholderKnownTestValue},
		{"max uint32", "429496729", PlaceholderKnownTestValue},
		{"pi", "314159265", PlaceholderKnownTestValue},
		{"e", "271828182", PlaceholderKnownTestValue},
		{"golden ratio", "161803398", PlaceholderKnownTestValue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isbn13ForCore(tt.core).PlaceholderKind(); got != tt.want {
				t.Fatalf("PlaceholderKind() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPlaceholderKindISBN10(t *testing.T) {
	if got := isbn10ForCore("123456789").PlaceholderKind(); got != PlaceholderKnownTestValue {
		t.Fatalf("PlaceholderKind() = %d, want %d", got, PlaceholderKnownTestValue)
	}
	if got := isbn10ForCore("030640615").PlaceholderKind(); got != NotPlaceholder {
		t.Fatalf("PlaceholderKind() = %d, want %d", got, NotPlaceholder)
	}
}

func TestZeroISBNPlaceholderKind(t *testing.T) {
	var isbn ISBN
	if got := isbn.PlaceholderKind(); got != NotPlaceholder {
		t.Fatalf("zero ISBN.PlaceholderKind() = %d, want %d", got, NotPlaceholder)
	}
}

func isbn13ForCore(core string) ISBN {
	body := "978" + core
	return ISBN{digits: body + string(rune('0'+isbn13CheckDigit(body)))}
}

func isbn10ForCore(core string) ISBN {
	check := isbn10CheckDigit(core)
	if check == 10 {
		return ISBN{digits: core + "X"}
	}
	return ISBN{digits: core + string(rune('0'+check))}
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
		{"9798400012341", "9798400012341"},
	}

	for _, tt := range tests {
		isbn, ok := NormalizeISBN(tt.input)
		if !ok {
			t.Fatalf("NormalizeISBN(%q) failed", tt.input)
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
		{"979 has no isbn10", "9798400012341", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isbn, ok := NormalizeISBN(tt.input)
			if !ok {
				t.Fatalf("NormalizeISBN(%q) failed", tt.input)
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
		ten, ok := NormalizeISBN(input)
		if !ok {
			t.Fatalf("NormalizeISBN(%q) failed", input)
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
		thirteen, ok := NormalizeISBN(input)
		if !ok {
			t.Fatalf("NormalizeISBN(%q) failed", input)
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
