package tools

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

type fakeBook struct {
	rs *book.ResourceSet
}

func (f *fakeBook) Metadata() book.Metadata      { return book.Metadata{} }
func (f *fakeBook) Resources() *book.ResourceSet { return f.rs }
func (f *fakeBook) FileType() book.FileType      { return book.EPUB }
func (f *fakeBook) Version() string              { return "3.0" }

func newTestResources(t *testing.T, contents []string) *book.ResourceSet {
	t.Helper()
	var resources []*book.Resource
	var ros []book.ReadingOrderItem
	for i, c := range contents {
		content := c
		r := &book.Resource{
			Id:           string(rune('a' + i)),
			MediaType:    mediatype.XHTML,
			ResolvedHref: string(rune('a'+i)) + ".xhtml",
			Open: func() (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader(content)), nil
			},
		}
		resources = append(resources, r)
		ros = append(ros, book.ReadingOrderItem{Resource: r, Linear: true})
	}
	return book.NewResourceSet(resources, ros, nil)
}

func TestFingerprintContent_Basic(t *testing.T) {
	ctx := context.Background()
	rs := newTestResources(t, []string{"<p>Hello world</p>", "<p>Second chapter content here</p>"})
	b := &fakeBook{rs: rs}
	fp, err := FingerprintContent(ctx, b, WithMinHash(MinHashConfig{NumHashes: 16, ShingleSize: 2}), WithSimHash())
	if err != nil {
		t.Fatalf("FingerprintContent failed: %v", err)
	}
	if len(fp.MinHash) != 16 {
		t.Fatalf("MinHash len %d want 16", len(fp.MinHash))
	}
	if !fp.HasSimHash {
		t.Fatalf("HasSimHash should be true")
	}
	// Ensure not empty
	if fp.MinHash[0] == ^uint64(0) {
		t.Fatalf("MinHash should be populated")
	}
}

func TestFingerprintContent_DelimiterInjected(t *testing.T) {
	ctx := context.Background()
	rs := newTestResources(t, []string{"<p>First</p>", "<p>Second</p>"})
	b := &fakeBook{rs: rs}
	// Use FingerprintContent to verify delimiter effect via extracted plain
	// Also directly test delimited reader
	rc, handle, err := OpenReadingOrderWithFingerprint(ctx, rs, mediatype.PlainText, WithMinHash(MinHashConfig{NumHashes: 8, ShingleSize: 1}), WithSimHash())
	if err != nil {
		t.Fatalf("OpenReadingOrderWithFingerprint failed: %v", err)
	}
	data, _ := io.ReadAll(rc)
	_ = rc.Close()
	s := string(data)
	if s != "First\nSecond" {
		t.Fatalf("delimiter not injected, got %q want %q", s, "First\nSecond")
	}
	_ = handle.Result()
	// Also via FingerprintContent result should match direct hasher on same delimited string
	fp, _ := FingerprintContent(ctx, b, WithMinHash(MinHashConfig{NumHashes: 8, ShingleSize: 1}), WithSimHash())
	directMH := NewMinHasher(WithShingleSize(1), WithNumHashes(8))
	mustWrite(t, directMH, []byte("First\nSecond"))
	if !equalSigs(fp.MinHash, directMH.Signature()) {
		t.Fatalf("FingerprintContent MinHash mismatch vs direct")
	}
	directSH := NewSimHasher()
	mustWrite(t, directSH, []byte("First\nSecond"))
	if fp.SimHash != directSH.Sum64() {
		t.Fatalf("SimHash mismatch")
	}
}

func TestFingerprintContent_Delimiter_ThreeChapters_SmallBuffer(t *testing.T) {
	ctx := context.Background()
	rs := newTestResources(t, []string{"<p>First</p>", "<p>Second</p>", "<p>Third</p>"})
	rc, handle, err := OpenReadingOrderWithFingerprint(ctx, rs, mediatype.PlainText, WithMinHash(MinHashConfig{NumHashes: 8, ShingleSize: 1}), WithSimHash())
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	defer func() { _ = rc.Close() }()
	buf := make([]byte, 1)
	var out bytes.Buffer
	for {
		n, e := rc.Read(buf)
		if n > 0 {
			mustWrite(t, &out, buf[:n])
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatalf("Read failed: %v", e)
		}
	}
	if got := out.String(); got != "First\nSecond\nThird" {
		t.Fatalf("3 chapters got %q want %q", got, "First\nSecond\nThird")
	}
	fp := handle.Result()
	manual := NewMinHasher(WithShingleSize(1), WithNumHashes(8))
	mustWrite(t, manual, []byte("First\nSecond\nThird"))
	if !equalSigs(fp.MinHash, manual.Signature()) {
		t.Fatalf("3-chapter Jaccard mismatch")
	}
}

func TestFingerprintContent_Streaming_vs_Direct_Equal(t *testing.T) {
	ctx := context.Background()
	rs := newTestResources(t, []string{"<p>the quick brown fox</p>", "<p>jumps over the lazy dog</p>"})
	b := &fakeBook{rs: rs}
	fp1, err := FingerprintContent(ctx, b, WithMinHash(MinHashConfig{NumHashes: 16, ShingleSize: 2}), WithSimHash())
	if err != nil {
		t.Fatalf("FingerprintContent failed: %v", err)
	}
	// Streaming via OpenReadingOrderWithFingerprint with 1-byte reads
	rc, handle, err := OpenReadingOrderWithFingerprint(ctx, rs, mediatype.PlainText, WithMinHash(MinHashConfig{NumHashes: 16, ShingleSize: 2}), WithSimHash())
	if err != nil {
		t.Fatalf("OpenReadingOrderWithFingerprint failed: %v", err)
	}
	buf := make([]byte, 1)
	for {
		n, e := rc.Read(buf)
		if e == io.EOF {
			break
		}
		if e != nil && e != io.EOF {
			t.Fatalf("Read failed: %v", e)
		}
		if n == 0 && e == nil {
			break
		}
	}
	_ = rc.Close()
	fp2 := handle.Result()
	if !equalSigs(fp1.MinHash, fp2.MinHash) {
		t.Fatalf("streaming vs direct MinHash mismatch")
	}
	if fp1.SimHash != fp2.SimHash {
		t.Fatalf("streaming vs direct SimHash mismatch %x vs %x", fp1.SimHash, fp2.SimHash)
	}
}

func TestFingerprintContent_FanOut_MultiWriter(t *testing.T) {
	ctx := context.Background()
	rs := newTestResources(t, []string{"<p>the quick brown fox jumps over the lazy dog</p>"})
	rc, err := rs.OpenReadingOrderAs(ctx, mediatype.PlainText)
	if err != nil {
		t.Fatalf("OpenReadingOrderAs failed: %v", err)
	}
	// Delimited reader for fair comparison (single chapter, no delimiter diff)
	// Use tools' FingerprintContent as ground truth
	b := &fakeBook{rs: rs}
	fp, _ := FingerprintContent(ctx, b, WithMinHash(MinHashConfig{NumHashes: 16, ShingleSize: 3}), WithSimHash())
	// Manual fan-out via TeeReader
	mh := NewMinHasher(WithShingleSize(3), WithNumHashes(16))
	sh := NewSimHasher()
	tee := io.TeeReader(rc, io.MultiWriter(mh, sh))
	_, _ = io.Copy(io.Discard, tee)
	_ = rc.Close()
	if !equalSigs(mh.Signature(), fp.MinHash) {
		t.Fatalf("fan-out MinHash mismatch Jaccard=%f", Jaccard(mh.Signature(), fp.MinHash))
	}
	if sh.Sum64() != fp.SimHash {
		t.Fatalf("fan-out SimHash mismatch")
	}
}

func TestFingerprintContent_OnlyMinHash(t *testing.T) {
	ctx := context.Background()
	rs := newTestResources(t, []string{"<p>only minhash test content here</p>"})
	b := &fakeBook{rs: rs}
	fp, err := FingerprintContent(ctx, b, WithMinHash(MinHashConfig{NumHashes: 8, ShingleSize: 2}))
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if len(fp.MinHash) != 8 {
		t.Fatalf("len %d", len(fp.MinHash))
	}
	if fp.HasSimHash {
		t.Fatalf("HasSimHash should be false")
	}
}

func TestFingerprintContent_OnlySimHash(t *testing.T) {
	ctx := context.Background()
	rs := newTestResources(t, []string{"<p>only simhash test</p>"})
	b := &fakeBook{rs: rs}
	fp, err := FingerprintContent(ctx, b, WithSimHash())
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if fp.MinHash != nil {
		t.Fatalf("MinHash should be nil")
	}
	if !fp.HasSimHash {
		t.Fatalf("HasSimHash true")
	}
}

func TestFingerprintContent_DefaultBoth(t *testing.T) {
	ctx := context.Background()
	rs := newTestResources(t, []string{"<p>default both test</p>"})
	b := &fakeBook{rs: rs}
	fp, err := FingerprintContent(ctx, b)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if len(fp.MinHash) != 128 {
		t.Fatalf("default MinHash len %d want 128", len(fp.MinHash))
	}
	if !fp.HasSimHash {
		t.Fatalf("default should have SimHash")
	}
}

func TestFingerprintContent_EmptyResourceSet(t *testing.T) {
	ctx := context.Background()
	rs := book.NewResourceSet(nil, nil, nil)
	b := &fakeBook{rs: rs}
	fp, err := FingerprintContent(ctx, b)
	if err != nil {
		t.Fatalf("empty should not error: %v", err)
	}
	if len(fp.MinHash) != 128 {
		t.Fatalf("len %d", len(fp.MinHash))
	}
	if fp.MinHash[0] != ^uint64(0) {
		t.Fatalf("empty sig should be max")
	}
	if fp.SimHash != 0 {
		t.Fatalf("empty sim should be 0, got %x", fp.SimHash)
	}
	// Also via OpenReadingOrderWithFingerprint
	rc, handle, err := OpenReadingOrderWithFingerprint(ctx, rs, mediatype.PlainText)
	if err != nil {
		t.Fatalf("open empty failed: %v", err)
	}
	data, _ := io.ReadAll(rc)
	_ = rc.Close()
	if len(data) != 0 {
		t.Fatalf("empty data len %d", len(data))
	}
	fp2 := handle.Result()
	if !equalSigs(fp.MinHash, fp2.MinHash) {
		t.Fatalf("empty mismatch")
	}
}

func TestFingerprintContent_OnlyNonLinear_Skipped(t *testing.T) {
	ctx := context.Background()
	ra := &book.Resource{Id: "a", MediaType: mediatype.XHTML, ResolvedHref: "a.xhtml", Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("<p>KeepMe</p>")), nil
	}}
	rSkip := &book.Resource{Id: "b", MediaType: mediatype.XHTML, ResolvedHref: "b.xhtml", Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("<p>SkipMe</p>")), nil
	}}
	rs := book.NewResourceSet([]*book.Resource{ra, rSkip}, []book.ReadingOrderItem{{Resource: ra, Linear: true}, {Resource: rSkip, Linear: false}}, nil)
	b := &fakeBook{rs: rs}
	fp, _ := FingerprintContent(ctx, b, WithMinHash(MinHashConfig{NumHashes: 8, ShingleSize: 1}))
	manual := NewMinHasher(WithShingleSize(1), WithNumHashes(8))
	mustWrite(t, manual, []byte("KeepMe"))
	if !equalSigs(fp.MinHash, manual.Signature()) {
		t.Fatalf("non-linear should be skipped")
	}
	// Sim likewise
	fp2, _ := FingerprintContent(ctx, b, WithSimHash())
	manualS := NewSimHasher()
	mustWrite(t, manualS, []byte("KeepMe"))
	if fp2.SimHash != manualS.Sum64() {
		t.Fatalf("sim non-linear skip failed")
	}
}

func TestFingerprintContent_NilBook_ResourceSet_Error(t *testing.T) {
	ctx := context.Background()
	if _, err := FingerprintContent(ctx, nil); err == nil {
		t.Fatalf("nil book should error")
	}
	var nilBook *fakeBook
	if _, err := FingerprintContent(ctx, nilBook); err == nil {
		t.Fatalf("nil book should error")
	}
	fb := &fakeBook{rs: nil}
	if _, err := FingerprintContent(ctx, fb); err == nil {
		t.Fatalf("nil rs should error")
	}
	if _, _, err := OpenReadingOrderWithFingerprint(ctx, nil, mediatype.PlainText); err == nil {
		t.Fatalf("nil rs should error")
	}
	emptyRs := book.NewResourceSet(nil, nil, nil)
	// Valid target empty should error on openDelimitedReadingOrder via FingerprintContent?
	// FingerprintContent uses PlainText internally, so not testing empty target there.
	// Test OpenReadingOrderWithFingerprint with empty target
	if _, _, err := OpenReadingOrderWithFingerprint(ctx, emptyRs, ""); err == nil {
		t.Fatalf("empty target should error")
	}
	// Also test invalid target
	if _, _, err := OpenReadingOrderWithFingerprint(ctx, emptyRs, "   "); err == nil {
		t.Fatalf("whitespace target should error")
	}
}

func TestFingerprintContent_NoTransformer_Propagates(t *testing.T) {
	ctx := context.Background()
	r := &book.Resource{Id: "a", MediaType: mediatype.JPEG, ResolvedHref: "a.jpg", Open: func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader([]byte{1, 2})), nil
	}}
	rs := book.NewResourceSet([]*book.Resource{r}, []book.ReadingOrderItem{{Resource: r, Linear: true}}, nil)
	b := &fakeBook{rs: rs}
	_, err := FingerprintContent(ctx, b)
	if err == nil {
		t.Fatalf("should propagate ErrNoTransformer")
	}
	if !errors.Is(err, book.ErrNoTransformer) && !strings.Contains(err.Error(), "no transformer") {
		t.Fatalf("expected ErrNoTransformer, got %v", err)
	}
	// Also streaming variant should error on Read
	rc, _, err := OpenReadingOrderWithFingerprint(ctx, rs, mediatype.PlainText)
	if err != nil {
		t.Fatalf("Open should not error lazily, got %v", err)
	}
	defer func() { _ = rc.Close() }()
	buf := make([]byte, 10)
	_, err = rc.Read(buf)
	if err == nil {
		t.Fatalf("Read should propagate ErrNoTransformer")
	}
}

func TestFingerprintContent_RealEPUB_vs_MOBI_SameBook(t *testing.T) {
	ctx := context.Background()
	// Two books with same textual content but different MediaTypes (XHTML vs MobiHTML)
	// Should be near-identical Jaccard >0.85
	content := "<p>the quick brown fox jumps over the lazy dog the quick brown fox jumps</p>"
	rEpub := &book.Resource{Id: "a", MediaType: mediatype.XHTML, ResolvedHref: "a.xhtml", Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(content)), nil
	}}
	rMobi := &book.Resource{Id: "a", MediaType: mediatype.MobiHTML, ResolvedHref: "a.html", Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(content)), nil
	}}
	rsE := book.NewResourceSet([]*book.Resource{rEpub}, []book.ReadingOrderItem{{Resource: rEpub, Linear: true}}, nil)
	rsM := book.NewResourceSet([]*book.Resource{rMobi}, []book.ReadingOrderItem{{Resource: rMobi, Linear: true}}, nil)
	bE := &fakeBook{rs: rsE}
	bM := &fakeBook{rs: rsM}
	fpE, _ := FingerprintContent(ctx, bE, WithMinHash(MinHashConfig{NumHashes: 64, ShingleSize: 3}))
	fpM, _ := FingerprintContent(ctx, bM, WithMinHash(MinHashConfig{NumHashes: 64, ShingleSize: 3}))
	j := Jaccard(fpE.MinHash, fpM.MinHash)
	if j < 0.85 {
		t.Fatalf("same content different MediaType Jaccard %.3f <0.85", j)
	}
	// SimHash should be identical
	fpEs, _ := FingerprintContent(ctx, bE, WithSimHash())
	fpMs, _ := FingerprintContent(ctx, bM, WithSimHash())
	if fpEs.SimHash != fpMs.SimHash {
		t.Fatalf("SimHash identical content should match, got %x vs %x", fpEs.SimHash, fpMs.SimHash)
	}
}

func TestFingerprintContent_SyntheticDedup(t *testing.T) {
	ctx := context.Background()
	base := "<p>the quick brown fox jumps over the lazy dog</p>"
	variant := "<p>the quick brown fox leaps over the lazy dog</p>" // one word changed
	r1 := &book.Resource{Id: "a", MediaType: mediatype.XHTML, ResolvedHref: "a.xhtml", Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(base)), nil
	}}
	r2 := &book.Resource{Id: "b", MediaType: mediatype.XHTML, ResolvedHref: "b.xhtml", Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(variant)), nil
	}}
	rs1 := book.NewResourceSet([]*book.Resource{r1}, []book.ReadingOrderItem{{Resource: r1, Linear: true}}, nil)
	rs2 := book.NewResourceSet([]*book.Resource{r2}, []book.ReadingOrderItem{{Resource: r2, Linear: true}}, nil)
	b1 := &fakeBook{rs: rs1}
	b2 := &fakeBook{rs: rs2}
	fp1, _ := FingerprintContent(ctx, b1, WithMinHash(MinHashConfig{NumHashes: 64, ShingleSize: 3}))
	fp2, _ := FingerprintContent(ctx, b2, WithMinHash(MinHashConfig{NumHashes: 64, ShingleSize: 3}))
	j := Jaccard(fp1.MinHash, fp2.MinHash)
	if j < 0.5 || j > 0.99 {
		t.Fatalf("near duplicate Jaccard %.3f out of expected 0.5-0.99", j)
	}
	// Far different
	rFar := &book.Resource{Id: "c", MediaType: mediatype.XHTML, ResolvedHref: "c.xhtml", Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("<p>completely different xyz 123 unrelated content</p>")), nil
	}}
	rsFar := book.NewResourceSet([]*book.Resource{rFar}, []book.ReadingOrderItem{{Resource: rFar, Linear: true}}, nil)
	bFar := &fakeBook{rs: rsFar}
	fpFar, _ := FingerprintContent(ctx, bFar, WithMinHash(MinHashConfig{NumHashes: 64, ShingleSize: 3}))
	if jFar := Jaccard(fp1.MinHash, fpFar.MinHash); jFar > 0.2 {
		t.Fatalf("far Jaccard should be low, got %f", jFar)
	}
}

func TestDelimitedMultiReadCloser_CloseIdempotent_ReadAfterClose(t *testing.T) {
	ctx := context.Background()
	rs := newTestResources(t, []string{"<p>A</p>"})
	rc, _, err := OpenReadingOrderWithFingerprint(ctx, rs, mediatype.PlainText)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	buf := make([]byte, 10)
	_, _ = rc.Read(buf)
	if err := rc.Close(); err != nil {
		t.Fatalf("first close %v", err)
	}
	if err := rc.Close(); err != nil {
		t.Fatalf("second close should be idempotent, got %v", err)
	}
	if _, err := rc.Read(buf); err == nil {
		t.Fatalf("Read after Close should error")
	}
}

func TestDelimitedMultiReadCloser_EmptyTarget(t *testing.T) {
	ctx := context.Background()
	rs := newTestResources(t, []string{"<p>A</p>"})
	if _, _, err := OpenReadingOrderWithFingerprint(ctx, rs, ""); err == nil {
		t.Fatalf("empty target should error")
	}
	if _, _, err := OpenReadingOrderWithFingerprint(ctx, rs, "   "); err == nil {
		t.Fatalf("whitespace target should error")
	}
}

func TestDelimitedMultiReadCloser_LazyOpen(t *testing.T) {
	ctx := context.Background()
	opens := 0
	ra := &book.Resource{Id: "a", MediaType: mediatype.XHTML, ResolvedHref: "a.xhtml", Open: func() (io.ReadCloser, error) {
		opens++
		return io.NopCloser(strings.NewReader("<p>A</p>")), nil
	}}
	rb := &book.Resource{Id: "b", MediaType: mediatype.XHTML, ResolvedHref: "b.xhtml", Open: func() (io.ReadCloser, error) {
		opens++
		return io.NopCloser(strings.NewReader("<p>B</p>")), nil
	}}
	rs := book.NewResourceSet([]*book.Resource{ra, rb}, []book.ReadingOrderItem{{Resource: ra, Linear: true}, {Resource: rb, Linear: true}}, nil)
	rc, _, err := OpenReadingOrderWithFingerprint(ctx, rs, mediatype.PlainText)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	defer func() { _ = rc.Close() }()
	if opens != 0 {
		t.Fatalf("should be lazy, opens=%d", opens)
	}
	buf := make([]byte, 1)
	var out bytes.Buffer
	for {
		n, e := rc.Read(buf)
		if n > 0 {
			mustWrite(t, &out, buf[:n])
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatalf("Read %v", e)
		}
	}
	if out.String() != "A\nB" {
		t.Fatalf("got %q want %q", out.String(), "A\nB")
	}
	if opens != 2 {
		t.Fatalf("should have opened 2, got %d", opens)
	}
}

func TestFingerprintContent_SimHashShingleSize_Variant(t *testing.T) {
	ctx := context.Background()
	rs := newTestResources(t, []string{"<p>a b c d e f g h i j k l m n o p</p>"})
	b := &fakeBook{rs: rs}
	fp3, _ := FingerprintContent(ctx, b, WithSimHashShingleSize(3))
	fp5, _ := FingerprintContent(ctx, b, WithSimHash())
	// For this text, different k should generally give different SimHash (not guaranteed but likely)
	if fp3.SimHash == fp5.SimHash {
		// Allow collision but log
		t.Logf("different SimShingle sizes gave same hash (possible collision)")
	}
	// Zero should default to 5
	fp0, _ := FingerprintContent(ctx, b, WithSimHashShingleSize(0))
	if fp0.SimHash != fp5.SimHash {
		t.Fatalf("zero shingle should default to 5")
	}
}

func TestFingerprintHandle_Result_BeforeRead(t *testing.T) {
	ctx := context.Background()
	rs := newTestResources(t, []string{"<p>hello world test</p>"})
	// Handle before any read should be empty (no shingles yet)
	rc, handle, err := OpenReadingOrderWithFingerprint(ctx, rs, mediatype.PlainText, WithMinHash(MinHashConfig{NumHashes: 8, ShingleSize: 2}))
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	fpBefore := handle.Result()
	if fpBefore.MinHash[0] != ^uint64(0) {
		t.Fatalf("before read should be empty max")
	}
	_ = rc.Close()

	// Fresh handle, read fully, then result should be populated and differ from empty
	rc2, handle2, err := OpenReadingOrderWithFingerprint(ctx, rs, mediatype.PlainText, WithMinHash(MinHashConfig{NumHashes: 8, ShingleSize: 2}))
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	_, _ = io.ReadAll(rc2)
	_ = rc2.Close()
	fpAfter := handle2.Result()
	if fpAfter.MinHash[0] == ^uint64(0) {
		t.Fatalf("after read should be populated")
	}
	if equalSigs(fpBefore.MinHash, fpAfter.MinHash) {
		t.Fatalf("before vs after should differ")
	}
	// Result is idempotent
	fpAfter2 := handle2.Result()
	if !equalSigs(fpAfter.MinHash, fpAfter2.MinHash) {
		t.Fatalf("Result idempotent failed")
	}
}
