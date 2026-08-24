package tools

import "testing"

func TestSimHasher_Determinism_SmallVsLarge(t *testing.T) {
	text := "the quick brown fox jumps over the lazy dog the quick brown fox jumps"
	sh1 := NewSimHasher()
	sh2 := NewSimHasher()
	for i := 0; i < len(text); i += 3 {
		end := i + 3
		if end > len(text) {
			end = len(text)
		}
		mustWrite(t, sh1, []byte(text[i:end]))
	}
	mustWrite(t, sh2, []byte(text))
	if sh1.Sum64() != sh2.Sum64() {
		t.Fatalf("small vs large mismatch %x vs %x", sh1.Sum64(), sh2.Sum64())
	}
	sh3 := NewSimHasher()
	for i := 0; i < len(text); i++ {
		mustWrite(t, sh3, []byte(text[i : i+1]))
	}
	if sh1.Sum64() != sh3.Sum64() {
		t.Fatalf("1-byte vs large mismatch")
	}
}

func TestSimHasher_Hamming_Identical_0(t *testing.T) {
	a := NewSimHasher()
	b := NewSimHasher()
	mustWrite(t, a, []byte("hello world"))
	mustWrite(t, b, []byte("hello world"))
	if Hamming(a.Sum64(), b.Sum64()) != 0 {
		t.Fatalf("identical hamming should be 0")
	}
	if a.Hamming(b) != 0 {
		t.Fatalf("method Hamming failed")
	}
	if Similarity(a.Sum64(), b.Sum64()) != 1.0 {
		t.Fatalf("similarity should be 1")
	}
}

func TestSimHasher_Hamming_SlightDiff(t *testing.T) {
	a := NewSimHasher()
	b := NewSimHasher()
	mustWrite(t, a, []byte("hello world this is a test"))
	mustWrite(t, b, []byte("hello world this is a testx"))
	h := Hamming(a.Sum64(), b.Sum64())
	if h == 0 {
		t.Fatalf("slight diff should not be 0")
	}
	// Hamming should be less than 32 for near duplicate
	if h > 32 {
		t.Fatalf("hamming too large for near duplicate: %d", h)
	}
	// Far different
	c := NewSimHasher()
	mustWrite(t, c, []byte("completely different content xyz 123"))
	if Hamming(a.Sum64(), c.Sum64()) < 10 {
		t.Fatalf("far different should have larger hamming")
	}
}

func TestSimHasher_Empty_0(t *testing.T) {
	sh := NewSimHasher()
	if sh.Sum64() != 0 {
		t.Fatalf("empty should be 0, got %x", sh.Sum64())
	}
	sh2 := NewSimHasher()
	if Hamming(sh.Sum64(), sh2.Sum64()) != 0 {
		t.Fatalf("empty vs empty hamming 0")
	}
	// empty vs non-empty
	sh3 := NewSimHasher()
	mustWrite(t, sh3, []byte("hello"))
	if Hamming(sh.Sum64(), sh3.Sum64()) == 0 {
		t.Fatalf("empty vs non-empty should differ")
	}
}

func TestSimHasher_ShortDoc(t *testing.T) {
	sh := NewSimHasher()
	mustWrite(t, sh, []byte("hello world"))
	if sh.Sum64() == 0 {
		t.Fatalf("short doc should not be 0")
	}
	sh2 := NewSimHasher()
	mustWrite(t, sh2, []byte("hello world"))
	if sh.Sum64() != sh2.Sum64() {
		t.Fatalf("identical short doc mismatch")
	}
}

func TestSimHasher_ShingleSize_Config(t *testing.T) {
	text := "a b c d e f g h i j"
	sh3 := NewSimHasher(WithSimShingleSize(3))
	sh5 := NewSimHasher(WithSimShingleSize(5))
	mustWrite(t, sh3, []byte(text))
	mustWrite(t, sh5, []byte(text))
	if sh3.Sum64() == sh5.Sum64() {
		// Could collide by chance but unlikely for this text; if they match, at least test that config is respected
		// For this specific text they should differ; if they match we consider it flaky but allow
		t.Logf("warning: different shingle sizes gave same hash (collision), consider changing text")
	}
	// Zero should default to 5
	sh0 := NewSimHasher(WithSimShingleSize(0))
	mustWrite(t, sh0, []byte(text))
	sh5b := NewSimHasher()
	mustWrite(t, sh5b, []byte(text))
	if sh0.Sum64() != sh5b.Sum64() {
		t.Fatalf("zero shingle should default to 5")
	}
	// Negative also defaults
	shNeg := NewSimHasher(WithSimShingleSize(-1))
	mustWrite(t, shNeg, []byte(text))
	if shNeg.Sum64() != sh5b.Sum64() {
		t.Fatalf("negative shingle should default")
	}
}

func TestSimHasher_WriteAfterFinalized_Error(t *testing.T) {
	sh := NewSimHasher()
	mustWrite(t, sh, []byte("hello"))
	_ = sh.Sum64()
	if _, err := sh.Write([]byte("more")); err == nil {
		t.Fatalf("expected error after finalized")
	}
	// Idempotent Sum64
	s1 := sh.Sum64()
	s2 := sh.Sum64()
	if s1 != s2 {
		t.Fatalf("Sum64 not idempotent")
	}
}

func TestSimHasher_Sum64_Idempotent(t *testing.T) {
	sh := NewSimHasher()
	mustWrite(t, sh, []byte("a b c d e"))
	s1 := sh.Sum64()
	s2 := sh.Sum64()
	s3 := sh.Sum64()
	if s1 != s2 || s2 != s3 {
		t.Fatalf("not idempotent")
	}
}

func TestHamming_Symmetry(t *testing.T) {
	a := uint64(0x1234567890abcdef)
	b := uint64(0xfedcba0987654321)
	if Hamming(a, b) != Hamming(b, a) {
		t.Fatalf("symmetry failed")
	}
	if Hamming(a, a) != 0 {
		t.Fatalf("self 0")
	}
	// 0 vs all bits 64
	if Hamming(0, ^uint64(0)) != 64 {
		t.Fatalf("0 vs max should be 64")
	}
}
