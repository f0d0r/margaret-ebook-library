package lrf

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

func objectTableBlob(size int, nobj, otab uint64) []byte {
	data := make([]byte, size)
	copy(data, lrfMagic)
	binary.LittleEndian.PutUint16(data[offVersion:], 1000)
	binary.LittleEndian.PutUint64(data[offNumberOfObjects:], nobj)
	binary.LittleEndian.PutUint64(data[offObjectTableOffset:], otab)
	return data
}

func TestReadObjectTable(t *testing.T) {
	data := objectTableBlob(0x78, 2, 0x58)

	entry := data[0x58:]
	binary.LittleEndian.PutUint32(entry[0:], 0x11)
	binary.LittleEndian.PutUint32(entry[4:], 0x1A6)
	binary.LittleEndian.PutUint32(entry[8:], 0x66)
	binary.LittleEndian.PutUint32(entry[12:], 0)
	binary.LittleEndian.PutUint32(entry[16:], 0x2C)
	binary.LittleEndian.PutUint32(entry[20:], 0x40FE9)
	binary.LittleEndian.PutUint32(entry[24:], 4739)
	binary.LittleEndian.PutUint32(entry[28:], 0xDEADBEEF)

	r := NewLrfReader(config.DefaultConfig())
	blob := book.NewBytesBlob(data)
	header, err := r.readHeader(blob)
	if err != nil {
		t.Fatal(err)
	}
	refs, err := r.readObjectTable(blob, header)
	if err != nil {
		t.Fatal(err)
	}
	want := []LrfObjectRef{
		{id: 0x11, offset: 0x1A6, size: 0x66},
		{id: 0x2C, offset: 0x40FE9, size: 4739},
	}
	if len(refs) != len(want) {
		t.Fatalf("len(refs) = %d, want %d", len(refs), len(want))
	}
	for i := range want {
		if refs[i] != want[i] {
			t.Errorf("refs[%d] = %+v, want %+v", i, refs[i], want[i])
		}
	}
}

func TestReadObjectTableErrors(t *testing.T) {
	tests := []struct {
		name string
		size int
		nobj uint64
		otab uint64
	}{
		{"count exceeds file", 0x60, 100, 0},
		{"offset beyond file", 0x60, 1, 0x1000},
		{"table end beyond file", 0x60, 2, 0x58},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewLrfReader(config.DefaultConfig())
			blob := book.NewBytesBlob(objectTableBlob(tt.size, tt.nobj, tt.otab))
			header, err := r.readHeader(blob)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := r.readObjectTable(blob, header); !errors.Is(err, book.ErrCorrupt) {
				t.Errorf("readObjectTable() error = %v, want book.ErrCorrupt", err)
			}
		})
	}
}

// lrfWithThumbnail builds a valid LRF file with the given thumbnail bytes
// directly after the info block.
func lrfWithThumbnail(t *testing.T, version uint16, thumbType uint16, thumb []byte) []byte {
	t.Helper()
	data := buildLrf(t, version, []byte("<Info/>"))
	data = append(data, thumb...)
	if version > 800 {
		binary.LittleEndian.PutUint16(data[offThumbnailType:], thumbType)
		binary.LittleEndian.PutUint32(data[offThumbnailSize:], uint32(len(thumb)))
	}
	return data
}

func readCoverResource(t *testing.T, cfg config.Config, data []byte) (*book.Resource, error) {
	t.Helper()
	r := NewLrfReader(cfg)
	blob := book.NewBytesBlob(data)
	header, err := r.readHeader(blob)
	if err != nil {
		t.Fatal(err)
	}
	return r.readCover(blob, header)
}

func TestReadCover(t *testing.T) {
	jpeg := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{0x42}, 60)...)

	// The header claims PNG, the bytes are JPEG: the sniff must win.
	cover, err := readCoverResource(t, config.DefaultConfig(), lrfWithThumbnail(t, 1000, 0x12, jpeg))
	if err != nil {
		t.Fatal(err)
	}
	if cover == nil {
		t.Fatal("readCover() = nil, want resource")
	}
	if cover.Id != "cover" || cover.Name != "cover.jpg" {
		t.Errorf("cover = %q/%q, want cover/cover.jpg", cover.Id, cover.Name)
	}
	if cover.MediaType != mediatype.JPEG {
		t.Errorf("MediaType = %q, want %q", cover.MediaType, mediatype.JPEG)
	}
	if cover.Properties != "cover-image" {
		t.Errorf("Properties = %q, want cover-image", cover.Properties)
	}
	if cover.Charset != book.CharsetBinary {
		t.Errorf("Charset = %q, want %q", cover.Charset, book.CharsetBinary)
	}
	if cover.Size != int64(len(jpeg)) {
		t.Errorf("Size = %d, want %d", cover.Size, len(jpeg))
	}
	got, err := cover.Data()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, jpeg) {
		t.Errorf("cover data = %d bytes, want %d identical bytes", len(got), len(jpeg))
	}
}

func TestReadCoverHeaderTypeFallback(t *testing.T) {
	// BMP is not recognized by the sniff, so the header type decides.
	bmp := append([]byte("BM"), bytes.Repeat([]byte{0x00}, 30)...)

	cover, err := readCoverResource(t, config.DefaultConfig(), lrfWithThumbnail(t, 1000, 0x13, bmp))
	if err != nil {
		t.Fatal(err)
	}
	if cover == nil {
		t.Fatal("readCover() = nil, want resource")
	}
	if cover.MediaType != mediatype.BMP || cover.Name != "cover.bmp" {
		t.Errorf("cover = %q/%q, want %q/cover.bmp", cover.MediaType, cover.Name, mediatype.BMP)
	}
}

func TestReadCoverNone(t *testing.T) {
	tests := []struct {
		name      string
		version   uint16
		thumbType uint16
		thumb     []byte
	}{
		{"no thumbnail", 1000, 0x11, nil},
		{"legacy version", 800, 0x11, []byte{0xFF, 0xD8, 0xFF, 0xE0}},
		{"unknown format", 1000, 0x00, []byte("garbage bytes here")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cover, err := readCoverResource(t, config.DefaultConfig(), lrfWithThumbnail(t, tt.version, tt.thumbType, tt.thumb))
			if err != nil {
				t.Fatal(err)
			}
			if cover != nil {
				t.Errorf("readCover() = %+v, want nil", cover)
			}
		})
	}
}

func TestReadCoverErrors(t *testing.T) {
	thumb := bytes.Repeat([]byte{0x42}, 32)

	t.Run("size beyond file", func(t *testing.T) {
		data := lrfWithThumbnail(t, 1000, 0x11, thumb)
		binary.LittleEndian.PutUint32(data[offThumbnailSize:], uint32(len(thumb)+100))

		_, err := readCoverResource(t, config.DefaultConfig(), data)
		if !errors.Is(err, book.ErrCorrupt) {
			t.Errorf("readCover() error = %v, want book.ErrCorrupt", err)
		}
	})

	t.Run("thumbnail without info block", func(t *testing.T) {
		data := lrfWithThumbnail(t, 1000, 0x11, thumb)
		binary.LittleEndian.PutUint16(data[offCompressedInfoSize:], 0)

		_, err := readCoverResource(t, config.DefaultConfig(), data)
		if !errors.Is(err, book.ErrCorrupt) {
			t.Errorf("readCover() error = %v, want book.ErrCorrupt", err)
		}
	})
}

func TestReadCoverLimit(t *testing.T) {
	thumb := bytes.Repeat([]byte{0x42}, 64)

	cover, err := readCoverResource(t, config.Config{MaxResourceSize: 16}, lrfWithThumbnail(t, 1000, 0x13, thumb))
	if err != nil {
		t.Fatal(err)
	}
	if cover == nil {
		t.Fatal("readCover() = nil, want resource")
	}
	if _, err := cover.Open(); !errors.Is(err, book.ErrLimitExceeded) {
		t.Errorf("Open() error = %v, want book.ErrLimitExceeded", err)
	}
}

func readResourceSet(t *testing.T, cfg config.Config, data []byte) *book.ResourceSet {
	t.Helper()
	b, err := NewLrfReader(cfg).Read(book.NewBytesBlob(data))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	return b.Resources()
}

func TestReadObjectsImagesAndFonts(t *testing.T) {
	jpeg := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{0x11}, 40)...)
	png := append([]byte{0x89, 'P', 'N', 'G'}, bytes.Repeat([]byte{0x22}, 20)...)
	ttf := append([]byte{0x00, 0x01, 0x00, 0x00}, bytes.Repeat([]byte{0x33}, 30)...)

	objects := []testObject{
		{0x10, objectTypeImageStream, tStream(0x11, jpeg)},
		{0x14, objectTypeImageStream, tStream(0x12, png)},
		{0x18, objectTypeFont, joinBytes(
			tString(tagFontFileName, "times.ttf"),
			tString(tagFontFaceName, "Times New Roman"),
			tStream(0, ttf),
		)},
		{0x19, 0x16, []byte{0x01, 0x02}},                             // Sound: skipped
		{0x1A, objectTypeImageStream, tStream(0x99, jpeg)},           // unknown image format: skipped
		{0x1B, objectTypeFont, tString(tagFontFaceName, "NoStream")}, // malformed font: skipped
	}
	rs := readResourceSet(t, config.DefaultConfig(), buildLrfBook(t, 0, objects...))

	all := rs.All()
	if len(all) != 3 {
		t.Fatalf("All() = %d resources, want 3: %v", len(all), all)
	}
	img := all[0]
	if img.Id != "image-16" || img.Name != "16.jpg" || img.Href != "images/16.jpg" {
		t.Errorf("jpeg resource = %q/%q/%q, want image-16/16.jpg/images/16.jpg", img.Id, img.Name, img.Href)
	}
	if img.MediaType != mediatype.JPEG || img.Charset != book.CharsetBinary {
		t.Errorf("jpeg media/charset = %q/%q, want %q/%q", img.MediaType, img.Charset, mediatype.JPEG, book.CharsetBinary)
	}
	if img.Size != int64(len(jpeg)) {
		t.Errorf("jpeg Size = %d, want %d", img.Size, len(jpeg))
	}
	if got, err := img.Data(); err != nil || !bytes.Equal(got, jpeg) {
		t.Errorf("jpeg Data() = %d bytes (err %v), want the identical %d bytes", len(got), err, len(jpeg))
	}
	if _, ok := rs.GetByHref("images/16.jpg"); !ok {
		t.Error("GetByHref(images/16.jpg) = false")
	}

	pngRes := all[1]
	if pngRes.MediaType != mediatype.PNG || pngRes.Name != "20.png" {
		t.Errorf("png resource = %q/%q, want png/20.png", pngRes.MediaType, pngRes.Name)
	}
	if got, err := pngRes.Data(); err != nil || !bytes.Equal(got, png) {
		t.Errorf("png Data() = %d bytes (err %v), want the identical %d bytes", len(got), err, len(png))
	}

	font := all[2]
	if font.Id != "font-24" || font.Name != "TimesNewRoman.ttf" || font.Href != "fonts/TimesNewRoman.ttf" {
		t.Errorf("font resource = %q/%q/%q, want font-24/TimesNewRoman.ttf/fonts/TimesNewRoman.ttf", font.Id, font.Name, font.Href)
	}
	if font.MediaType != mediatype.TTF || font.Charset != book.CharsetBinary {
		t.Errorf("font media/charset = %q/%q, want %q/%q", font.MediaType, font.Charset, mediatype.TTF, book.CharsetBinary)
	}
	if font.Size != int64(len(ttf)) {
		t.Errorf("font Size = %d, want %d", font.Size, len(ttf))
	}
	if got, err := font.Data(); err != nil || !bytes.Equal(got, ttf) {
		t.Errorf("font Data() = %d bytes (err %v), want the identical %d bytes", len(got), err, len(ttf))
	}
}

func TestReadObjectsFontNameFallbacks(t *testing.T) {
	ttf := bytes.Repeat([]byte{0x42}, 16)

	t.Run("file name", func(t *testing.T) {
		objects := []testObject{
			{0x30, objectTypeFont, joinBytes(tString(tagFontFileName, "fonts/arial.TTF"), tStream(0, ttf))},
		}
		rs := readResourceSet(t, config.DefaultConfig(), buildLrfBook(t, 0, objects...))
		if all := rs.All(); len(all) != 1 || all[0].Name != "arial.ttf" {
			t.Errorf("All() = %v, want a single arial.ttf font", all)
		}
	})

	t.Run("object id", func(t *testing.T) {
		objects := []testObject{
			{0x30, objectTypeFont, joinBytes(tString(tagFontFaceName, "///"), tStream(0, ttf))},
		}
		rs := readResourceSet(t, config.DefaultConfig(), buildLrfBook(t, 0, objects...))
		if all := rs.All(); len(all) != 1 || all[0].Name != "font-48.ttf" {
			t.Errorf("All() = %v, want a single font-48.ttf font", all)
		}
	})
}

func TestReadObjectsCompressedImage(t *testing.T) {
	jpeg := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{0x55}, 100)...)
	objects := []testObject{
		{0x10, objectTypeImageStream, tStream(streamFlagCompressed|0x11, zlibStreamPayload(t, jpeg))},
	}
	rs := readResourceSet(t, config.DefaultConfig(), buildLrfBook(t, 0, objects...))

	all := rs.All()
	if len(all) != 1 {
		t.Fatalf("All() = %d resources, want 1", len(all))
	}
	if all[0].Size != 0 {
		t.Errorf("Size = %d, want 0 for a compressed stream", all[0].Size)
	}
	if got, err := all[0].Data(); err != nil || !bytes.Equal(got, jpeg) {
		t.Errorf("Data() = %d bytes (err %v), want the identical %d bytes", len(got), err, len(jpeg))
	}
}

func TestReadObjectsScrambledImage(t *testing.T) {
	jpeg := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{0x66}, 30)...)
	const xorKey = 0x0042
	payload := scrambleTestBytes(jpeg, xorKey, scrambleMaxBytes)

	objects := []testObject{
		{0x10, objectTypeImageStream, tStream(streamFlagScrambled|0x11, payload)},
	}
	data := buildLrfBook(t, 0, objects...)
	binary.LittleEndian.PutUint16(data[offXorKey:], xorKey)

	rs := readResourceSet(t, config.DefaultConfig(), data)
	all := rs.All()
	if len(all) != 1 {
		t.Fatalf("All() = %d resources, want 1", len(all))
	}
	if got, err := all[0].Data(); err != nil || !bytes.Equal(got, jpeg) {
		t.Errorf("Data() = %d bytes (err %v), want the identical %d bytes", len(got), err, len(jpeg))
	}
}

func TestReadObjectsImageLimit(t *testing.T) {
	jpeg := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{0x77}, 100)...)
	objects := []testObject{
		{0x10, objectTypeImageStream, tStream(streamFlagCompressed|0x11, zlibStreamPayload(t, jpeg))},
	}
	rs := readResourceSet(t, config.Config{MaxResourceSize: 64}, buildLrfBook(t, 0, objects...))

	all := rs.All()
	if len(all) != 1 {
		t.Fatalf("All() = %d resources, want 1 (object sizes are under the limit)", len(all))
	}
	if _, err := all[0].Data(); !errors.Is(err, book.ErrLimitExceeded) {
		t.Errorf("Data() error = %v, want book.ErrLimitExceeded", err)
	}
}

func TestReadObjectsTableOrderAndContentFirst(t *testing.T) {
	textStream := joinBytes(tPara(), tTextSpan("body"))
	jpeg := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{0x88}, 16)...)

	objects := append(contentObjects(testText1ID, objectTypeText, 0, textStream),
		testObject{0x10, objectTypeImageStream, tStream(0x11, jpeg)})
	rs := readResourceSet(t, config.DefaultConfig(), buildLrfBook(t, testBookAttrID, objects...))

	all := rs.All()
	if len(all) != 2 {
		t.Fatalf("All() = %d resources, want 2", len(all))
	}
	if all[0].Id != contentID {
		t.Errorf("All()[0].Id = %q, want content first", all[0].Id)
	}
	if all[1].Id != "image-16" {
		t.Errorf("All()[1].Id = %q, want image-16", all[1].Id)
	}
	ro := rs.ReadingOrder()
	if len(ro) != 1 || ro[0].Resource.Id != contentID || !ro[0].Linear {
		t.Errorf("ReadingOrder() = %+v, want a single linear content item", ro)
	}
}

func TestReadObjectsNoRootNoReadingOrder(t *testing.T) {
	jpeg := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{0x11}, 16)...)
	objects := []testObject{
		{0x10, objectTypeImageStream, tStream(0x11, jpeg)},
	}
	rs := readResourceSet(t, config.DefaultConfig(), buildLrfBook(t, 0, objects...))

	if all := rs.All(); len(all) != 1 || all[0].Id != "image-16" {
		t.Errorf("All() = %v, want only image-16", all)
	}
	if ro := rs.ReadingOrder(); len(ro) != 0 {
		t.Errorf("ReadingOrder() = %+v, want empty without a root object", ro)
	}
}

func TestReadResourcesEndToEndWithCover(t *testing.T) {
	thumb := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{0x99}, 40)...)
	textStream := joinBytes(tPara(), tTextSpan("body"))
	jpeg := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{0xAA}, 16)...)

	data := buildLrf(t, 1000, []byte("<Info/>"))
	data = append(data, thumb...)
	binary.LittleEndian.PutUint16(data[offThumbnailType:], 0x11)
	binary.LittleEndian.PutUint32(data[offThumbnailSize:], uint32(len(thumb)))
	objects := append(contentObjects(testText1ID, objectTypeText, 0, textStream),
		testObject{0x10, objectTypeImageStream, tStream(0x11, jpeg)})
	data = appendObjectsAndTable(t, data, testBookAttrID, objects...)

	rs := readResourceSet(t, config.DefaultConfig(), data)
	all := rs.All()
	if len(all) != 3 {
		t.Fatalf("All() = %d resources, want 3", len(all))
	}
	if all[0].Id != contentID || all[1].Id != "image-16" || all[2].Id != "cover" {
		t.Errorf("All() ids = [%s %s %s], want [content image-16 cover]", all[0].Id, all[1].Id, all[2].Id)
	}
	cover, ok := rs.CoverImage()
	if !ok || cover != all[2] {
		t.Errorf("CoverImage() = %v/%v, want the cover resource", cover, ok)
	}
	ro := rs.ReadingOrder()
	if len(ro) != 1 || ro[0].Resource != all[0] || !ro[0].Linear {
		t.Errorf("ReadingOrder() = %+v, want only the linear content item", ro)
	}
}

func TestOpenReadingOrderAsPlainText(t *testing.T) {
	plot := []byte{0x10, 0x00, 0x20, 0x00, 0x10, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00}
	textStream := joinBytes(tPara(), tTextSpan("before"), tTag(tagTextPlot, plot...), tTextSpan(" after"))
	data := buildLrfBook(t, testBookAttrID, contentObjects(testText1ID, objectTypeText, 0, textStream)...)
	rs := readResourceSet(t, config.DefaultConfig(), data)

	rc, err := rs.OpenReadingOrderAs(context.Background(), mediatype.PlainText)
	if err != nil {
		t.Fatalf("OpenReadingOrderAs() error: %v", err)
	}
	defer func() { _ = rc.Close() }()
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	if want := "\n\nbefore after"; string(got) != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func TestReadCoverEndToEnd(t *testing.T) {
	jpeg := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{0x99}, 40)...)

	r := NewLrfReader(config.DefaultConfig())
	b, err := r.Read(book.NewBytesBlob(lrfWithThumbnail(t, 1000, 0x11, jpeg)))
	if err != nil {
		t.Fatal(err)
	}
	cover, ok := b.Resources().CoverImage()
	if !ok {
		t.Fatal("CoverImage() ok = false, want true")
	}
	if all := b.Resources().All(); len(all) != 1 || all[0] != cover {
		t.Errorf("All() = %v, want the cover resource only", all)
	}
	got, err := cover.Data()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, jpeg) {
		t.Errorf("cover data = %d bytes, want %d identical bytes", len(got), len(jpeg))
	}
}
