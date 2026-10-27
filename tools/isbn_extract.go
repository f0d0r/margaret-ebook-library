package tools

import (
	"regexp"
	"sort"
	"unicode"
	"unicode/utf8"
)

const (
	isbn13Core = `97[89](?:[\s\p{Pd}]*[0-9]){10}`
	isbn10Core = `(?:[0-9](?:[\s\p{Pd}]*)){9}[0-9Xx]`
	isbnMarker = `(?i)(?:[1Il]\s*[S5]\s*[8B]\s*N)(?:\s*\p{Pd}?\s*(?:10|13))?\s*[:：]?\s*`
)

var (
	isbnMarker13  = regexp.MustCompile(isbnMarker + `(` + isbn13Core + `)`)
	isbnMarker10  = regexp.MustCompile(isbnMarker + `(` + isbn10Core + `)`)
	isbn13Pattern = regexp.MustCompile(`(?i)(` + isbn13Core + `)`)
	isbn10Pattern = regexp.MustCompile(`(?i)(` + isbn10Core + `)`)
)

// ExtractISBNs returns the validated ISBNs found in text in order of first
// appearance, deduplicated by canonical ISBN-13. It accepts markered and bare
// ISBN-10/ISBN-13 forms and normalizes NFKC and Unicode digits before
// matching. Returned values keep the form in which they were found; use
// ISBN.ToISBN13 for canonical values.
func ExtractISBNs(text string) []ISBN {
	if text == "" {
		return nil
	}
	text = normalizeISBNText(text)

	type candidate struct {
		pos       int
		isbn      ISBN
		canonical string
	}
	var candidates []candidate
	addMatches := func(re *regexp.Regexp, marker bool) {
		for _, match := range re.FindAllStringSubmatchIndex(text, -1) {
			start, end := match[2], match[3]
			if !marker && !isbnBoundaryBefore(text, start) {
				continue
			}
			if !isbnBoundaryAfter(text, end) {
				continue
			}
			isbn, ok := ParseISBN(text[start:end])
			if !ok {
				continue
			}
			isbn13, ok := isbn.ToISBN13()
			if !ok {
				continue
			}
			candidates = append(candidates, candidate{
				pos:       start,
				isbn:      isbn,
				canonical: isbn13.String(),
			})
		}
	}
	addMatches(isbnMarker13, true)
	addMatches(isbnMarker10, true)
	addMatches(isbn13Pattern, false)
	addMatches(isbn10Pattern, false)
	if len(candidates) == 0 {
		return nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].pos < candidates[j].pos
	})

	out := make([]ISBN, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, item := range candidates {
		if _, ok := seen[item.canonical]; ok {
			continue
		}
		seen[item.canonical] = struct{}{}
		out = append(out, item.isbn)
	}
	return out
}

func isbnBoundaryBefore(text string, pos int) bool {
	if pos <= 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(text[:pos])
	return !unicode.IsDigit(r)
}

func isbnBoundaryAfter(text string, pos int) bool {
	if pos >= len(text) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(text[pos:])
	return !unicode.IsDigit(r)
}
