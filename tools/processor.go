package tools

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// caseFolder implements Unicode full case folding for caseless comparison.
var caseFolder = cases.Fold()

// maxLineEndHyphenGap bounds the whitespace between a line-end hyphen and the
// continuation. Longer gaps are treated as word boundaries, not hyphenation.
const maxLineEndHyphenGap = 1024

// minHyphenatedPrefixLetters is the minimum number of letters a word prefix
// must contain before its trailing hyphen can be treated as line-end
// hyphenation.
const minHyphenatedPrefixLetters = 2

// wordShingler is a streaming word-based shingler that normalizes text
// (NFKC + case folding + punctuation and invisible-character removal, plus
// line-end hyphen joining) and emits k-word shingles via callback. It is the
// shared core for MinHasher and SimHasher.
type wordShingler struct {
	k         int
	leftover  []byte
	cur       []rune
	letters   int
	recent    []string
	pos       int
	filled    int
	total     int
	prefix    []string
	onShingle func(string)
	flushed   bool

	pendingHyphen bool
	sawLineBreak  bool
	lastWasCR     bool
	hyphenGap     int
}

func newWordShingler(k int, cb func(string)) *wordShingler {
	if k <= 0 {
		k = 5
	}
	return &wordShingler{
		k:         k,
		recent:    make([]string, k),
		onShingle: cb,
	}
}

func (w *wordShingler) Write(p []byte) (int, error) {
	if w.flushed {
		// Should not write after flush; caller should treat as error externally.
		// We still process to avoid silent loss, but reset flushed flag.
		w.flushed = false
	}
	// Combine leftover + p.
	buf := p
	if len(w.leftover) > 0 {
		buf = append(append([]byte(nil), w.leftover...), p...)
		w.leftover = w.leftover[:0]
	}
	i := 0
	for i < len(buf) {
		if !utf8.FullRune(buf[i:]) {
			break
		}
		r, size := utf8.DecodeRune(buf[i:])
		i += size
		w.consume(r)
	}
	if i < len(buf) {
		w.leftover = append(w.leftover, buf[i:]...)
	}
	return len(p), nil
}

// consume applies the per-rune normalization rules: format characters are
// dropped, separators and symbols delimit words, everything else is word text.
// An ASCII hyphen directly after a word may start a line-end hyphenation that
// is joined once the next line begins with a non-uppercase letter.
func (w *wordShingler) consume(r rune) {
	if unicode.Is(unicode.Cf, r) {
		return
	}
	if w.pendingHyphen {
		w.consumePending(r)
		return
	}
	w.consumePlain(r)
}

func (w *wordShingler) consumePlain(r rune) {
	if r == '-' && w.canStartLineHyphen() {
		w.pendingHyphen = true
		w.hyphenGap = 0
		return
	}
	switch {
	case unicode.IsSpace(r), unicode.IsPunct(r), unicode.IsSymbol(r), unicode.IsControl(r):
		w.flushWord()
	default:
		w.appendRune(r)
	}
}

// consumePending decides whether a hyphen seen after a word is line-end
// hyphenation. Only "hyphen, optional horizontal space, one line break,
// optional indentation, non-uppercase letter" joins; everything else falls back
// to the plain delimiter behaviour.
func (w *wordShingler) consumePending(r rune) {
	// A LF directly after CR is part of the same line terminator.
	if r == '\n' && w.lastWasCR {
		w.lastWasCR = false
		return
	}
	w.lastWasCR = false

	if isLineTerminator(r) {
		if w.sawLineBreak {
			// Blank line: paragraph boundary, do not join.
			w.abortHyphen()
			w.consumePlain(r)
			return
		}
		w.sawLineBreak = true
		w.lastWasCR = r == '\r'
		if !w.bumpGap() {
			w.abortHyphen()
			w.consumePlain(r)
		}
		return
	}
	if unicode.IsSpace(r) {
		if !w.bumpGap() {
			w.abortHyphen()
			w.consumePlain(r)
		}
		return
	}
	if w.sawLineBreak && isJoinableContinuation(r) {
		w.appendRune(r)
		w.clearHyphen()
		return
	}
	// Mid-line hyphen, uppercase/digit/punctuation continuation, or a
	// non-joinable rune after the line break.
	w.abortHyphen()
	w.consumePlain(r)
}

func (w *wordShingler) appendRune(r rune) {
	w.cur = append(w.cur, r)
	if unicode.IsLetter(r) {
		w.letters++
	}
}

func (w *wordShingler) canStartLineHyphen() bool {
	if w.letters < minHyphenatedPrefixLetters || len(w.cur) == 0 {
		return false
	}
	last := w.cur[len(w.cur)-1]
	return unicode.IsLetter(last) || unicode.IsMark(last)
}

func (w *wordShingler) bumpGap() bool {
	w.hyphenGap++
	return w.hyphenGap <= maxLineEndHyphenGap
}

func (w *wordShingler) clearHyphen() {
	w.pendingHyphen = false
	w.sawLineBreak = false
	w.lastWasCR = false
	w.hyphenGap = 0
}

func (w *wordShingler) abortHyphen() {
	w.flushWord()
	w.clearHyphen()
}

func isLineTerminator(r rune) bool {
	return r == '\n' || r == '\r'
}

// isJoinableContinuation reports whether r can continue a hyphenated word
// across a line break. Lowercase letters and letters of caseless scripts
// qualify; uppercase, titlecase, digits, punctuation and marks do not.
func isJoinableContinuation(r rune) bool {
	return unicode.IsLetter(r) && !unicode.IsUpper(r) && !unicode.IsTitle(r)
}

func (w *wordShingler) flushWord() {
	if len(w.cur) == 0 {
		return
	}
	word := string(w.cur)
	word = norm.NFKC.String(word)
	word = caseFolder.String(word)
	// After normalization word may contain spaces (unlikely for single word),
	// but we treat it as one token as normalized.
	w.total++
	if w.total <= w.k {
		w.prefix = append(w.prefix, word)
	}
	if w.k == 1 {
		w.onShingle(word)
	} else {
		// Maintain circular buffer of last k words.
		w.recent[w.pos] = word
		w.pos = (w.pos + 1) % w.k
		if w.filled < w.k {
			w.filled++
		}
		if w.filled == w.k {
			// Build shingle in chronological order.
			var b strings.Builder
			for j := 0; j < w.k; j++ {
				if j > 0 {
					b.WriteByte(' ')
				}
				idx := (w.pos + j) % w.k
				b.WriteString(w.recent[idx])
			}
			w.onShingle(b.String())
		}
	}
	w.cur = w.cur[:0]
	w.letters = 0
}

// Flush processes any pending bytes/word and emits the final shingle for
// short documents (total < k). It is idempotent.
func (w *wordShingler) Flush() {
	if w.flushed {
		return
	}
	w.flushed = true
	// Decode any leftover bytes.
	if len(w.leftover) > 0 {
		// Leftover may contain an incomplete rune at the end; decode what we can.
		// Invalid sequences are replaced by RuneError which we treat as delimiter if space else part of word.
		buf := w.leftover
		w.leftover = w.leftover[:0]
		for len(buf) > 0 {
			if !utf8.FullRune(buf) {
				// Incomplete trailing bytes: decode lossily to avoid silent loss.
				for _, r := range string(buf) {
					w.consume(r)
				}
				break
			}
			r, size := utf8.DecodeRune(buf)
			buf = buf[size:]
			w.consume(r)
		}
	}
	w.clearHyphen()
	if len(w.cur) > 0 {
		w.flushWord()
	}
	if w.total > 0 && w.total < w.k {
		shingle := strings.Join(w.prefix, " ")
		w.onShingle(shingle)
	}
}
