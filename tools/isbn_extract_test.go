package tools

import (
	"slices"
	"testing"
)

func canonicalStrings(values []ISBN) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		isbn13, ok := value.ToISBN13()
		if !ok {
			continue
		}
		out = append(out, isbn13.String())
	}
	return out
}

func TestExtractISBNs(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want []string
	}{
		{"marker isbn13", "ISBN 978-0-306-40615-7", []string{"9780306406157"}},
		{"marker isbn10", "ISBN 0-306-40615-2", []string{"9780306406157"}},
		{"marker label", "ISBN-13: 978-0-306-40615-7", []string{"9780306406157"}},
		{"no marker", "Published in 2003. 978-0-306-40615-7.", []string{"9780306406157"}},
		{"ocr marker", "1SBN 978-0-306-40615-7", []string{"9780306406157"}},
		{"full width", "ＩＳＢＮ　９７８－０－３０６－４０６１５－７", []string{"9780306406157"}},
		{"arabic indic", "ISBN ٩٧٨٠٣٠٦٤٠٦١٥٧", []string{"9780306406157"}},
		{"persian", "ISBN ۹۷۸۰۳۰۶۴۰۶۱۵۷", []string{"9780306406157"}},
		{"trailing year", "ISBN 978-0-306-40615-7 2024", []string{"9780306406157"}},
		{"no space after marker", "ISBN978-0-306-40615-7", []string{"9780306406157"}},
		{"invalid checksum", "ISBN 978-0-306-40615-8", nil},
		{"duplicate formats", "ISBN 0-306-40615-2 and 978-0-306-40615-7", []string{"9780306406157"}},
		{"multiple", "9780306406157 and 978-0-306-40615-7", []string{"9780306406157"}},
		{"order", "9780375704024 then 9780306406157", []string{"9780375704024", "9780306406157"}},
		{"adjacent bare", "9780306406157 9780375704024", []string{"9780306406157", "9780375704024"}},
		{"adjacent after marker", "ISBN 9780306406157 9780375704024", []string{"9780306406157", "9780375704024"}},
		{"digit prefix", "19780306406157", nil},
		{"digit suffix", "97803064061570", nil},
		{"surrounded by letters", "x9780306406157y", []string{"9780306406157"}},
		{"empty", "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := canonicalStrings(ExtractISBNs(tc.in)); !slices.Equal(got, tc.want) {
				t.Errorf("ExtractISBNs(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
