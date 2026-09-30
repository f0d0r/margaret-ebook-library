package mobi

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
)

// The TAGX tables mirror the tags calibre reads from the SKEL and DIV indices:
// SKEL tag 1 (div count) and tag 6 (start, length); DIV tag 2 (CNCX reference),
// tag 3 (file number), tag 4 (sequence number) and tag 6 (start, length).
var (
	skelTags = []tagxDef{
		{tag: 1, numOfValues: 1},
		{tag: 6, numOfValues: 2},
	}
	divTags = []tagxDef{
		{tag: 2, numOfValues: 1},
		{tag: 3, numOfValues: 1},
		{tag: 4, numOfValues: 1},
		{tag: 6, numOfValues: 2},
	}
)

// kf8Text is a text flow laid out the way a real KF8 flow is: the skeleton
// bytes first, then the div payloads the DIV table points at. The DIV entry's
// key is the insert position inside the skeleton where the payload belongs.
var kf8Text = []byte("<html><head></head><body></body></html><p>hello</p>")

const (
	kf8Skeleton    = "<html><head></head><body></body></html>"
	kf8SkeletonLen = len(kf8Skeleton)
	kf8PartStart   = kf8SkeletonLen
	kf8Part        = "<p>hello</p>"
	kf8PartLen     = len(kf8Part)
	kf8InsertPos   = len("<html><head></head><body>") // inside the body
	kf8Assembled   = "<html><head></head><body><p>hello</p></body></html>"
)

// makeStandaloneKF8Blob builds a standalone KF8 book: MOBI8 header with FDST,
// SKEL and DIV indices pointing at a single text record.
func makeStandaloneKF8Blob(t *testing.T, mutate func(*[]recordSpec, *mobiHeaderOpts)) []byte {
	t.Helper()

	opts := defaultMobiHeaderOpts()
	opts.compression = CompressionNone
	opts.textLength = uint32(len(kf8Text))
	opts.textRecordCount = 1
	opts.headerLength = 264
	opts.mobiType = MobiTypeKF8
	opts.mobiVersion = 8
	opts.fdstIdx = 2
	opts.fdstCount = 2
	opts.skelIdx = 4
	opts.divIdx = 6

	skel := makeIndxSection(skelTags, []indxEntry{{
		ident:  "SKEL0000000000",
		values: map[indxTagRef][]int{0: {1}, 1: {0, kf8SkeletonLen}},
	}})
	div := makeIndxSection(divTags, []indxEntry{{
		ident:  fmt.Sprint(kf8InsertPos), // the key is the insert position
		values: map[indxTagRef][]int{1: {0}, 3: {kf8PartStart, kf8PartLen}},
	}})

	records := []recordSpec{
		makeMobiHeaderRecord(opts), // 0
		{data: kf8Text},            // 1
		{data: makeFDSTRecord([][2]int{{0, len(kf8Text)}})}, // 2
		{data: []byte("pad")},                               // 3
		{data: skel[0]},                                     // 4
		{data: skel[1]},                                     // 5
		{data: div[0]},                                      // 6
		{data: div[1]},                                      // 7
	}
	if mutate != nil {
		mutate(&records, &opts)
		records[0] = makeMobiHeaderRecord(opts)
	}
	return makePdbBlob(t, "TestKF8", records)
}

// makeDualKF8Blob builds a joint MOBI6+KF8 book: a MOBI6 header and text, a
// BOUNDARY record, then the KF8 header and text. EXTH type 121 points at the
// KF8 header record (index 3), which is how calibre finds it.
func makeDualKF8Blob(t *testing.T, mutate func(*[]recordSpec, *mobiHeaderOpts)) []byte {
	t.Helper()

	mobi6Text := []byte("<html><body>MOBI6 fallback</body></html>")
	mobi6 := defaultMobiHeaderOpts()
	mobi6.textLength = uint32(len(mobi6Text))
	mobi6.textRecordCount = 1
	mobi6.exthRecords = map[uint32][]byte{KF8_HEADER_INDEX: makeWordValue(3)}

	kf8 := defaultMobiHeaderOpts()
	kf8.textLength = uint32(len(kf8Text))
	kf8.textRecordCount = 1
	kf8.headerLength = 264
	kf8.mobiType = MobiTypeKF8
	kf8.mobiVersion = 8
	// KF8-relative record numbers. The reader views the KF8 records starting at
	// the record before the KF8 header (the BOUNDARY record at index 2), so the
	// KF8 header itself is record 1 and FDST/SKEL/DIV are 2/3/5.
	kf8.fdstIdx = 2
	kf8.fdstCount = 2
	kf8.skelIdx = 3
	kf8.divIdx = 5

	skel := makeIndxSection(skelTags, []indxEntry{{
		ident:  "SKEL0000000000",
		values: map[indxTagRef][]int{0: {1}, 1: {0, kf8SkeletonLen}},
	}})
	div := makeIndxSection(divTags, []indxEntry{{
		ident:  fmt.Sprint(kf8InsertPos),
		values: map[indxTagRef][]int{1: {0}, 3: {kf8PartStart, kf8PartLen}},
	}})

	records := []recordSpec{
		makeMobiHeaderRecord(mobi6),                         // 0
		{data: mobi6Text},                                   // 1
		{data: []byte("BOUNDARY")},                          // 2
		makeMobiHeaderRecord(kf8),                           // 3 (KF8 record 0)
		{data: kf8Text},                                     // 4 (KF8 record 1)
		{data: makeFDSTRecord([][2]int{{0, len(kf8Text)}})}, // 5 (KF8 record 2)
		{data: skel[0]},                                     // 6 (KF8 record 3)
		{data: skel[1]},                                     // 7 (KF8 record 4)
		{data: div[0]},                                      // 8 (KF8 record 5)
		{data: div[1]},                                      // 9 (KF8 record 6)
	}
	if mutate != nil {
		mutate(&records, &kf8)
		records[3] = makeMobiHeaderRecord(kf8)
	}
	return makePdbBlob(t, "TestDualKF8", records)
}

// readAllResource returns the bytes one resource serves on Open.
func readAllResource(t *testing.T, r book.Resource) string {
	t.Helper()
	rc, err := r.Open()
	if err != nil {
		t.Fatalf("Open(%s) error: %v", r.Name, err)
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll(%s) error: %v", r.Name, err)
	}
	return string(data)
}

func TestKF8StandaloneContent(t *testing.T) {
	blob := makeStandaloneKF8Blob(t, nil)

	got, err := NewMobiReader(config.DefaultConfig()).Read(book.NewBytesBlob(blob))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	content := got.Resources().ReadingOrder()
	if len(content) != 1 {
		t.Fatalf("reading order = %d resources, want 1", len(content))
	}
	if content[0].Resource.Name != "part0000.html" {
		t.Errorf("resource name = %q, want part0000.html", content[0].Resource.Name)
	}
	if body := readAllResource(t, *content[0].Resource); body != kf8Assembled {
		t.Errorf("content = %q, want %q", body, kf8Assembled)
	}
}

func TestKF8DualReadsKF8Content(t *testing.T) {
	blob := makeDualKF8Blob(t, nil)

	got, err := NewMobiReader(config.DefaultConfig()).Read(book.NewBytesBlob(blob))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if v := got.Version(); v != "6/8" {
		t.Errorf("Version() = %q, want 6/8", v)
	}

	content := got.Resources().ReadingOrder()
	if len(content) != 1 {
		t.Fatalf("reading order = %d resources, want 1", len(content))
	}
	body := readAllResource(t, *content[0].Resource)
	if !strings.Contains(body, "hello") {
		t.Errorf("content = %q, want the KF8 part content", body)
	}
	if strings.Contains(body, "MOBI6 fallback") {
		t.Errorf("content = %q, want the KF8 part, not the MOBI6 fallback", body)
	}
}

func TestKF8DualEmptyKF8ContentFallsBackToMOBI6(t *testing.T) {
	// The KF8 header declares no text records, so no KF8 content can be
	// produced and the intact MOBI6 content must be served instead.
	blob := makeDualKF8Blob(t, func(records *[]recordSpec, opts *mobiHeaderOpts) {
		opts.textRecordCount = 0
	})

	got, err := NewMobiReader(config.DefaultConfig()).Read(book.NewBytesBlob(blob))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	content := got.Resources().ReadingOrder()
	if len(content) != 1 {
		t.Fatalf("reading order = %d resources, want 1", len(content))
	}
	if content[0].Resource.Name != "index.html" {
		t.Errorf("resource name = %q, want index.html (MOBI6 fallback)", content[0].Resource.Name)
	}
	if body := readAllResource(t, *content[0].Resource); !strings.Contains(body, "MOBI6 fallback") {
		t.Errorf("content = %q, want the MOBI6 fallback content", body)
	}
}

func TestKF8DualStrictModeReportsParseError(t *testing.T) {
	blob := makeDualKF8Blob(t, func(records *[]recordSpec, opts *mobiHeaderOpts) {
		opts.textRecordCount = 0
	})

	cfg := config.DefaultConfig()
	strict := false
	cfg.KF8FallbackToMOBI6 = &strict
	got, err := NewMobiReader(cfg).Read(book.NewBytesBlob(blob))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	content := got.Resources().ReadingOrder()
	if len(content) != 1 {
		t.Fatalf("reading order = %d resources, want 1", len(content))
	}
	if content[0].Resource.Name != "part0000.html" {
		t.Errorf("resource name = %q, want part0000.html", content[0].Resource.Name)
	}
	_, openErr := content[0].Resource.Open()
	if openErr == nil {
		t.Fatal("Open() = nil error, want the KF8 parse failure")
	}
	if !strings.Contains(openErr.Error(), "KF8 parsing failed") {
		t.Errorf("Open() error = %v, want a KF8 parse failure", openErr)
	}
}

func TestKF8StandaloneEmptyContentReportsError(t *testing.T) {
	// A standalone KF8 file has no MOBI6 content to fall back to, so the
	// parse failure must reach the caller even with fallback enabled.
	blob := makeStandaloneKF8Blob(t, func(records *[]recordSpec, opts *mobiHeaderOpts) {
		opts.textRecordCount = 0
	})

	got, err := NewMobiReader(config.DefaultConfig()).Read(book.NewBytesBlob(blob))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	content := got.Resources().ReadingOrder()
	if len(content) != 1 {
		t.Fatalf("reading order = %d resources, want 1", len(content))
	}
	_, openErr := content[0].Resource.Open()
	if openErr == nil {
		t.Fatal("Open() = nil error, want the KF8 parse failure")
	}
	if !strings.Contains(openErr.Error(), "KF8 parsing failed") {
		t.Errorf("Open() error = %v, want a KF8 parse failure", openErr)
	}
}

func TestKF8RecoveredPanicIsClassifiedCorrupt(t *testing.T) {
	// A nil dereference inside the KF8 reader must come back as an error
	// classified as ErrCorrupt, never as a panic.
	raw, kf8, offset, err := extractMobi8Raw(&PdbDb{}, &Mobi{KF8: &Mobi{}}, -1)
	if err == nil {
		t.Fatalf("extractMobi8Raw() = (%v, %v, %d, nil), want an error", raw, kf8, offset)
	}
	if !errors.Is(kf8ContentError(err), book.ErrCorrupt) {
		t.Errorf("kf8ContentError(%v) is not ErrCorrupt", err)
	}
}

func TestKF8IndexFailureStillServesRawContent(t *testing.T) {
	// A broken index does not cost the reader the content: the decompressed
	// markup is still served as a single resource.
	blob := makeStandaloneKF8Blob(t, func(records *[]recordSpec, opts *mobiHeaderOpts) {
		(*records)[6] = recordSpec{data: []byte("not an index at all")}
	})

	got, err := NewMobiReader(config.DefaultConfig()).Read(book.NewBytesBlob(blob))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	content := got.Resources().ReadingOrder()
	if len(content) != 1 {
		t.Fatalf("reading order = %d resources, want 1", len(content))
	}
	if body := readAllResource(t, *content[0].Resource); !strings.Contains(body, "hello") {
		t.Errorf("content = %q, want the raw markup", body)
	}
}

func TestKF8DRMIsReported(t *testing.T) {
	// DRM on the KF8 header must surface as ErrDRM and never fall back.
	blob := makeStandaloneKF8Blob(t, func(records *[]recordSpec, opts *mobiHeaderOpts) {
		opts.drmOffset = 0
	})

	_, err := NewMobiReader(config.DefaultConfig()).Read(book.NewBytesBlob(blob))
	if !errors.Is(err, book.ErrDRM) {
		t.Fatalf("Read() error = %v, want ErrDRM", err)
	}
}

func TestKF8DualDRMIsReported(t *testing.T) {
	blob := makeDualKF8Blob(t, func(records *[]recordSpec, opts *mobiHeaderOpts) {
		opts.encryption = EncryptionMobi
	})

	_, err := NewMobiReader(config.DefaultConfig()).Read(book.NewBytesBlob(blob))
	if !errors.Is(err, book.ErrDRM) {
		t.Fatalf("Read() error = %v, want ErrDRM", err)
	}
}

func TestMalformedMOBIReturnsSentinelErrors(t *testing.T) {
	t.Run("truncated record 0 is ErrParseFailed", func(t *testing.T) {
		blob := makePdbBlob(t, "TestShort", []recordSpec{{data: []byte("MOBI")}})
		_, err := NewMobiReader(config.DefaultConfig()).Read(book.NewBytesBlob(blob))
		if !errors.Is(err, book.ErrParseFailed) {
			t.Errorf("Read() error = %v, want ErrParseFailed", err)
		}
	})

	t.Run("file shorter than the PDB header is ErrCorrupt", func(t *testing.T) {
		_, err := NewMobiReader(config.DefaultConfig()).Read(book.NewBytesBlob([]byte("BOOKMOBI")))
		if !errors.Is(err, book.ErrCorrupt) {
			t.Errorf("Read() error = %v, want ErrCorrupt", err)
		}
	})
}

func TestKF8ResourceLimitOnContent(t *testing.T) {
	blob := makeStandaloneKF8Blob(t, nil)

	cfg := config.DefaultConfig()
	cfg.MaxResourceSize = 8 // smaller than the text flow
	got, err := NewMobiReader(cfg).Read(book.NewBytesBlob(blob))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	content := got.Resources().ReadingOrder()
	if len(content) != 1 {
		t.Fatalf("reading order = %d resources, want 1", len(content))
	}
	_, openErr := content[0].Resource.Open()
	if !errors.Is(openErr, book.ErrLimitExceeded) {
		t.Errorf("Open() error = %v, want ErrLimitExceeded", openErr)
	}
}

func TestReadIndexParsesSkeletonEntries(t *testing.T) {
	skel := makeIndxSection(skelTags, []indxEntry{{
		ident:  "SKEL0000000000",
		values: map[indxTagRef][]int{0: {3}, 1: {100, 20}},
	}})
	sections := [][]byte{skel[0], skel[1]}

	table, ordered, _, err := ReadIndex(sections, 0, "utf-8")
	if err != nil {
		t.Fatalf("ReadIndex() error: %v", err)
	}
	if len(ordered) != 1 || ordered[0] != "SKEL0000000000" {
		t.Fatalf("ordered = %v, want [SKEL0000000000]", ordered)
	}
	tagMap := table["SKEL0000000000"]
	if got := tagMap[1]; len(got) != 1 || got[0] != 3 {
		t.Errorf("tag 1 (div count) = %v, want [3]", got)
	}
	if got := tagMap[6]; len(got) != 2 || got[0] != 100 || got[1] != 20 {
		t.Errorf("tag 6 (start, length) = %v, want [100 20]", got)
	}
}

func TestReadIndexUsesCNCXForTocText(t *testing.T) {
	// The DIV table references its toc text through CNCX: the string at offset
	// 0 of the first CNCX record is what tag 2 points at.
	cncx := makeCNCXRecord("<a id=\"x\">")
	div := makeIndxSection(divTags, []indxEntry{{
		ident:  "0",
		values: map[indxTagRef][]int{0: {0}, 1: {0}, 3: {10, 5}},
	}})

	// Layout: 0 = DIV header (ncncx=1), 1 = entry, 2 = CNCX record.
	header := makeIndxHeaderRecord(divTags, 1)
	// Rebuild the header with ncncx set to 1, as a DIV table with toc text has.
	headerRec := makeIndxEntryRecord(divTags, indxEntry{})
	_ = headerRec
	sections := [][]byte{setNcncx(header, 1), div[1], cncx}

	table, ordered, gotCNCX, err := ReadIndex(sections, 0, "utf-8")
	if err != nil {
		t.Fatalf("ReadIndex() error: %v", err)
	}
	if len(ordered) != 1 {
		t.Fatalf("ordered = %v, want one entry", ordered)
	}
	if s, ok := gotCNCX[0]; !ok || s != "<a id=\"x\">" {
		t.Errorf("cncx[0] = %q (%v), want the toc text", s, ok)
	}
	if got := table["0"][2]; len(got) != 1 || got[0] != 0 {
		t.Errorf("tag 2 (cncx ref) = %v, want [0]", got)
	}
}

// setNcncx patches the ncncx field (index 12 of the INDX header fields) of an
// INDX record.
func setNcncx(rec []byte, n int) []byte {
	out := append([]byte{}, rec...)
	binaryPutUint32(out, 4+12*4, uint32(n))
	return out
}

func TestLocateBegEndOfTag(t *testing.T) {
	ml := []byte(`<html><body><div aid="a1">x</div><p AID='a2'>y</p></body></html>`)

	begin, end := locateBegEndOfTag(ml, []byte("a1"))
	if got := string(ml[begin : end+1]); got != `<div aid="a1">` {
		t.Errorf("locateBegEndOfTag(a1) spans %q, want the div tag", got)
	}

	begin, end = locateBegEndOfTag(ml, []byte("a2"))
	if got := string(ml[begin : end+1]); got != `<p AID='a2'>` {
		t.Errorf("locateBegEndOfTag(a2) spans %q, want the p tag", got)
	}

	if b, e := locateBegEndOfTag(ml, []byte("missing")); b != 0 || e != 0 {
		t.Errorf("locateBegEndOfTag(missing) = (%d, %d), want (0, 0)", b, e)
	}
}

func TestBuildMobi8PartsNoSkeleton(t *testing.T) {
	// Without SKEL the whole flow becomes one part (calibre's fallback).
	parts, infos, err := buildMobi8Parts(kf8Text, nil, nil, nil)
	if err != nil {
		t.Fatalf("buildMobi8Parts() error: %v", err)
	}
	if len(parts) != 1 || len(infos) != 1 {
		t.Fatalf("parts = %d infos = %d, want 1/1", len(parts), len(infos))
	}
	if string(parts[0]) != string(kf8Text) {
		t.Errorf("parts[0] = %q, want the whole flow", parts[0])
	}
	if infos[0].Filename != "part0000.html" {
		t.Errorf("filename = %q, want part0000.html", infos[0].Filename)
	}
}

func TestBuildMobi8PartsClampsOutOfRangePositions(t *testing.T) {
	// Corrupt start/length pairs must be clamped, not panic.
	files := []FileInfo{{FileNumber: 0, DivCount: 1, StartPos: 3, Length: 1 << 20}}
	elems := []Elem{{InsertPos: 1 << 20, StartPos: 0, Length: 1 << 20}}
	parts, _, err := buildMobi8Parts(kf8Text, nil, files, elems)
	if err != nil {
		t.Fatalf("buildMobi8Parts() error: %v", err)
	}
	if len(parts) != 1 {
		t.Fatalf("parts = %d, want 1", len(parts))
	}
}

func TestParseFDSTValidAndMalformed(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		flows, err := parseFDST(makeFDSTRecord([][2]int{{0, 10}, {10, 20}}))
		if err != nil {
			t.Fatalf("parseFDST() error: %v", err)
		}
		if len(flows) != 2 || flows[0] != [2]int{0, 10} || flows[1] != [2]int{10, 20} {
			t.Errorf("flows = %v, want [[0 10] [10 20]]", flows)
		}
	})

	t.Run("malformed section count errors instead of panicking", func(t *testing.T) {
		if _, err := parseFDST(makeMalformedFDSTRecord()); err == nil {
			t.Error("parseFDST() = nil error, want a bounds error")
		}
	})

	t.Run("not FDST", func(t *testing.T) {
		if _, err := parseFDST([]byte("NOPE00000000")); err == nil {
			t.Error("parseFDST() = nil error, want a signature error")
		}
	})

	t.Run("zero sections", func(t *testing.T) {
		data := make([]byte, 12)
		copy(data[0:4], "FDST")
		binaryPutUint32(data, 4, 12)
		flows, err := parseFDST(data)
		if err != nil || flows != nil {
			t.Errorf("parseFDST() = (%v, %v), want (nil, nil)", flows, err)
		}
	})
}

func TestParseCNCX(t *testing.T) {
	rec := makeCNCXRecord("alpha", "beta")
	cncx, err := parseCNCX([][]byte{rec}, "utf-8")
	if err != nil {
		t.Fatalf("parseCNCX() error: %v", err)
	}
	if s, ok := cncx[0]; !ok || s != "alpha" {
		t.Errorf("cncx[0] = %q (%v), want alpha", s, ok)
	}
	alphaEnd := len(encVarint(len("alpha"))) + len("alpha")
	if s, ok := cncx[alphaEnd]; !ok || s != "beta" {
		t.Errorf("cncx[%d] = %q (%v), want beta", alphaEnd, s, ok)
	}

	// A second record is offset by 0x10000, like calibre's CNCX.
	cncx, err = parseCNCX([][]byte{rec, makeCNCXRecord("gamma")}, "utf-8")
	if err != nil {
		t.Fatalf("parseCNCX() error: %v", err)
	}
	if s, ok := cncx[0x10000]; !ok || s != "gamma" {
		t.Errorf("cncx[0x10000] = %q (%v), want gamma", s, ok)
	}

	if _, err := parseCNCX([][]byte{{0x7F, 'a'}}, "utf-8"); err != nil {
		t.Errorf("parseCNCX(truncated) error = %v, want nil (calibre skips it)", err)
	}
}

func TestKF8NoPanicOnMalformedInput(t *testing.T) {
	// Every entry point that indexes into raw file data must return an error
	// instead of letting a panic escape to the caller.
	cases := []struct {
		name string
		blob []byte
	}{
		{"malformed FDST", makeStandaloneKF8Blob(t, func(records *[]recordSpec, opts *mobiHeaderOpts) {
			(*records)[2] = recordSpec{data: makeMalformedFDSTRecord()}
		})},
		{"corrupt text data", makeStandaloneKF8Blob(t, func(records *[]recordSpec, opts *mobiHeaderOpts) {
			(*records)[1] = recordSpec{data: []byte{0xFF, 0xFF, 0xFF}}
		})},
		{"broken DIV index", makeStandaloneKF8Blob(t, func(records *[]recordSpec, opts *mobiHeaderOpts) {
			(*records)[6] = recordSpec{data: []byte("not an index")}
		})},
		{"index records missing", makeStandaloneKF8Blob(t, func(records *[]recordSpec, opts *mobiHeaderOpts) {
			opts.skelIdx = NullIndex
			opts.divIdx = NullIndex
		})},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("Read() panicked: %v", r)
					}
				}()
				ebook, err := NewMobiReader(config.DefaultConfig()).Read(book.NewBytesBlob(tc.blob))
				if err != nil {
					return // an error is a valid outcome
				}
				for _, res := range ebook.Resources().ReadingOrder() {
					if _, openErr := res.Resource.Open(); openErr != nil {
						continue
					}
				}
			}()
		})
	}
}
