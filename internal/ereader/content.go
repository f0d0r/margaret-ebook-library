package ereader

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strings"

	"github.com/f0d0r/margaret-ebook-library/book"
	compressutil "github.com/f0d0r/margaret-ebook-library/internal/compress"
	"github.com/f0d0r/margaret-ebook-library/internal/pdb"
	"github.com/f0d0r/margaret-ebook-library/internal/sections"
	"github.com/f0d0r/margaret-ebook-library/internal/util"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

const (
	compressionPalmDoc = 2
	compressionZlib    = 10
)

const (
	imageHeaderSize = 62
	imageProbeSize  = 70
	coverImageName  = "cover.png"
)

type header struct {
	dropbook      bool
	compression   uint16
	version       string
	textCount     int
	imageFirst    int
	imageCount    int
	footnoteFirst int
	footnoteCount int
	sidebarFirst  int
	sidebarCount  int
	metadataIndex int
	hasMetadata   bool
}

func parseHeader(pdbDb *pdb.PdbDb) (header, error) {
	var hdr header
	if len(pdbDb.PdbRecords) == 0 {
		return hdr, fmt.Errorf("%w: ereader has no records", book.ErrCorrupt)
	}
	rec0 := pdbDb.PdbRecords[0]
	hdr.version = fmt.Sprintf("%d", rec0.Length)
	raw, err := rec0.Data()
	if err != nil {
		return hdr, fmt.Errorf("%w: failed to read record 0: %w", book.ErrCorrupt, err)
	}
	if rec0.Length == headerSizeDropbook {
		if len(raw) < headerSizeDropbook {
			return hdr, fmt.Errorf("%w: truncated dropbook header", book.ErrCorrupt)
		}
		hdr.dropbook = true
		hdr.compression = binary.BigEndian.Uint16(raw[0:2])
		hdr.textCount = max(int(binary.BigEndian.Uint16(raw[12:14]))-1, 0)
		hdr.hasMetadata = binary.BigEndian.Uint16(raw[24:26]) == 1
		hdr.metadataIndex = int(binary.BigEndian.Uint16(raw[44:46]))
		hdr.imageFirst = int(binary.BigEndian.Uint16(raw[40:42]))
		hdr.imageCount = int(binary.BigEndian.Uint16(raw[20:22]))
		hdr.footnoteCount = max(int(binary.BigEndian.Uint16(raw[28:30]))-1, 0)
		hdr.footnoteFirst = int(binary.BigEndian.Uint16(raw[48:50])) + 1
		hdr.sidebarCount = max(int(binary.BigEndian.Uint16(raw[30:32]))-1, 0)
		hdr.sidebarFirst = int(binary.BigEndian.Uint16(raw[50:52])) + 1
		return hdr, nil
	}
	if len(raw) < 10 {
		return hdr, fmt.Errorf("%w: truncated makebook header", book.ErrCorrupt)
	}
	hdr.textCount = max(int(binary.BigEndian.Uint16(raw[8:10]))-1, 0)
	hdr.imageCount = -1
	if len(raw) >= 118 {
		hdr.imageCount = int(binary.BigEndian.Uint16(raw[116:118]))
	}
	return hdr, nil
}

func (r *EreaderReader) resources(b book.Blob, pdbDb *pdb.PdbDb, hdr header) ([]*book.Resource, []book.ReadingOrderItem, *book.Resource) {
	maxSize := r.cfg.MaxResourceSize
	content := textResource("content", "index.pml", pdbDb, hdr, pageRange(1, hdr.textCount, len(pdbDb.PdbRecords)), maxSize)
	order := []book.ReadingOrderItem{{Resource: content, Linear: true}}
	all := []*book.Resource{content}
	if hdr.dropbook {
		for _, notes := range []struct {
			id, name string
			pages    []int
		}{
			{"footnotes", "footnotes.pml", pageRange(max(hdr.footnoteFirst, 2), hdr.footnoteCount, len(pdbDb.PdbRecords))},
			{"sidebars", "sidebars.pml", pageRange(max(hdr.sidebarFirst, 2), hdr.sidebarCount, len(pdbDb.PdbRecords))},
		} {
			if len(notes.pages) == 0 {
				continue
			}
			res := textResource(notes.id, notes.name, pdbDb, hdr, notes.pages, maxSize)
			all = append(all, res)
			order = append(order, book.ReadingOrderItem{Resource: res, Linear: false})
		}
	}
	var cover *book.Resource
	for _, img := range imageResources(b, pdbDb, hdr, maxSize, r.cfg.MaxRecordSize) {
		all = append(all, img)
		if cover == nil && strings.EqualFold(img.Name, coverImageName) {
			img.Properties = "cover-image"
			cover = img
		}
	}
	return all, order, cover
}

func pageRange(first, count, total int) []int {
	end := min(first+count, total)
	out := make([]int, 0, max(end-first, 0))
	for i := first; i < end; i++ {
		out = append(out, i)
	}
	return out
}

func textResource(id, name string, pdbDb *pdb.PdbDb, hdr header, pages []int, maxSize int64) *book.Resource {
	var size int64
	for _, i := range pages {
		size += int64(pdbDb.PdbRecords[i].Length)
	}
	return &book.Resource{
		Id:           id,
		Name:         name,
		Href:         name,
		ResolvedHref: name,
		MediaType:    mediatype.EreaderPML,
		Size:         size,
		Open: func() (io.ReadCloser, error) {
			if maxSize > 0 && size > maxSize {
				return nil, book.LimitError(name, size, maxSize)
			}
			data, err := extractPages(pdbDb, hdr, pages, maxSize)
			if err != nil {
				return nil, err
			}
			return io.NopCloser(bytes.NewReader(data)), nil
		},
	}
}

func extractPages(pdbDb *pdb.PdbDb, hdr header, pages []int, maxSize int64) ([]byte, error) {
	var out []byte
	for _, i := range pages {
		data, err := pdbDb.PdbRecords[i].Data()
		if err != nil {
			return nil, fmt.Errorf("failed to read text record %d: %w", i, err)
		}
		dec, err := decompressPage(data, hdr, remainingBudget(maxSize, len(out)))
		if err != nil {
			return nil, err
		}
		if maxSize > 0 && int64(len(out)+len(dec)) > maxSize {
			return nil, book.ErrLimitExceeded
		}
		out = append(out, dec...)
	}
	return out, nil
}

func remainingBudget(maxSize int64, have int) int64 {
	if maxSize <= 0 {
		return -1
	}
	return max(maxSize-int64(have), 0)
}

func decompressPage(data []byte, hdr header, remain int64) ([]byte, error) {
	if hdr.dropbook {
		switch hdr.compression {
		case compressionPalmDoc:
			return compressutil.DecompressPalmDoc(data, remain)
		case compressionZlib:
			return compressutil.DecompressZlib(data, remain)
		default:
			return nil, fmt.Errorf("unsupported ereader compression type %d: %w", hdr.compression, book.ErrParseFailed)
		}
	}
	xored := make([]byte, len(data))
	for i, c := range data {
		xored[i] = c ^ 0xA5
	}
	return compressutil.DecompressPalmDoc(xored, remain)
}

func imageResources(b book.Blob, pdbDb *pdb.PdbDb, hdr header, maxResourceSize, maxRecordSize int64) []*book.Resource {
	var out []*book.Resource
	add := func(idx int) bool {
		if idx <= 0 || idx >= len(pdbDb.PdbRecords) {
			return false
		}
		rec := pdbDb.PdbRecords[idx]
		if rec.Length < imageProbeSize {
			return false
		}
		raw := make([]byte, imageProbeSize)
		n, err := b.ReadAt(raw, int64(rec.Offset))
		if err != nil && err != io.EOF {
			return false
		}
		if n < imageProbeSize || string(raw[:3]) != "PNG" {
			return false
		}
		name := decodeName(raw[4:36])
		mediaType, ok := imageMediaType(name, raw[imageHeaderSize:imageProbeSize])
		if !ok || name == "" {
			return false
		}
		length := rec.Length - imageHeaderSize
		size := int64(length)
		href := util.CleanHref("images/" + name)
		out = append(out, &book.Resource{
			Id:           fmt.Sprintf("image-%05d", idx),
			Name:         name,
			Href:         href,
			ResolvedHref: href,
			MediaType:    mediaType,
			Size:         size,
			Open: func() (io.ReadCloser, error) {
				return sections.Open(b, int64(rec.Offset)+imageHeaderSize, size, maxResourceSize, maxRecordSize, href)
			},
		})
		return true
	}
	if hdr.dropbook {
		for i := hdr.imageFirst; i < hdr.imageFirst+hdr.imageCount && i < len(pdbDb.PdbRecords); i++ {
			add(i)
		}
		return out
	}
	found := 0
	for i := hdr.textCount + 1; i < len(pdbDb.PdbRecords); i++ {
		if hdr.imageCount >= 0 && found >= hdr.imageCount {
			break
		}
		if add(i) {
			found++
		}
	}
	return out
}

func imageMediaType(name string, magic []byte) (string, bool) {
	if media := util.DetectImageMedia(magic); media != nil {
		return media.Type, true
	}
	switch strings.ToLower(imageExt(name)) {
	case "png":
		return mediatype.PNG, true
	case "jpg", "jpeg":
		return mediatype.JPEG, true
	case "gif":
		return mediatype.GIF, true
	}
	return "", false
}

func imageExt(name string) string {
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		return name[i+1:]
	}
	return ""
}
