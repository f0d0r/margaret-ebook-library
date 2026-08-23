package mobi

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/f0d0r/margaret-ebook-library/internal/config"
	"github.com/f0d0r/margaret-ebook-library/internal/resource"
	"github.com/f0d0r/margaret-ebook-library/internal/util"
	"github.com/f0d0r/margaret-ebook-library/pkg/errs"
	"github.com/f0d0r/margaret-ebook-library/pkg/model"
)

type MobiReader struct {
	cfg config.Config
}

// NewMobiReader creates a new MOBI reader instance with the given
// safety limits.
func NewMobiReader(cfg config.Config) *MobiReader {
	return &MobiReader{cfg: cfg}
}

func (r *MobiReader) Supports(b model.Blob) bool {
	// The "BOOKMOBI" identifier starts at byte 60 and ends at byte 67.
	// Therefore reading exactly 68 bytes is sufficient.
	buf := make([]byte, 68)
	n, err := b.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		return false
	}
	if n < 68 {
		return false
	}

	// Check whether bytes 60-67 contain the "BOOKMOBI" string.
	return string(buf[60:68]) == "BOOKMOBI"
}

func (r *MobiReader) Read(b model.Blob) (*model.Ebook, error) {
	maxRecordSize := r.cfg.MaxRecordSize
	if maxRecordSize <= 0 {
		maxRecordSize = config.DefaultConfig().MaxRecordSize
	}
	pdbDb, err := ReadPdbDb(b, maxRecordSize)
	if err != nil {
		return nil, fmt.Errorf("failed to read PDB database: %w", err)
	}

	maxExthRecords := r.cfg.MaxExthRecords
	if maxExthRecords <= 0 {
		maxExthRecords = config.DefaultConfig().MaxExthRecords
	}
	mobi, err := ReadMobi(pdbDb, maxExthRecords)
	if err != nil {
		return nil, fmt.Errorf("failed to read MOBI file: %w", err)
	}

	var languages []string
	if mobi.Language() != "" {
		languages = []string{mobi.Language()}
	}

	cover := r.cover(b, pdbDb, mobi)
	content := r.content(pdbDb, mobi)
	maxResourceSize := r.cfg.MaxResourceSize
	if maxResourceSize <= 0 {
		maxResourceSize = config.DefaultConfig().MaxResourceSize
	}
	if maxRecordSize <= 0 {
		maxRecordSize = config.DefaultConfig().MaxRecordSize
	}

	// Build All + ReadingOrder for ResourceSet (cover alias handled internally)
	all, readingOrder, coverAliased := r.buildMobiResources(b, pdbDb, mobi, cover, content, maxResourceSize, maxRecordSize)

	return &model.Ebook{
		Metadata: model.Metadata{
			Title:       mobi.Title(),
			Authors:     mobi.Authors(),
			Description: mobi.Description(),
			Languages:   languages,
		},
		Resources: resource.NewResourceSet(all, readingOrder, coverAliased),
		FileType:  model.MOBI,
		Version:   mobi.Version(),
	}, nil
}

func (r *MobiReader) cover(b model.Blob, pdbDb *PdbDb, mobi *Mobi) *model.Resource {
	coverIdx := mobi.CoverRecordIdx()
	if coverIdx == 0 || int(coverIdx) >= len(pdbDb.PdbRecords) {
		return nil
	}
	coverRecord := pdbDb.PdbRecords[coverIdx]
	coverLength := coverRecord.Length
	if coverLength == 0 {
		return nil
	}
	maxCoverSize := r.cfg.MaxResourceSize
	if maxCoverSize <= 0 {
		maxCoverSize = config.DefaultConfig().MaxResourceSize
	}
	maxRecordSize := r.cfg.MaxRecordSize
	if maxRecordSize <= 0 {
		maxRecordSize = config.DefaultConfig().MaxRecordSize
	}
	coverOffset := coverRecord.Offset
	// Try to detect media; for oversized records DataSlice will fail due to limit check, so fallback to direct read.
	magicData, err := coverRecord.DataSlice(8)
	var media *util.Media
	if err == nil {
		media = util.DetectImageMedia(magicData)
	}
	if media == nil {
		if coverLength > uint32(maxCoverSize) || coverLength > uint32(maxRecordSize) {
			// Oversized: try direct read without limit for magic detection
			buf := make([]byte, 8)
			if n, rerr := b.ReadAt(buf, int64(coverOffset)); rerr == nil || rerr == io.EOF {
				media = util.DetectImageMedia(buf[:n])
			}
			if media == nil {
				media = &util.Media{Type: "application/octet-stream", Extension: "bin"}
			}
		} else {
			return nil
		}
	}
	href := "cover." + media.Extension
	mediaType := media.Type
	return &model.Resource{
		Id:           "cover",
		Name:         href,
		Href:         href,
		ResolvedHref: href,
		MediaType:    mediaType,
		Properties:   "cover-image",
		Size:         int64(coverLength),
		Open: func() (io.ReadCloser, error) {
			if maxCoverSize > 0 && int64(coverLength) > maxCoverSize {
				return nil, fmt.Errorf("%w: %q %d bytes exceeds MaxResourceSize %d", errs.ErrLimitExceeded, href, coverLength, maxCoverSize)
			}
			if maxRecordSize > 0 && int64(coverLength) > maxRecordSize {
				return nil, fmt.Errorf("%w: %q %d bytes exceeds MaxRecordSize %d", errs.ErrLimitExceeded, href, coverLength, maxRecordSize)
			}
			sr := io.NewSectionReader(b, int64(coverOffset), int64(coverLength))
			limit := maxCoverSize
			if maxRecordSize > 0 && (limit <= 0 || maxRecordSize < limit) {
				limit = maxRecordSize
			}
			if limit > 0 {
				return io.NopCloser(util.LimitReader(sr, limit)), nil
			}
			return io.NopCloser(sr), nil
		},
	}
}

func (r *MobiReader) buildMobiResources(b model.Blob, pdbDb *PdbDb, mobi *Mobi, cover *model.Resource, content []model.Resource, maxResourceSize, maxRecordSize int64) ([]*model.Resource, []model.ReadingOrderItem, *model.Resource) {
	// Content pointers (for reading order)
	var contentPtrs []*model.Resource
	for i := range content {
		c := content[i]
		if c.Href == "" {
			c.Href = c.Name
		}
		if c.ResolvedHref == "" {
			c.ResolvedHref = util.CleanHref(c.Name)
		} else {
			c.ResolvedHref = util.CleanHref(c.ResolvedHref)
		}
		c.Href = util.CleanHref(c.Href)
		if c.Id == "" {
			c.Id = c.Name
		}
		ptr := new(model.Resource)
		*ptr = c
		contentPtrs = append(contentPtrs, ptr)
	}

	readingOrder := buildMobiReadingOrder(contentPtrs)

	images := imageResources(b, pdbDb, mobi, maxResourceSize, maxRecordSize)

	// Deduplicate cover vs images by PDB record index — ResolvedHref differs (cover.jpg vs images/00042.jpg).
	coverIdx := mobi.CoverRecordIdx()
	if cover != nil && coverIdx != 0 {
		imagesByRecord := make(map[uint32]*model.Resource, len(images))
		for _, im := range images {
			imagesByRecord[im.recordIndex] = im.resource
		}
		if img, ok := imagesByRecord[coverIdx]; ok {
			cover = img
		}
	}

	// All: reading order first, then images not already in reading order + cover if not in images.
	// Deduplicate by canonical ResolvedHref.
	seen := make(map[string]bool)
	all := make([]*model.Resource, 0, len(contentPtrs)+len(images)+1)
	for _, ptr := range contentPtrs {
		key := util.CleanHref(ptr.ResolvedHref)
		if !seen[key] {
			seen[key] = true
			all = append(all, ptr)
		}
	}
	for _, im := range images {
		key := util.CleanHref(im.resource.ResolvedHref)
		if !seen[key] {
			seen[key] = true
			all = append(all, im.resource)
		}
	}
	if cover != nil {
		key := util.CleanHref(cover.ResolvedHref)
		if !seen[key] {
			seen[key] = true
			all = append(all, cover)
		}
	}
	return all, readingOrder, cover
}

// content returns the book's text as HTML resources.
// For MOBI6 it is a single index.html; for MOBI8/KF8 it is multiple part files in correct order.
func (r *MobiReader) content(pdbDb *PdbDb, mobi *Mobi) []model.Resource {
	maxResourceSize := r.cfg.MaxResourceSize
	if maxResourceSize <= 0 {
		maxResourceSize = config.DefaultConfig().MaxResourceSize
	}

	// Try MOBI8 path first
	if isMobi8(mobi) {
		resources, err := r.contentMobi8(pdbDb, mobi, maxResourceSize)
		if err != nil {
			// Per-resource limit exceeded: return a single resource that
			// fails on Open with ErrLimitExceeded so the error is explicit
			// to the caller (instead of silently falling back to MOBI6).
			if errors.Is(err, errs.ErrLimitExceeded) {
				return []model.Resource{{
					Name:      "part0000.html",
					MediaType: "application/x-mobipocket-html",
					Size:      0,
					Open: func() (io.ReadCloser, error) {
						return nil, err
					},
				}}
			}
		}
		if resources != nil {
			return resources
		}
		// fallback to MOBI6 if MOBI8 parsing failed
	}

	if mobi.TextRecordCount == 0 || int(mobi.FirstTextRecord) >= len(pdbDb.PdbRecords) {
		return nil
	}
	return []model.Resource{{
		Name:      "index.html",
		MediaType: "application/x-mobipocket-html",
		Size:      int64(mobi.TextLength),
		Open: func() (io.ReadCloser, error) {
			data, err := extractText(pdbDb, mobi, maxResourceSize)
			if err != nil {
				return nil, err
			}
			return io.NopCloser(bytes.NewReader(data)), nil
		},
	}}
}

func isMobi8(mobi *Mobi) bool {
	if mobi.KF8 != nil {
		return true
	}
	if mobi.MobiVersion == 8 && mobi.SkelIdx != NullIndex {
		return true
	}
	if mobi.MobiVersion == 8 && mobi.DivIdx != NullIndex {
		return true
	}
	return false
}

func (r *MobiReader) contentMobi8(pdbDb *PdbDb, mobi *Mobi, maxResourceSize int64) ([]model.Resource, error) {
	// rawML is the whole decompressed KF8 HTML before splitting; limiting it
	// as a single resource would reject books where each part is under the
	// limit but the sum is not (e.g. 2x60 MB with 100 MB limit). MaxResourceSize
	// is per Resource, so rawML is decompressed without a limit and the
	// per-part check below enforces the documented semantics. The transient
	// rawML allocation is noted explicitly.
	rawML, kf8, offset, err := extractMobi8Raw(pdbDb, mobi, -1)
	if err != nil || len(rawML) == 0 {
		return nil, err
	}
	sections, err := loadSections(pdbDb)
	if err != nil {
		return nil, err
	}
	// For joint files, indices are KF8-relative. Slice to KF8 start so that
	// readMobi8Indices can use them directly (mimics calibre's kf8_sections = sections[offset-1:]).
	kf8Sections := sections
	if offset > 1 && offset-1 < len(sections) {
		kf8Sections = sections[offset-1:]
	}
	codec := kf8.Codec
	if codec == "" {
		codec = "utf-8"
	}
	flowTable, files, elems, err := readMobi8Indices(kf8Sections, kf8, codec)
	if err != nil {
		// If indices fail, return rawML as single part (still better than MOBI6 fallback)
		if len(rawML) > 0 {
			// Enforce per-resource limit even on fallback part.
			if maxResourceSize > 0 && int64(len(rawML)) > maxResourceSize {
				return nil, fmt.Errorf("%w: part %q %d bytes exceeds MaxResourceSize %d", errs.ErrLimitExceeded, "part0000.html", len(rawML), maxResourceSize)
			}
			return []model.Resource{{
				Name:      "part0000.html",
				MediaType: "application/x-mobipocket-html",
				Size:      int64(len(rawML)),
				Open: func() (io.ReadCloser, error) {
					return io.NopCloser(bytes.NewReader(rawML)), nil
				},
			}}, nil
		}
		return nil, err
	}

	parts, partInfos, err := buildMobi8Parts(rawML, flowTable, files, elems)
	if err != nil || len(parts) == 0 {
		return nil, err
	}

	// Enforce MaxResourceSize per Resource (documented semantics): any single
	// part exceeding the limit fails the KF8 content with ErrLimitExceeded
	// instead of silently returning oversized parts or checking the total sum.
	for i, p := range parts {
		if maxResourceSize > 0 && int64(len(p)) > maxResourceSize {
			name := partInfos[i].Filename
			if name == "" {
				name = fmt.Sprintf("part%04d.html", i)
			}
			return nil, fmt.Errorf("%w: part %q %d bytes exceeds MaxResourceSize %d", errs.ErrLimitExceeded, name, len(p), maxResourceSize)
		}
	}

	resources := make([]model.Resource, 0, len(parts))
	for i, part := range parts {
		p := part // capture
		info := partInfos[i]
		name := info.Filename
		if name == "" {
			name = fmt.Sprintf("part%04d.html", i)
		}
		size := int64(len(p))
		resources = append(resources, model.Resource{
			Name:      name,
			MediaType: "application/x-mobipocket-html",
			Size:      size,
			Open: func() (io.ReadCloser, error) {
				if maxResourceSize > 0 && int64(len(p)) > maxResourceSize {
					return nil, fmt.Errorf("%w: part %q %d bytes exceeds MaxResourceSize %d", errs.ErrLimitExceeded, name, len(p), maxResourceSize)
				}
				return io.NopCloser(bytes.NewReader(p)), nil
			},
		})
	}
	return resources, nil
}
