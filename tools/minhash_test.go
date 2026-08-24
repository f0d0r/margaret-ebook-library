package tools

import (
	"io"
	"strings"
	"testing"
)

func mustWrite(t *testing.T, w io.Writer, p []byte) {
	t.Helper()
	if _, err := w.Write(p); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
}


func equalSigs(a, b []uint64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestMinHasher_Determinism_SmallVsLargeBuffer(t *testing.T) {
	text := "the quick brown fox jumps over the lazy dog the quick brown fox jumps"
	mh1 := NewMinHasher(WithShingleSize(5), WithNumHashes(16))
	mh2 := NewMinHasher(WithShingleSize(5), WithNumHashes(16))
	for i := 0; i < len(text); i += 3 {
		end := i + 3
		if end > len(text) {
			end = len(text)
		}
		if _, err := mh1.Write([]byte(text[i:end])); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}
	if _, err := mh2.Write([]byte(text)); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	s1 := mh1.Signature()
	s2 := mh2.Signature()
	if !equalSigs(s1, s2) {
		t.Fatalf("small vs large buffer sig mismatch\n s1=%v\n s2=%v", s1, s2)
	}
	// Also test 1-byte streaming
	mh3 := NewMinHasher(WithShingleSize(5), WithNumHashes(16))
	for i := 0; i < len(text); i++ {
		if _, err := mh3.Write([]byte(text[i : i+1])); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}
	if !equalSigs(s1, mh3.Signature()) {
		t.Fatalf("1-byte vs large buffer mismatch")
	}
}

func TestMinHasher_Normalization_WhitespaceCase(t *testing.T) {
	mh1 := NewMinHasher(WithShingleSize(2), WithNumHashes(8))
	mh2 := NewMinHasher(WithShingleSize(2), WithNumHashes(8))
	mustWrite(t, mh1, []byte("Hello   WORLD"))
	mustWrite(t, mh2, []byte("hello world"))
	if !equalSigs(mh1.Signature(), mh2.Signature()) {
		t.Fatalf("whitespace/case normalization failed, Jaccard=%f", Jaccard(mh1.Signature(), mh2.Signature()))
	}
	// also test tabs/newlines
	mh3 := NewMinHasher(WithShingleSize(2), WithNumHashes(8))
	mustWrite(t, mh3, []byte("hello\t\nworld"))
	if !equalSigs(mh2.Signature(), mh3.Signature()) {
		t.Fatalf("tab/newline normalization failed")
	}
}

func TestMinHasher_Normalization_NFKC(t *testing.T) {
	// e + combining acute vs precomposed é
	mh1 := NewMinHasher(WithShingleSize(1), WithNumHashes(8))
	mh2 := NewMinHasher(WithShingleSize(1), WithNumHashes(8))
	mustWrite(t, mh1, []byte("e\xcc\x81"))
	mustWrite(t, mh2, []byte("\xc3\xa9"))
	if !equalSigs(mh1.Signature(), mh2.Signature()) {
		t.Fatalf("NFKC normalization failed, Jaccard=%f", Jaccard(mh1.Signature(), mh2.Signature()))
	}
	// Also test Angstrom sign etc? Keep simple
	mh3 := NewMinHasher(WithShingleSize(1), WithNumHashes(8))
	mh4 := NewMinHasher(WithShingleSize(1), WithNumHashes(8))
	// Fullwidth vs ascii (NFKC should normalize)
	mustWrite(t, mh3, []byte("hello"))
	mustWrite(t, mh4, []byte("hello"))
	if !equalSigs(mh3.Signature(), mh4.Signature()) {
		t.Fatalf("sanity failed")
	}
}

func TestMinHasher_Normalization_NFKC_SplitAcrossWrites(t *testing.T) {
	mh1 := NewMinHasher(WithShingleSize(1), WithNumHashes(8))
	mh2 := NewMinHasher(WithShingleSize(1), WithNumHashes(8))
	// Split e + combining across writes
	mustWrite(t, mh1, []byte("e"))
	mustWrite(t, mh1, []byte("\xcc\x81"))
	mustWrite(t, mh2, []byte("\xc3\xa9"))
	if !equalSigs(mh1.Signature(), mh2.Signature()) {
		t.Fatalf("NFKC split across writes failed")
	}
}

func TestMinHasher_UTF8_Split_MultiByte(t *testing.T) {
	mh1 := NewMinHasher(WithShingleSize(1), WithNumHashes(8))
	mh2 := NewMinHasher(WithShingleSize(1), WithNumHashes(8))
	// "é" is 2 bytes \xc3\xa9, split into 1-byte writes
	mustWrite(t, mh1, []byte("\xc3"))
	mustWrite(t, mh1, []byte("\xa9"))
	mustWrite(t, mh2, []byte("\xc3\xa9"))
	if !equalSigs(mh1.Signature(), mh2.Signature()) {
		t.Fatalf("UTF-8 1-byte split failed")
	}
	// Test 3-byte char € \xe2\x82\xac split
	mh3 := NewMinHasher(WithShingleSize(1), WithNumHashes(8))
	mh4 := NewMinHasher(WithShingleSize(1), WithNumHashes(8))
	mustWrite(t, mh3, []byte("\xe2"))
	mustWrite(t, mh3, []byte("\x82"))
	mustWrite(t, mh3, []byte("\xac"))
	mustWrite(t, mh4, []byte("\xe2\x82\xac"))
	if !equalSigs(mh3.Signature(), mh4.Signature()) {
		t.Fatalf("3-byte UTF-8 split failed")
	}
}

func TestMinHasher_ShortDoc_LessThanK(t *testing.T) {
	mh := NewMinHasher(WithShingleSize(5))
	mustWrite(t, mh, []byte("hello world"))
	sig := mh.Signature()
	if len(sig) != 128 {
		t.Fatalf("sig len = %d, want 128", len(sig))
	}
	if sig[0] == ^uint64(0) {
		t.Fatalf("short doc should have been hashed, got max")
	}
	// Same short doc should be identical
	mh2 := NewMinHasher(WithShingleSize(5))
	mustWrite(t, mh2, []byte("hello world"))
	if Jaccard(sig, mh2.Signature()) != 1.0 {
		t.Fatalf("identical short doc Jaccard !=1")
	}
	// Single word
	mh3 := NewMinHasher(WithShingleSize(5))
	mustWrite(t, mh3, []byte("hello"))
	if mh3.Signature()[0] == ^uint64(0) {
		t.Fatalf("single word should be hashed")
	}
}

func TestMinHasher_Empty(t *testing.T) {
	mh := NewMinHasher()
	sig := mh.Signature()
	if len(sig) != 128 {
		t.Fatalf("empty sig len %d", len(sig))
	}
	if sig[0] != ^uint64(0) {
		t.Fatalf("empty sig should be max, got %x", sig[0])
	}
	mh2 := NewMinHasher()
	if Jaccard(sig, mh2.Signature()) != 1.0 {
		t.Fatalf("empty vs empty Jaccard should be 1")
	}
	mh3 := NewMinHasher()
	mustWrite(t, mh3, []byte("hello world test"))
	if Jaccard(sig, mh3.Signature()) != 0.0 {
		t.Fatalf("empty vs non-empty should be 0, got %f", Jaccard(sig, mh3.Signature()))
	}
	// Method Jaccard
	mh4 := NewMinHasher()
	mustWrite(t, mh4, []byte("hello world test"))
	mh5 := NewMinHasher()
	mustWrite(t, mh5, []byte("hello world test"))
	if mh4.Jaccard(mh5) != 1.0 {
		t.Fatalf("method Jaccard failed")
	}
}

func TestMinHasher_SingleWord_K1(t *testing.T) {
	mh1 := NewMinHasher(WithShingleSize(1), WithNumHashes(8))
	mustWrite(t, mh1, []byte("hello world hello"))
	s1 := mh1.Signature()
	// With k=1, shingles are individual words: hello, world, hello (dedup to 2 unique)
	// Ensure not empty and deterministic
	mh2 := NewMinHasher(WithShingleSize(1), WithNumHashes(8))
	mustWrite(t, mh2, []byte("hello world hello"))
	if !equalSigs(s1, mh2.Signature()) {
		t.Fatalf("k=1 determinism failed")
	}
	// Different order should change Jaccard? For k=1, "hello world" vs "world hello" should be identical set
	mh3 := NewMinHasher(WithShingleSize(1), WithNumHashes(8))
	mustWrite(t, mh3, []byte("world hello"))
	// Sets are {hello,world} both, so Jaccard 1
	if Jaccard(s1, mh3.Signature()) != 1.0 {
		t.Fatalf("k=1 set equality failed, got %f", Jaccard(s1, mh3.Signature()))
	}
}

func TestMinHasher_Configurable_K(t *testing.T) {
	text := "a b c d e f g h i j"
	mh3 := NewMinHasher(WithShingleSize(3), WithNumHashes(16))
	mh5 := NewMinHasher(WithShingleSize(5), WithNumHashes(16))
	mustWrite(t, mh3, []byte(text))
	mustWrite(t, mh5, []byte(text))
	if equalSigs(mh3.Signature(), mh5.Signature()) {
		t.Fatalf("different k should give different sig")
	}
	if Jaccard(mh3.Signature(), mh5.Signature()) == 1.0 {
		t.Fatalf("Jaccard should not be 1 for different k")
	}
}

func TestMinHasher_Configurable_NumHashes_Seed(t *testing.T) {
	text := "hello world this is a test"
	mh8 := NewMinHasher(WithNumHashes(8), WithShingleSize(3))
	mh128 := NewMinHasher(WithNumHashes(128), WithShingleSize(3))
	mustWrite(t, mh8, []byte(text))
	mustWrite(t, mh128, []byte(text))
	if len(mh8.Signature()) != 8 || len(mh128.Signature()) != 128 {
		t.Fatalf("numHashes mismatch")
	}
	// Same config, same text => identical
	mhA := NewMinHasher(WithNumHashes(16), WithShingleSize(3), WithSeed(42))
	mhB := NewMinHasher(WithNumHashes(16), WithShingleSize(3), WithSeed(42))
	mustWrite(t, mhA, []byte(text))
	mustWrite(t, mhB, []byte(text))
	if !equalSigs(mhA.Signature(), mhB.Signature()) {
		t.Fatalf("same seed should give same sig")
	}
	mhC := NewMinHasher(WithNumHashes(16), WithShingleSize(3), WithSeed(99))
	mustWrite(t, mhC, []byte(text))
	if equalSigs(mhA.Signature(), mhC.Signature()) {
		t.Fatalf("different seed should give different sig")
	}
	// Zero defaults
	mhZero := NewMinHasher(WithNumHashes(0), WithShingleSize(0))
	if len(mhZero.Signature()) != 128 {
		t.Fatalf("zero should default to 128")
	}
	// Also via MinHashConfig
	mhCfg := newMinHasherWithConfig(MinHashConfig{NumHashes: 0, ShingleSize: 0, Seed: 0})
	if len(mhCfg.Signature()) != 128 {
		t.Fatalf("config zero should default")
	}
}

func TestMinHasher_Jaccard_Identical_vs_Different(t *testing.T) {
	a := NewMinHasher(WithNumHashes(32), WithShingleSize(3))
	b := NewMinHasher(WithNumHashes(32), WithShingleSize(3))
	c := NewMinHasher(WithNumHashes(32), WithShingleSize(3))
	text := "hello world this is a test of minhash jaccard implementation with some words"
	mustWrite(t, a, []byte(text))
	mustWrite(t, b, []byte(text))
	mustWrite(t, c, []byte("completely different content xyz 123 not overlapping at all vocab"))
	if Jaccard(a.Signature(), b.Signature()) != 1.0 {
		t.Fatalf("identical should be 1, got %f", Jaccard(a.Signature(), b.Signature()))
	}
	if j := Jaccard(a.Signature(), c.Signature()); j > 0.2 {
		t.Fatalf("different should be low, got %f", j)
	}
	// Near duplicate: one word changed
	d := NewMinHasher(WithNumHashes(32), WithShingleSize(3))
	mustWrite(t, d, []byte("hello world this is a test of minhash jaccard implementation with some WORDS"))
	if j := Jaccard(a.Signature(), d.Signature()); j < 0.5 {
		t.Fatalf("near duplicate should be high, got %f", j)
	}
}

func TestMinHasher_Jaccard_LengthMismatch(t *testing.T) {
	if Jaccard(nil, nil) != 1.0 {
		t.Fatalf("nil nil should be 1")
	}
	if Jaccard([]uint64{1, 2}, []uint64{1}) != 0.0 {
		t.Fatalf("len mismatch should be 0")
	}
	if Jaccard([]uint64{}, []uint64{1}) != 0.0 {
		t.Fatalf("empty vs non-empty 0")
	}
	if Jaccard([]uint64{1, 2}, []uint64{1, 2, 3}) != 0.0 {
		t.Fatalf("len mismatch 2 vs 3")
	}
}

func TestMinHasher_WriteAfterFinalized_Error(t *testing.T) {
	mh := NewMinHasher()
	mustWrite(t, mh, []byte("hello world"))
	_ = mh.Signature()
	if _, err := mh.Write([]byte("more")); err == nil {
		t.Fatalf("expected error after finalized")
	}
	// Second Signature should be idempotent
	s1 := mh.Signature()
	s2 := mh.Signature()
	if !equalSigs(s1, s2) {
		t.Fatalf("idempotent Signature failed")
	}
}

func TestMinHasher_Signature_Idempotent(t *testing.T) {
	mh := NewMinHasher(WithShingleSize(2))
	mustWrite(t, mh, []byte("a b c d e"))
	s1 := mh.Signature()
	s2 := mh.Signature()
	s3 := mh.Signature()
	if !equalSigs(s1, s2) || !equalSigs(s2, s3) {
		t.Fatalf("Signature not idempotent")
	}
}

func TestMinHasher_ExactJaccard_vs_Estimate(t *testing.T) {
	// Build exact sets with same normalization/shingling logic as processor
	// Use simple helper that mimics processor's normalization (NFKC+lower) via same fn?
	// For this test, use the public MinHasher path vs a naive exact Set Jaccard on same input.
	textA := "the quick brown fox jumps over the lazy dog"
	textB := "the quick brown fox leaps over the lazy dog" // one word diff
	k := 3
	mhA := NewMinHasher(WithShingleSize(k), WithNumHashes(256))
	mhB := NewMinHasher(WithShingleSize(k), WithNumHashes(256))
	mustWrite(t, mhA, []byte(textA))
	mustWrite(t, mhB, []byte(textB))
	est := Jaccard(mhA.Signature(), mhB.Signature())

	// Exact: build shingles via naive strings.Fields(lower) — our processor also lowercases
	// For this ascii test, naive matches processor exactly
	wordsA := strings.Fields(strings.ToLower(textA))
	wordsB := strings.Fields(strings.ToLower(textB))
	setA := makeShingles(wordsA, k)
	setB := makeShingles(wordsB, k)
	exact := exactJaccard(setA, setB)
	diff := est - exact
	if diff < -0.15 || diff > 0.15 {
		t.Fatalf("estimate %.3f vs exact %.3f diff %.3f too large", est, exact, diff)
	}
}

func makeShingles(words []string, k int) map[string]struct{} {
	set := make(map[string]struct{})
	if len(words) == 0 {
		return set
	}
	if len(words) < k {
		set[strings.Join(words, " ")] = struct{}{}
		return set
	}
	for i := 0; i <= len(words)-k; i++ {
		set[strings.Join(words[i:i+k], " ")] = struct{}{}
	}
	return set
}

func exactJaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	inter := 0
	for k := range a {
		if _, ok := b[k]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 1
	}
	return float64(inter) / float64(union)
}
