package tools

import (
	"slices"
	"strings"
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

func TestWordShingler_LineEndHyphen(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"crlf", "balatoncsilla-\r\ngon", []string{"balatoncsillagon"}},
		{"lf", "balatoncsilla-\ngon", []string{"balatoncsillagon"}},
		{"cr", "balatoncsilla-\rgon", []string{"balatoncsillagon"}},
		{"spaces around break", "balatoncsilla- \r\n  gon", []string{"balatoncsillagon"}},
		{"uppercase continuation", "word-\nWord", []string{"word", "word"}},
		{"digit continuation", "word-\n123", []string{"word", "123"}},
		{"punctuation continuation", "word-\n,next", []string{"word", "next"}},
		{"mid-line hyphen stays a boundary", "co-operate", []string{"co", "operate"}},
		{"line-end hyphen joins", "co-\noperate", []string{"cooperate"}},
		{"accented lowercase continuation", "ki-\r\nőrölt", []string{"kiőrölt"}},
		{"accented uppercase continuation", "al-\r\nMa", []string{"al", "ma"}},
		{"soft hyphen mid-line", "ba\u00adlaton", []string{"balaton"}},
		{"blank line is a paragraph boundary", "word-\n\nnext", []string{"word", "next"}},
		{"pending hyphen at EOF", "word-", []string{"word"}},
		{"spaced hyphen is not a line-end hyphen", "word - x", []string{"word", "x"}},
		{"short prefix does not join", "A-\nB", []string{"a", "b"}},
		{"mid-line hyphen before break", "word-x\n", []string{"word", "x"}},
		{"dialogue dash continuation", "errefelé-\n- valami", []string{"errefelé", "valami"}},
		{"quote before prefix", "\"öre-\ngördül", []string{"öregördül"}},
		{"caseless continuation", "漢字-\nテスト", []string{"漢字テスト"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shinglesOf(t, tt.input, 1); !slices.Equal(got, tt.want) {
				t.Fatalf("shingles = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWordShingler_LineEndHyphenGapLimit(t *testing.T) {
	text := "word-" + strings.Repeat(" ", maxLineEndHyphenGap+1) + "\nnext"
	want := []string{"word", "next"}
	if got := shinglesOf(t, text, 1); !slices.Equal(got, want) {
		t.Fatalf("shingles = %q, want %q", got, want)
	}
}

func TestWordShingler_LineEndHyphenEquivalence(t *testing.T) {
	pairs := [][2]string{
		{"balatoncsilla-\r\ngon vagyunk", "balatoncsillagon vagyunk"},
		{"a va-\r\ngyunk a ki-\r\nőrölt kertben", "a vagyunk a kiőrölt kertben"},
		{"word-\nword word-\nword", "wordword wordword"},
	}
	for _, pair := range pairs {
		a := shinglesOf(t, pair[0], 2)
		b := shinglesOf(t, pair[1], 2)
		if !slices.Equal(a, b) {
			t.Fatalf("%q shingles %q != %q shingles %q", pair[0], a, pair[1], b)
		}
	}
}

func TestWordShingler_LineEndHyphenStreaming(t *testing.T) {
	text := "balatoncsilla-\r\ngon vagyunk a va-\ngyunk ki-\r\nőrölt kertben word-\nWord word-\n123"
	want := shinglesOf(t, text, 3)

	for _, chunk := range []int{1, 2, 3, 7, 64} {
		var got []string
		w := newWordShingler(3, func(s string) { got = append(got, s) })
		for i := 0; i < len(text); i += chunk {
			end := min(i+chunk, len(text))
			if _, err := w.Write([]byte(text[i:end])); err != nil {
				t.Fatalf("chunk %d Write failed: %v", chunk, err)
			}
		}
		w.Flush()
		if !slices.Equal(got, want) {
			t.Fatalf("chunk %d shingles = %q, want %q", chunk, got, want)
		}
	}
}

func TestIsJoinableContinuation(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
	}{
		{'a', true},
		{'ő', true},
		{'é', true},
		{'漢', true},
		{'か', true},
		{'ك', true},
		{'א', true},
		{'ก', true},
		{'A', false},
		{'Ő', false},
		{'ǅ', false},
		{'1', false},
		{'.', false},
		{'-', false},
		{'¬', false},
		{'\u00ad', false},
	}
	for _, tt := range tests {
		if got := isJoinableContinuation(tt.r); got != tt.want {
			t.Fatalf("isJoinableContinuation(%q) = %v, want %v", tt.r, got, tt.want)
		}
	}
}

func TestHashers_LineEndHyphenEquivalence(t *testing.T) {
	a := "balatoncsilla-\r\ngon"
	b := "balatoncsillagon"

	mhA := NewMinHasher(WithShingleSize(1), WithNumHashes(8))
	mhB := NewMinHasher(WithShingleSize(1), WithNumHashes(8))
	mustWrite(t, mhA, []byte(a))
	mustWrite(t, mhB, []byte(b))
	if !equalSigs(mhA.Signature(), mhB.Signature()) {
		t.Fatalf("MinHash mismatch: %v vs %v", mhA.Signature(), mhB.Signature())
	}

	shA := NewSimHasher(WithSimShingleSize(1))
	shB := NewSimHasher(WithSimShingleSize(1))
	mustWrite(t, shA, []byte(a))
	mustWrite(t, shB, []byte(b))
	if shA.Sum64() != shB.Sum64() {
		t.Fatalf("SimHash mismatch: %#x vs %#x", shA.Sum64(), shB.Sum64())
	}
}
