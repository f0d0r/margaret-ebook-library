package tools

import (
	"slices"
	"testing"
)

func shinglesOf(t *testing.T, text string, k int) []string {
	t.Helper()
	var got []string
	w := newWordShingler(k, func(s string) { got = append(got, s) })
	if _, err := w.Write([]byte(text)); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	w.Flush()
	return got
}

func TestWordShingler_TokenNormalization(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty", "", nil},
		{"punctuation only", "---", nil},
		{"trailing punctuation", "word.", []string{"word"}},
		{"punctuation boundary", "Hello—world", []string{"hello", "world"}},
		{"ellipsis", "wait...what", []string{"wait", "what"}},
		{"symbols", "foo©bar", []string{"foo", "bar"}},
		{"control", "foo\x00bar", []string{"foo", "bar"}},
		{"zero width space", "foo\u200bbar", []string{"foobar"}},
		{"soft hyphen", "Sze\u00adchenyi", []string{"szechenyi"}},
		{"bom", "\ufeffHello", []string{"hello"}},
		{"zwnj", "foo\u200cbar", []string{"foobar"}},
		{"zwj", "foo\u200dbar", []string{"foobar"}},
		{"nbsp", "hello\u00a0world", []string{"hello", "world"}},
		{"tabs and newlines", "a\tb\nc", []string{"a", "b", "c"}},
		{"nfkc fullwidth", "Ｆｏｏ", []string{"foo"}},
		{"case fold sharp s", "Straße", []string{"strasse"}},
		{"case fold capital sharp s", "ẞ", []string{"ss"}},
		{"combining acute", "e\u0301", []string{"é"}},
		{"mixed punctuation and invisible", "a.\u200bB!", []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shinglesOf(t, tt.input, 1); !slices.Equal(got, tt.want) {
				t.Fatalf("shingles = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWordShingler_EquivalentInputs(t *testing.T) {
	pairs := [][2]string{
		{"word. word!", "word word"},
		{"Hello—world", "Hello world"},
		{"foo\u200bbar baz", "foobar baz"},
		{"Sze\u00adchenyi", "Szechenyi"},
		{"Straße", "STRASSE"},
		{"a\u00a0b\tc\nd", "a b c d"},
		{"foo\u200cbar\u200dbaz", "foobarbaz"},
	}
	for _, pair := range pairs {
		a := shinglesOf(t, pair[0], 2)
		b := shinglesOf(t, pair[1], 2)
		if !slices.Equal(a, b) {
			t.Fatalf("%q shingles %q != %q shingles %q", pair[0], a, pair[1], b)
		}
	}
}

func TestWordShingler_StreamingSplit(t *testing.T) {
	text := "A dűne.\u200bLegdöbbenetesebb\u00adtisztelendő—és Straße."
	want := shinglesOf(t, text, 3)

	var got []string
	w := newWordShingler(3, func(s string) { got = append(got, s) })
	for i := 0; i < len(text); i++ {
		if _, err := w.Write([]byte(text[i : i+1])); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}
	w.Flush()
	if !slices.Equal(got, want) {
		t.Fatalf("1-byte shingles = %q, want %q", got, want)
	}
}
