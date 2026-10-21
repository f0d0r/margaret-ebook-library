package tools

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// ISBNKind identifies whether an ISBN is the 10- or 13-digit variant.
type ISBNKind uint8

const (
	ISBN10 ISBNKind = 10
	ISBN13 ISBNKind = 13
)

// ISBN is a normalized, validated ISBN-10 or ISBN-13.
type ISBN struct {
	digits string
}

// IsValidISBN reports whether s is a valid ISBN-10 or ISBN-13.
//
// It accepts common representations: an optional "urn:isbn:" prefix or
// "ISBN", "ISBN-10", "ISBN-13" label, and whitespace or hyphen separators.
// ISBN-10 values may end in an upper- or lowercase "X"; ISBN-13 values must
// start with 978 or 979.
func IsValidISBN(s string) bool {
	_, ok := ParseISBN(s)
	return ok
}

// ParseISBN normalizes and validates s as an ISBN-10 or ISBN-13.
//
// On success it returns the canonical digits with an uppercase X for
// ISBN-10. It reports false for malformed or checksum-invalid input.
func ParseISBN(s string) (ISBN, bool) {
	digits, ok := normalizeISBN(s)
	if !ok {
		return ISBN{}, false
	}
	switch len(digits) {
	case 10:
		if !validISBN10(digits) {
			return ISBN{}, false
		}
	case 13:
		if !validISBN13(digits) {
			return ISBN{}, false
		}
	default:
		return ISBN{}, false
	}
	return ISBN{digits: digits}, true
}

// String returns the canonical digits of the ISBN.
func (i ISBN) String() string {
	return i.digits
}

// Kind reports whether the ISBN is ISBN-10 or ISBN-13. The zero value
// returns 0.
func (i ISBN) Kind() ISBNKind {
	switch len(i.digits) {
	case 10:
		return ISBN10
	case 13:
		return ISBN13
	default:
		return 0
	}
}

// ToISBN13 returns the ISBN-13 equivalent of i. An ISBN-13 is returned
// unchanged. It reports false if i is not a valid ISBN.
func (i ISBN) ToISBN13() (ISBN, bool) {
	switch i.Kind() {
	case ISBN13:
		return i, true
	case ISBN10:
		body := "978" + i.digits[:9]
		check := isbn13CheckDigit(body)
		return ISBN{digits: body + string(rune('0'+check))}, true
	default:
		return ISBN{}, false
	}
}

// ToISBN10 returns the ISBN-10 equivalent of i. An ISBN-10 is returned
// unchanged. It reports false if i is not a valid ISBN or if i is a
// 979-prefixed ISBN-13, which has no ISBN-10 equivalent.
func (i ISBN) ToISBN10() (ISBN, bool) {
	switch i.Kind() {
	case ISBN10:
		return i, true
	case ISBN13:
		if !strings.HasPrefix(i.digits, "978") {
			return ISBN{}, false
		}
		body := i.digits[3:12]
		check := isbn10CheckDigit(body)
		if check == 10 {
			return ISBN{digits: body + "X"}, true
		}
		return ISBN{digits: body + string(rune('0'+check))}, true
	default:
		return ISBN{}, false
	}
}

func normalizeISBN(s string) (string, bool) {
	s = strings.TrimSpace(s)
	const urnPrefix = "urn:isbn:"
	if len(s) >= len(urnPrefix) && strings.EqualFold(s[:len(urnPrefix)], urnPrefix) {
		s = strings.TrimSpace(s[len(urnPrefix):])
	}
	s = trimISBNLabel(s)

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == 'x' || r == 'X':
			b.WriteByte('X')
		case isISBNSeparator(r):
		default:
			return "", false
		}
	}

	digits := b.String()
	if len(digits) != 10 && len(digits) != 13 {
		return "", false
	}
	if len(digits) == 13 {
		if strings.IndexByte(digits, 'X') >= 0 {
			return "", false
		}
	} else if strings.IndexByte(digits[:9], 'X') >= 0 {
		return "", false
	}
	return digits, true
}

func trimISBNLabel(s string) string {
	const label = "isbn"
	if len(s) < len(label) || !strings.EqualFold(s[:len(label)], label) {
		return s
	}
	rest := strings.TrimLeftFunc(s[len(label):], isISBNSeparator)
	if strings.HasPrefix(rest, "10") || strings.HasPrefix(rest, "13") {
		if after := rest[2:]; isISBNLabelEnd(after) {
			rest = strings.TrimLeftFunc(after, isISBNSeparator)
		}
	}
	rest = strings.TrimPrefix(rest, ":")
	return strings.TrimLeftFunc(rest, isISBNSeparator)
}

func isISBNLabelEnd(s string) bool {
	if s == "" {
		return true
	}
	r, _ := utf8.DecodeRuneInString(s)
	return r == ':' || isISBNSeparator(r)
}

func isISBNSeparator(r rune) bool {
	switch r {
	case '-', '\u2010', '\u2011', '\u2012', '\u2013', '\u2014', '\u2212':
		return true
	default:
		return unicode.IsSpace(r)
	}
}

func validISBN10(digits string) bool {
	sum := 0
	for i := 0; i < len(digits); i++ {
		v := 10
		if digits[i] != 'X' {
			v = int(digits[i] - '0')
		}
		sum += (10 - i) * v
	}
	return sum%11 == 0
}

func validISBN13(digits string) bool {
	if !strings.HasPrefix(digits, "978") && !strings.HasPrefix(digits, "979") {
		return false
	}
	sum := 0
	for i := 0; i < len(digits); i++ {
		v := int(digits[i] - '0')
		if i%2 == 0 {
			sum += v
		} else {
			sum += 3 * v
		}
	}
	return sum%10 == 0
}

func isbn10CheckDigit(body string) int {
	sum := 0
	for i := 0; i < len(body); i++ {
		sum += (10 - i) * int(body[i]-'0')
	}
	return (11 - sum%11) % 11
}

func isbn13CheckDigit(body string) int {
	sum := 0
	for i := 0; i < len(body); i++ {
		v := int(body[i] - '0')
		if i%2 == 0 {
			sum += v
		} else {
			sum += 3 * v
		}
	}
	return (10 - sum%10) % 10
}
