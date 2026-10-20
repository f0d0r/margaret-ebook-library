package lrf

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

const (
	testBookAttrID   = 0x20
	testPageTreeID   = 0x21
	testPage1ID      = 0x22
	testPage2ID      = 0x23
	testBlock1ID     = 0x24
	testBlock2ID     = 0x25
	testBlock3ID     = 0x29
	testImageBlockID = 0x26
	testText1ID      = 0x27
	testText2ID      = 0x28
	testText3ID      = 0x2A
	testImageID      = 0x30
)

func joinBytes(chunks ...[]byte) []byte {
	var out []byte
	for _, c := range chunks {
		out = append(out, c...)
	}
	return out
}

// tTextSpan encodes an F5CC text string token: a u16 byte count followed by
// UTF-16LE text.
func tTextSpan(s string) []byte {
	text := utf16LEBytes(s)
	payload := append([]byte{byte(len(text)), byte(len(text) >> 8)}, text...)
	return tTag(tagTextString, payload...)
}

func tPara() []byte    { return tTag(tagTextStartPara, 0, 0, 0, 0) }
func tEndPara() []byte { return tTag(0xF5A2) }
func tCR() []byte      { return tTag(tagTextCR) }

// contentObjects builds a BookAttr -> PageTree -> Page -> Block -> Text
// chain around one text object.
func contentObjects(textID uint32, textType uint16, textFlags uint16, textStream []byte) []testObject {
	return []testObject{
		{testBookAttrID, objectTypeBookAttr, tDword(tagBookPageTree, testPageTreeID)},
		{testPageTreeID, objectTypePageTree, tTypeOne(tagPageTreePages, testPage1ID)},
		{testPage1ID, objectTypePage, joinBytes(tDword(tagLink, 0xA0), tStream(0, tDword(tagLink, testBlock1ID)))},
		{testBlock1ID, objectTypeBlock, joinBytes(tDword(tagLink, 0xA1), tStream(0, tDword(tagLink, textID)))},
		{textID, textType, joinBytes(tDword(tagLink, 0xA2), tStream(textFlags, textStream))},
	}
}

// multiPageObjects builds a two-page book: the first page holds a text block
// and an image block, the second page a SimpleText block.
func multiPageObjects(textStream, simpleStream []byte) []testObject {
	imageObject := testObject{testImageID, objectTypeImage, tDword(0xF54C, 0x31)}
	objects := []testObject{
		{testBookAttrID, objectTypeBookAttr, tDword(tagBookPageTree, testPageTreeID)},
		{testPageTreeID, objectTypePageTree, tTypeOne(tagPageTreePages, testPage1ID, testPage2ID)},
		{testPage1ID, objectTypePage, joinBytes(tDword(tagLink, 0xA0), tStream(0, joinBytes(tDword(tagLink, testBlock1ID), tDword(tagLink, testImageBlockID))))},
		{testPage2ID, objectTypePage, joinBytes(tDword(tagLink, 0xA0), tStream(0, tDword(tagLink, testBlock2ID)))},
		{testBlock1ID, objectTypeBlock, joinBytes(tDword(tagLink, 0xA1), tStream(0, tDword(tagLink, testText1ID)))},
		{testImageBlockID, objectTypeBlock, joinBytes(tDword(tagLink, 0xA1), tStream(0, tDword(tagLink, testImageID)))},
		{testBlock2ID, objectTypeBlock, joinBytes(tDword(tagLink, 0xA1), tStream(0, tDword(tagLink, testText2ID)))},
		{testText1ID, objectTypeText, joinBytes(tDword(tagLink, 0xA2), tStream(0, textStream))},
		{testText2ID, objectTypeSimpleText, joinBytes(tDword(tagLink, 0xA3), tStream(0, simpleStream))},
		imageObject,
	}
	return objects
}

func readContentResource(t *testing.T, cfg config.Config, data []byte) (*book.Resource, bool) {
	t.Helper()
	b, err := NewLrfReader(cfg).Read(book.NewBytesBlob(data))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	return b.Resources().GetByID(contentID)
}

func openContent(t *testing.T, r *book.Resource) []byte {
	t.Helper()
	got, err := r.Data()
	if err != nil {
		t.Fatalf("content Data() error: %v", err)
	}
	return got
}

func TestReadContentEndToEnd(t *testing.T) {
	textStream := joinBytes(
		tPara(),
		tTextSpan("Hello "),
		utf16LEBytes("world"),
		tCR(),
		tTag(0xF515, 0x01, 0x00), // fontweight: skipped
		tTextSpan("again"),
		tEndPara(),
	)
	want := joinBytes(paraUTF16LE, utf16LEBytes("Hello world"), crUTF16LE, utf16LEBytes("again"))

	data := buildLrfBook(t, testBookAttrID, contentObjects(testText1ID, objectTypeText, 0, textStream)...)
	content, ok := readContentResource(t, config.DefaultConfig(), data)
	if !ok {
		t.Fatal("content resource not found")
	}
	if content.Id != contentID || content.Name != contentName {
		t.Errorf("content = %q/%q, want %q/%q", content.Id, content.Name, contentID, contentName)
	}
	if content.MediaType != mediatype.LRFText {
		t.Errorf("MediaType = %q, want %q", content.MediaType, mediatype.LRFText)
	}
	if content.Charset != contentCharset {
		t.Errorf("Charset = %q, want %q", content.Charset, contentCharset)
	}
	if content.Size != 0 {
		t.Errorf("Size = %d, want 0", content.Size)
	}
	if got := openContent(t, content); !bytes.Equal(got, want) {
		t.Errorf("content = % X, want % X", got, want)
	}
}

func TestReadContentCompressed(t *testing.T) {
	textStream := joinBytes(tPara(), tTextSpan("compressed"))
	payload := zlibStreamPayload(t, textStream)
	want := joinBytes(paraUTF16LE, utf16LEBytes("compressed"))

	data := buildLrfBook(t, testBookAttrID, contentObjects(testText1ID, objectTypeText, streamFlagCompressed, payload)...)
	content, ok := readContentResource(t, config.DefaultConfig(), data)
	if !ok {
		t.Fatal("content resource not found")
	}
	if got := openContent(t, content); !bytes.Equal(got, want) {
		t.Errorf("content = % X, want % X", got, want)
	}
}

func TestReadContentScrambled(t *testing.T) {
	textStream := joinBytes(tPara(), tTextSpan("scrambled text"))
	want := joinBytes(paraUTF16LE, utf16LEBytes("scrambled text"))
	const xorKey = 0x0045
	payload := scrambleTestBytes(textStream, xorKey, 0)

	data := buildLrfBook(t, testBookAttrID, contentObjects(testText1ID, objectTypeText, streamFlagScrambled, payload)...)
	binary.LittleEndian.PutUint16(data[offXorKey:], xorKey)

	content, ok := readContentResource(t, config.DefaultConfig(), data)
	if !ok {
		t.Fatal("content resource not found")
	}
	if got := openContent(t, content); !bytes.Equal(got, want) {
		t.Errorf("content = % X, want % X", got, want)
	}
}

func TestReadContentMultiplePages(t *testing.T) {
	textStream := joinBytes(tRaw("First page"), tCR())
	simpleStream := joinBytes(tRaw("Second page"))
	want := joinBytes(utf16LEBytes("First page"), crUTF16LE, utf16LEBytes("Second page"))

	data := buildLrfBook(t, testBookAttrID, multiPageObjects(textStream, simpleStream)...)
	content, ok := readContentResource(t, config.DefaultConfig(), data)
	if !ok {
		t.Fatal("content resource not found")
	}
	if got := openContent(t, content); !bytes.Equal(got, want) {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func tRaw(s string) []byte { return utf16LEBytes(s) }

func TestReadContentNonBlockPageObject(t *testing.T) {
	// Pages may link non-Block objects (here a Canvas, as LRF writers use
	// for cover pages); they carry no text and must not break extraction.
	const canvasID = 0xC1
	textStream := joinBytes(tRaw("body"))
	want := utf16LEBytes("body")
	objects := []testObject{
		{testBookAttrID, objectTypeBookAttr, tDword(tagBookPageTree, testPageTreeID)},
		{testPageTreeID, objectTypePageTree, tTypeOne(tagPageTreePages, testPage1ID)},
		{testPage1ID, objectTypePage, joinBytes(tDword(tagLink, 0xA0), tStream(0, joinBytes(
			tDword(tagLink, canvasID), tDword(tagLink, testBlock1ID))))},
		{canvasID, objectTypeCanvas, tStream(0, nil)},
		{testBlock1ID, objectTypeBlock, joinBytes(tDword(tagLink, 0xA1), tStream(0, tDword(tagLink, testText1ID)))},
		{testText1ID, objectTypeText, joinBytes(tDword(tagLink, 0xA2), tStream(0, textStream))},
	}

	data := buildLrfBook(t, testBookAttrID, objects...)
	content, ok := readContentResource(t, config.DefaultConfig(), data)
	if !ok {
		t.Fatal("content resource not found")
	}
	if got := openContent(t, content); !bytes.Equal(got, want) {
		t.Errorf("content = % X, want % X", got, want)
	}
}

func TestReadContentCanvasText(t *testing.T) {
	// A page-level Canvas references its Blocks through PutObj tags; their
	// text belongs to the page flow in PutObj order.
	const (
		canvasID = 0xC1
		imageID  = 0xC2
	)
	first := joinBytes(tRaw("first"))
	second := joinBytes(tRaw("second"))
	want := joinBytes(utf16LEBytes("first"), utf16LEBytes("tail"), utf16LEBytes("second"))
	objects := []testObject{
		{testBookAttrID, objectTypeBookAttr, tDword(tagBookPageTree, testPageTreeID)},
		{testPageTreeID, objectTypePageTree, tTypeOne(tagPageTreePages, testPage1ID)},
		{testPage1ID, objectTypePage, joinBytes(tDword(tagLink, 0xA0), tStream(0, joinBytes(
			tDword(tagLink, canvasID), tDword(tagLink, testBlock1ID))))},
		{canvasID, objectTypeCanvas, tStream(0, joinBytes(
			tPutObj(10, 20, testBlock2ID), tPutObj(30, 40, imageID), tPutObj(50, 60, testBlock3ID)))},
		{testBlock1ID, objectTypeBlock, joinBytes(tDword(tagLink, 0xA1), tStream(0, tDword(tagLink, testText1ID)))},
		{testBlock2ID, objectTypeBlock, joinBytes(tDword(tagLink, 0xA1), tStream(0, tDword(tagLink, testText2ID)))},
		{testBlock3ID, objectTypeBlock, joinBytes(tDword(tagLink, 0xA1), tStream(0, tDword(tagLink, testText3ID)))},
		{imageID, objectTypeImage, tDword(0xF54C, 0x31)},
		{testText1ID, objectTypeText, joinBytes(tDword(tagLink, 0xA2), tStream(0, second))},
		{testText2ID, objectTypeText, joinBytes(tDword(tagLink, 0xA2), tStream(0, first))},
		{testText3ID, objectTypeText, joinBytes(tDword(tagLink, 0xA2), tStream(0, joinBytes(tRaw("tail"))))},
	}

	data := buildLrfBook(t, testBookAttrID, objects...)
	content, ok := readContentResource(t, config.DefaultConfig(), data)
	if !ok {
		t.Fatal("content resource not found")
	}
	// Page order: Canvas first (its blocks in PutObj order), then the
	// page-level block.
	if got := openContent(t, content); !bytes.Equal(got, want) {
		t.Errorf("content = % X, want % X", got, want)
	}
}

func TestReadContentPlot(t *testing.T) {
	plot := []byte{0x10, 0x00, 0x20, 0x00, 0x10, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00}
	textStream := joinBytes(tPara(), tTextSpan("before"), tTag(tagTextPlot, plot...), tTextSpan("after"))
	want := joinBytes(paraUTF16LE, utf16LEBytes("before"), plotUTF16LE, utf16LEBytes("after"))

	data := buildLrfBook(t, testBookAttrID, contentObjects(testText1ID, objectTypeText, 0, textStream)...)
	content, ok := readContentResource(t, config.DefaultConfig(), data)
	if !ok {
		t.Fatal("content resource not found")
	}
	if got := openContent(t, content); !bytes.Equal(got, want) {
		t.Errorf("content = % X, want % X", got, want)
	}
}

func TestReadContentNonTagF5Pair(t *testing.T) {
	// 0x0C is not a text stream tag, so an F5 pair involving it is raw text.
	textStream := joinBytes(tRaw("ok"), []byte{0x0C, 0xF5}, tRaw("!"))
	want := joinBytes(utf16LEBytes("ok"), []byte{0x0C, 0xF5}, utf16LEBytes("!"))

	data := buildLrfBook(t, testBookAttrID, contentObjects(testText1ID, objectTypeText, 0, textStream)...)
	content, ok := readContentResource(t, config.DefaultConfig(), data)
	if !ok {
		t.Fatal("content resource not found")
	}
	if got := openContent(t, content); !bytes.Equal(got, want) {
		t.Errorf("content = % X, want % X", got, want)
	}
}

func TestReadContentNoRoot(t *testing.T) {
	objects := contentObjects(testText1ID, objectTypeText, 0, tRaw("text"))
	data := buildLrfBook(t, 0, objects...)

	_, ok := readContentResource(t, config.DefaultConfig(), data)
	if ok {
		t.Error("GetByID(content) = ok for a file without root object")
	}
}

func TestReadContentErrors(t *testing.T) {
	text := func(stream []byte) []testObject {
		return contentObjects(testText1ID, objectTypeText, 0, stream)
	}
	compressed := func(stream []byte) []testObject {
		return contentObjects(testText1ID, objectTypeText, streamFlagCompressed, stream)
	}
	tests := []struct {
		name  string
		root  uint32
		objs  []testObject
		match error
	}{
		{
			name:  "root object missing",
			root:  0x99,
			objs:  text(tRaw("text")),
			match: book.ErrCorrupt,
		},
		{
			name:  "root object is not a BookAttr",
			root:  testPage1ID,
			objs:  text(tRaw("text")),
			match: book.ErrCorrupt,
		},
		{
			name: "book attr without page tree",
			root: testBookAttrID,
			objs: []testObject{
				{testBookAttrID, objectTypeBookAttr, nil},
			},
			match: book.ErrCorrupt,
		},
		{
			name: "page tree without page list",
			root: testBookAttrID,
			objs: []testObject{
				{testBookAttrID, objectTypeBookAttr, tDword(tagBookPageTree, testPageTreeID)},
				{testPageTreeID, objectTypePageTree, tDword(tagLink, 1)},
			},
			match: book.ErrCorrupt,
		},
		{
			name: "page list points at a non-page",
			root: testBookAttrID,
			objs: []testObject{
				{testBookAttrID, objectTypeBookAttr, tDword(tagBookPageTree, testPageTreeID)},
				{testPageTreeID, objectTypePageTree, tTypeOne(tagPageTreePages, testText1ID)},
				{testText1ID, objectTypeText, tStream(0, tRaw("text"))},
			},
			match: book.ErrCorrupt,
		},
		{
			name: "block without content link",
			root: testBookAttrID,
			objs: []testObject{
				{testBookAttrID, objectTypeBookAttr, tDword(tagBookPageTree, testPageTreeID)},
				{testPageTreeID, objectTypePageTree, tTypeOne(tagPageTreePages, testPage1ID)},
				{testPage1ID, objectTypePage, joinBytes(tDword(tagLink, 0xA0), tStream(0, tDword(tagLink, testBlock1ID)))},
				{testBlock1ID, objectTypeBlock, joinBytes(tDword(tagLink, 0xA1), tStream(0, tDword(tagLink, 0x99)))},
			},
			match: book.ErrCorrupt,
		},
		{
			name:  "truncated text string",
			root:  testBookAttrID,
			objs:  text(tTag(tagTextString, 0x40, 0x00, 'a', 0)),
			match: book.ErrCorrupt,
		},
		{
			name:  "truncated plot payload",
			root:  testBookAttrID,
			objs:  text(tTag(tagTextPlot, 0x01)),
			match: book.ErrCorrupt,
		},
		{
			name:  "malformed text tag",
			root:  testBookAttrID,
			objs:  text(tTag(0xF578)), // tag_78 without its contents
			match: book.ErrCorrupt,
		},
		{
			name: "compressed size mismatch",
			root: testBookAttrID,
			objs: compressed(func() []byte {
				bad := zlibStreamPayload(t, []byte("data"))
				binary.LittleEndian.PutUint32(bad[:4], 999)
				return bad
			}()),
			match: book.ErrCorrupt,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := buildLrfBook(t, tt.root, tt.objs...)
			content, ok := readContentResource(t, config.DefaultConfig(), data)
			if !ok {
				t.Fatal("content resource not found")
			}
			if _, err := content.Open(); !errors.Is(err, tt.match) {
				t.Errorf("Open() error = %v, want %v", err, tt.match)
			}
		})
	}
}

func TestReadContentLimit(t *testing.T) {
	long := strings.Repeat("x", 20)
	textStream := tRaw(long)
	objs := []testObject{
		{testBookAttrID, objectTypeBookAttr, tDword(tagBookPageTree, testPageTreeID)},
		{testPageTreeID, objectTypePageTree, tTypeOne(tagPageTreePages, testPage1ID)},
		{testPage1ID, objectTypePage, joinBytes(tDword(tagLink, 0xA0), tStream(0, joinBytes(
			tDword(tagLink, testBlock1ID), tDword(tagLink, testBlock2ID), tDword(tagLink, testImageBlockID))))},
		{testBlock1ID, objectTypeBlock, joinBytes(tDword(tagLink, 0xA1), tStream(0, tDword(tagLink, testText1ID)))},
		{testBlock2ID, objectTypeBlock, joinBytes(tDword(tagLink, 0xA1), tStream(0, tDword(tagLink, testText1ID)))},
		{testImageBlockID, objectTypeBlock, joinBytes(tDword(tagLink, 0xA1), tStream(0, tDword(tagLink, testText1ID)))},
		{testText1ID, objectTypeText, joinBytes(tDword(tagLink, 0xA2), tStream(0, textStream))},
	}
	data := buildLrfBook(t, testBookAttrID, objs...)

	content, ok := readContentResource(t, config.Config{MaxResourceSize: 100}, data)
	if !ok {
		t.Fatal("content resource not found")
	}
	if _, err := content.Open(); !errors.Is(err, book.ErrLimitExceeded) {
		t.Errorf("Open() error = %v, want book.ErrLimitExceeded", err)
	}
}
