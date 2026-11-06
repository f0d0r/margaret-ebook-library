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

// wordShingler is a streaming word-based shingler that normalizes text
// (NFKC + case folding + punctuation and invisible-character removal) and
// emits k-word shingles via callback. It is the shared core for MinHasher and
// SimHasher.
type wordShingler struct {
	k         int
	leftover  []byte
	cur       []rune
	recent    []string
	pos       int
	filled    int
	total     int
	prefix    []string
	onShingle func(string)
	flushed   bool
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
func (w *wordShingler) consume(r rune) {
	switch {
	case unicode.Is(unicode.Cf, r):
		return
	case unicode.IsSpace(r), unicode.IsPunct(r), unicode.IsSymbol(r), unicode.IsControl(r):
		w.flushWord()
	default:
		w.cur = append(w.cur, r)
	}
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
	if len(w.cur) > 0 {
		w.flushWord()
	}
	if w.total > 0 && w.total < w.k {
		shingle := strings.Join(w.prefix, " ")
		w.onShingle(shingle)
	}
}
