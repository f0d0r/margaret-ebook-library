package mobi

import (
	"fmt"
	"io"
	"path"

	"github.com/f0d0r/margaret-ebook-library/internal/util"
	"github.com/f0d0r/margaret-ebook-library/pkg/errs"
	"github.com/f0d0r/margaret-ebook-library/pkg/model"
)

// mobiImageResource is internal; recordIndex stays MOBI-specific and is not exposed via model.Resource.
type mobiImageResource struct {
	recordIndex uint32
	resource    *model.Resource
}

// imageResources extracts image resources from PDB records between FirstImageRecord and LastContentRecord.
// It returns synthetic Resources with ResolvedHref like "images/00001.jpg" plus the underlying record index.
// For MOBI6, FirstImageRecord is used; for KF8 standalone, also FirstImageRecord; for joint, absolute Kf8FirstImageIndex is preferred if available.
// Non-image records are skipped (FDAT/CRES etc. without image magic).
// Recognized images are always included in All(); if they exceed either limit, Open() returns ErrLimitExceeded.
func imageResources(b model.Blob, pdbDb *PdbDb, mobi *Mobi, maxResourceSize, maxRecordSize int64) []mobiImageResource {
	if pdbDb == nil || mobi == nil {
		return nil
	}
	firstImage := mobi.FirstImageRecord
	if mobi.KF8 != nil && mobi.KF8.Kf8FirstImageIndex != 0 {
		if mobi.KF8.FirstImageRecord != 0xFFFFFFFF && mobi.KF8.FirstImageRecord != 0xFFFF && mobi.KF8.FirstImageRecord != 0 {
			firstImage = mobi.KF8.Kf8FirstImageIndex
		}
	}
	if firstImage == 0 || firstImage == 0xFFFF || firstImage == 0xFFFFFFFF {
		return nil
	}
	last := uint32(mobi.LastContentRecord)
	if last == 0 || last >= uint32(len(pdbDb.PdbRecords)) {
		last = uint32(len(pdbDb.PdbRecords) - 1)
	}
	if firstImage > last {
		return nil
	}
	var out []mobiImageResource
	for idx := firstImage; idx <= last && int(idx) < len(pdbDb.PdbRecords); idx++ {
		rec := pdbDb.PdbRecords[idx]
		if rec.Length == 0 {
			continue
		}
		magic, err := rec.DataSlice(8)
		if err != nil {
			continue
		}
		media := util.DetectImageMedia(magic)
		if media == nil {
			continue
		}
		offset := rec.Offset
		length := rec.Length
		ext := media.Extension
		mediaType := media.Type
		href := util.CleanHref(fmt.Sprintf("images/%05d.%s", idx, ext))
		name := path.Base(href)
		maxRes := maxResourceSize
		maxRec := maxRecordSize
		recOffset := offset
		recLength := length
		hrefCopy := href
		mediaCopy := mediaType
		out = append(out, mobiImageResource{
			recordIndex: idx,
			resource: &model.Resource{
				Id:           fmt.Sprintf("image-%05d", idx),
				Name:         name,
				Href:         hrefCopy,
				ResolvedHref: hrefCopy,
				MediaType:    mediaCopy,
				Properties:   "",
				Size:         int64(recLength),
				Open: func() (io.ReadCloser, error) {
					if maxRes > 0 && int64(recLength) > maxRes {
						return nil, fmt.Errorf("%w: %q %d bytes exceeds MaxResourceSize %d", errs.ErrLimitExceeded, hrefCopy, recLength, maxRes)
					}
					if maxRec > 0 && int64(recLength) > maxRec {
						return nil, fmt.Errorf("%w: %q %d bytes exceeds MaxRecordSize %d", errs.ErrLimitExceeded, hrefCopy, recLength, maxRec)
					}
					sr := io.NewSectionReader(b, int64(recOffset), int64(recLength))
					// Secondary protection in case limits change between construction and Open.
					limit := maxRes
					if maxRec > 0 && (limit <= 0 || maxRec < limit) {
						limit = maxRec
					}
					if limit > 0 {
						return io.NopCloser(util.LimitReader(sr, limit)), nil
					}
					return io.NopCloser(sr), nil
				},
			},
		})
	}
	return out
}

// buildMobiReadingOrder builds ReadingOrderItems from content resources (text parts).
// All MOBI reading order items are Linear=true.
func buildMobiReadingOrder(content []*model.Resource) []model.ReadingOrderItem {
	out := make([]model.ReadingOrderItem, 0, len(content))
	for _, r := range content {
		if r == nil {
			continue
		}
		out = append(out, model.ReadingOrderItem{Resource: r, Linear: true})
	}
	return out
}
