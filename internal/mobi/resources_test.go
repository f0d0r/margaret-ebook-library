package mobi

import (
	"encoding/binary"
	"errors"
	"io"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
)

func TestImageResources_CoverAliasIdentity(t *testing.T) {
	// Create PdbDb with 3 records: 0 header, 1 text, 2 image
	// Use Blob via memBlob helper (implement book.Blob)
	data := makeTestMobiBlobWithImage(t, 2, []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46})
	b := newMemBlob(data)
	cfg := config.DefaultConfig()
	r := NewMobiReader(cfg)
	// Manually craft Mobi with FirstImageRecord=2, LastContentRecord=2, CoverOffset=0
	exth := &Exth{Records: map[uint32][][]byte{}}
	coverOffset := make([]byte, 4)
	binary.BigEndian.PutUint32(coverOffset, 0)
	exth.Records[COVER_OFFSET] = [][]byte{coverOffset}
	mobi := &Mobi{
		FirstImageRecord:   2,
		LastContentRecord:  2,
		EXTH:               exth,
		recordCount:        3,
		MobiVersion:        6,
		FirstNonTextRecord: 1,
	}
	// Need TextRecordCount etc to avoid panic in CoverRecordIdx? Set FirstImageRecord check passes.
	pdbDb, err := ReadPdbDb(b, cfg.MaxRecordSize)
	if err != nil {
		t.Fatalf("ReadPdbDb: %v", err)
	}
	// Override mobi EXTH after read: read from pdb will parse EXTH empty, so set manually
	mobi.EXTH = exth
	// Recompute cover idx: FirstImageRecord + CoverOffset =2
	coverIdx := mobi.CoverRecordIdx()
	if coverIdx != 2 {
		t.Fatalf("CoverRecordIdx = %d, want 2", coverIdx)
	}
	// Use reader's build flow via Read() or directly via buildMobiResources
	// Build via full Read path with custom mobi not easy, so test imageResources + buildMobiResources alias logic
	// Create cover resource via reader.cover
	coverRes := r.cover(b, pdbDb, mobi)
	if coverRes == nil {
		t.Fatalf("cover should not be nil")
	}
	content := []book.Resource{{Name: "index.html", Id: "index", Href: "index.html", ResolvedHref: "index.html", MediaType: "application/x-mobipocket-html", Size: 10, Open: func() (io.ReadCloser, error) { return io.NopCloser(nil), nil }}}
	maxRes := cfg.MaxResourceSize
	maxRec := cfg.MaxRecordSize
	all, ro, coverAliased := r.buildMobiResources(b, pdbDb, mobi, coverRes, content, maxRes, maxRec)
	rs := book.NewResourceSet(all, ro, coverAliased)
	// After build, cover should be aliased to image resource
	coverViaRS, ok := rs.CoverImage()
	if !ok || coverViaRS == nil {
		t.Fatalf("CoverImage not found")
	}
	img, ok := rs.GetByHref("images/00002.jpg")
	if !ok || img == nil {
		t.Fatalf("image not found by href")
	}
	if coverViaRS != img {
		t.Errorf("cover alias failed: cover %p != image %p", coverViaRS, img)
	}
	// All should contain only one instance of that image (deduped)
	count := 0
	for _, res := range rs.All() {
		if res.ResolvedHref == "images/00002.jpg" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("All() should deduplicate cover/image, got count %d", count)
	}
	// All should also not contain duplicate cover.jpg
	for _, res := range rs.All() {
		if res.ResolvedHref == "cover.jpg" {
			t.Errorf("All() should not contain stale cover.jpg after alias, found %v", res)
		}
	}
}

func TestImageResources_NonImageSkipped(t *testing.T) {
	// Record with non-image magic should be skipped
	data := makeTestMobiBlobWithImage(t, 2, []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07})
	b := newMemBlob(data)
	cfg := config.DefaultConfig()
	mobi := &Mobi{FirstImageRecord: 2, LastContentRecord: 2, recordCount: 3}
	pdbDb, err := ReadPdbDb(b, cfg.MaxRecordSize)
	if err != nil {
		t.Fatalf("ReadPdbDb: %v", err)
	}
	images := imageResources(b, pdbDb, mobi, cfg.MaxResourceSize, cfg.MaxRecordSize)
	if len(images) != 0 {
		t.Fatalf("non-image record should be skipped, got %d images", len(images))
	}
}

func TestImageResources_MaxLimits(t *testing.T) {
	// Image of 20 bytes, set limits to 10 to trigger ErrLimitExceeded
	jpegHeader := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C}
	data := makeTestMobiBlobWithImage(t, 2, jpegHeader)
	b := newMemBlob(data)
	cfg := config.DefaultConfig()
	mobi := &Mobi{FirstImageRecord: 2, LastContentRecord: 2, recordCount: 3}
	pdbDb, _ := ReadPdbDb(b, cfg.MaxRecordSize)

	// MaxResourceSize exceeded
	images := imageResources(b, pdbDb, mobi, 10, cfg.MaxRecordSize)
	if len(images) != 1 {
		t.Fatalf("image should be included even if oversized, got %d", len(images))
	}
	_, err := images[0].resource.Open()
	if !errors.Is(err, book.ErrLimitExceeded) {
		t.Errorf("Open with MaxResourceSize exceeded should return ErrLimitExceeded, got %v", err)
	}

	// MaxRecordSize exceeded
	images2 := imageResources(b, pdbDb, mobi, cfg.MaxResourceSize, 10)
	if len(images2) != 1 {
		t.Fatalf("image should be included even if record oversized")
	}
	_, err = images2[0].resource.Open()
	if !errors.Is(err, book.ErrLimitExceeded) {
		t.Errorf("Open with MaxRecordSize exceeded should return ErrLimitExceeded, got %v", err)
	}

	// Both within limits should succeed
	images3 := imageResources(b, pdbDb, mobi, cfg.MaxResourceSize, cfg.MaxRecordSize)
	if len(images3) != 1 {
		t.Fatalf("expected 1 image")
	}
	rc, err := images3[0].resource.Open()
	if err != nil {
		t.Fatalf("Open within limits should not error, got %v", err)
	}
	_ = rc.Close()
}

// Helpers

type memBlob struct {
	data []byte
}

func newMemBlob(data []byte) book.Blob { return memBlob{data: data} }
func (m memBlob) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(m.data)) {
		return 0, io.EOF
	}
	n := copy(p, m.data[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}
func (m memBlob) Size() (int64, error) { return int64(len(m.data)), nil }

func makeTestMobiBlobWithImage(t *testing.T, imageRecIdx int, imageData []byte) []byte {
	t.Helper()
	const pdbHeaderSize = 78
	// Build minimal PDB with 3 records (0 header, 1 text, 2 image)
	header := make([]byte, pdbHeaderSize)
	copy(header[0:], "TestBook")
	binary.BigEndian.PutUint16(header[32:34], 0)
	binary.BigEndian.PutUint16(header[34:36], 0)
	binary.BigEndian.PutUint32(header[36:40], 2082844800+1000)
	binary.BigEndian.PutUint32(header[40:44], 2082844800+1000+86400)
	binary.BigEndian.PutUint32(header[44:48], 0)
	binary.BigEndian.PutUint32(header[48:52], 1)
	binary.BigEndian.PutUint32(header[52:56], 0)
	binary.BigEndian.PutUint32(header[56:60], 0)
	copy(header[60:64], "BOOK")
	copy(header[64:68], "MOBI")
	binary.BigEndian.PutUint32(header[68:72], 1)
	binary.BigEndian.PutUint32(header[72:76], 0)
	binary.BigEndian.PutUint16(header[76:78], 3)
	buf := append([]byte{}, header...)
	off0 := uint32(pdbHeaderSize + 3*8)
	// Need to compute offsets: record0 length 300 (mobi header), record1 length 20 (text), record2 = len(imageData)
	rec0Len := 300
	rec1Len := 20
	for i := 0; i < 3; i++ {
		ri := make([]byte, 8)
		var off uint32
		switch i {
		case 0:
			off = off0
		case 1:
			off = off0 + uint32(rec0Len)
		case 2:
			off = off0 + uint32(rec0Len) + uint32(rec1Len)
		}
		binary.BigEndian.PutUint32(ri[0:4], off)
		ri[4] = 0
		copy(ri[5:8], []byte{0, 0, byte(i + 1)})
		buf = append(buf, ri...)
	}
	mobiData := make([]byte, rec0Len)
	copy(mobiData[16:20], "MOBI")
	binary.BigEndian.PutUint32(mobiData[20:24], 232)
	binary.BigEndian.PutUint16(mobiData[0:2], 1)
	binary.BigEndian.PutUint32(mobiData[4:8], 1000)
	binary.BigEndian.PutUint16(mobiData[8:10], 1)
	binary.BigEndian.PutUint16(mobiData[10:12], 4096)
	binary.BigEndian.PutUint32(mobiData[24:28], 2)
	binary.BigEndian.PutUint32(mobiData[28:32], 65001)
	binary.BigEndian.PutUint32(mobiData[36:40], 6)
	binary.BigEndian.PutUint32(mobiData[80:84], 1)
	binary.BigEndian.PutUint32(mobiData[108:112], 2)
	binary.BigEndian.PutUint16(mobiData[192:194], 1)
	binary.BigEndian.PutUint16(mobiData[194:196], 2)
	buf = append(buf, mobiData...)
	// text
	buf = append(buf, []byte("hello mobi text!!!!!")...)
	// pad to rec1 len 20 already (we wrote 20 exactly)
	// image
	buf = append(buf, imageData...)
	return buf
}
