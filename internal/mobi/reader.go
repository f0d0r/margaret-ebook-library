package mobi

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
	"github.com/f0d0r/margaret-ebook-library/internal/opf"
	"github.com/f0d0r/margaret-ebook-library/internal/util"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

type MobiReader struct {
	cfg config.Config
}

// NewMobiReader creates a new MOBI reader instance with the given
// safety limits. A zero Config is normalized to [config.DefaultConfig] so that
// a hand-built struct cannot silently disable the limits or the KF8 fallback.
func NewMobiReader(cfg config.Config) *MobiReader {
	return &MobiReader{cfg: cfg.Normalize()}
}

func (r *MobiReader) Supports(b book.Blob) bool {
	// The type/creator identifier sits at bytes 60-67 ("BOOKMOBI" for
	// Mobipocket, "TEXtREAd" for PalmDOC). Reading exactly 68 bytes is
	// sufficient. Like calibre the ident is uppercased, so TEXtREAd
	// matches TEXTREAD.
	buf := make([]byte, 68)
	n, err := b.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		return false
	}
	if n < 68 {
		return false
	}

	// Check whether bytes 60-67 contain a known identifier.
	switch strings.ToUpper(string(buf[60:68])) {
	case pdbIdentMobi, pdbIdentPalmDoc:
		return true
	default:
		return false
	}
}

func (r *MobiReader) Read(b book.Blob) (book.Book, error) {
	maxRecordSize := r.cfg.MaxRecordSize
	pdbDb, err := ReadPdbDb(b, maxRecordSize)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to read PDB database: %w", book.ErrCorrupt, err)
	}

	if isPalmDocDb(pdbDb) {
		return r.readPalmDoc(pdbDb)
	}

	mobiDoc, err := ReadMobi(pdbDb, r.cfg.MaxExthRecords)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to read MOBI file: %w", book.ErrParseFailed, err)
	}

	if isDRMProtected(mobiDoc) {
		return nil, fmt.Errorf("mobi is DRM protected: %w", book.ErrDRM)
	}

	var languages []string
	if mobiDoc.Language() != "" {
		languages = []string{mobiDoc.Language()}
	}

	cover := r.cover(b, pdbDb, mobiDoc)
	content := r.content(pdbDb, mobiDoc)

	// Build All + ReadingOrder for ResourceSet (cover alias handled internally)
	all, readingOrder, coverAliased := r.buildMobiResources(b, pdbDb, mobiDoc, cover, content, r.cfg.MaxResourceSize, maxRecordSize)

	return &mobiBook{
		metadata: book.Metadata{
			Title:       mobiDoc.Title(),
			Authors:     mobiDoc.Authors(),
			Description: mobiDoc.Description(),
			Languages:   languages,
		},
		resources: book.NewResourceSet(all, readingOrder, coverAliased),
		version:   mobiDoc.Version(),
		mobiDoc:   mobiDoc,
	}, nil
}

// readPalmDoc reads a pure PalmDOC book (TEXtREAd container). The payload is
// preserved as-is: a sniff on the decompressed prefix decides only the media
// type (text/html vs text/plain) and, for HTML, the OEB dc-metadata source.
// The content itself stays lazily-opened so MaxResourceSize surfaces from
// Open like on the MOBI path. PalmDOC has no cover or image records.
func (r *MobiReader) readPalmDoc(pdbDb *PdbDb) (book.Book, error) {
	mobiDoc, err := readPalmDocHeader(pdbDb)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to read PalmDOC file: %w", book.ErrParseFailed, err)
	}

	if isDRMProtected(mobiDoc) {
		return nil, fmt.Errorf("palmdoc is DRM protected: %w", book.ErrDRM)
	}

	// Best-effort prefix for the media type decision and the OEB metadata.
	// A prefix failure must not fail the book: Open reports the real error.
	prefix, _ := palmDocPrefixText(pdbDb, mobiDoc)
	isHTML := isPalmDocHTML(prefix)

	title := pdbDb.Name
	var authors []string
	var description string
	var languages []string
	if isHTML {
		if meta, merr := opf.ParseMetadata(strings.NewReader(decodeString(prefix, CP1252))); merr == nil {
			pkg := opf.Package{Metadata: meta}
			if t := pkg.Title(); t != "" {
				title = t
			}
			authors = pkg.Authors()
			description = pkg.Description()
			languages = pkg.Languages()
		}
	}

	name, mediaType := "index.txt", mediatype.PlainText
	if isHTML {
		name, mediaType = "index.html", mediatype.HTML
	}
	maxResourceSize := r.cfg.MaxResourceSize
	content := &book.Resource{
		Id:           "content",
		Name:         name,
		Href:         name,
		ResolvedHref: name,
		MediaType:    mediaType,
		Size:         int64(mobiDoc.TextLength),
		Open: func() (io.ReadCloser, error) {
			data, err := extractPalmDocText(pdbDb, mobiDoc, maxResourceSize)
			if err != nil {
				return nil, err
			}
			return io.NopCloser(bytes.NewReader(data)), nil
		},
	}
	readingOrder := buildMobiReadingOrder([]*book.Resource{content})

	return &mobiBook{
		metadata: book.Metadata{
			Title:       title,
			Authors:     authors,
			Description: description,
			Languages:   languages,
		},
		resources: book.NewResourceSet([]*book.Resource{content}, readingOrder, nil),
		version:   mobiDoc.Version(),
		mobiDoc:   mobiDoc,
		isPalmDoc: true,
	}, nil
}

func (r *MobiReader) cover(b book.Blob, pdbDb *PdbDb, mobiDoc *Mobi) *book.Resource {
	coverIdx := mobiDoc.CoverRecordIdx()
	if coverIdx == 0 || int(coverIdx) >= len(pdbDb.PdbRecords) {
		return nil
	}
	coverRecord := pdbDb.PdbRecords[coverIdx]
	coverLength := coverRecord.Length
	if coverLength == 0 {
		return nil
	}
	maxCoverSize := r.cfg.MaxResourceSize
	maxRecordSize := r.cfg.MaxRecordSize
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
				media = util.UnknownMedia()
			}
		} else {
			return nil
		}
	}
	href := "cover." + media.Extension
	mediaType := media.Type
	return &book.Resource{
		Id:           "cover",
		Name:         href,
		Href:         href,
		ResolvedHref: href,
		MediaType:    mediaType,
		Properties:   "cover-image",
		Size:         int64(coverLength),
		Open: func() (io.ReadCloser, error) {
			if maxCoverSize > 0 && int64(coverLength) > maxCoverSize {
				return nil, book.LimitError(href, int64(coverLength), maxCoverSize)
			}
			if maxRecordSize > 0 && int64(coverLength) > maxRecordSize {
				return nil, fmt.Errorf("%w: %q %d bytes exceeds MaxRecordSize %d", book.ErrLimitExceeded, href, coverLength, maxRecordSize)
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

func (r *MobiReader) buildMobiResources(b book.Blob, pdbDb *PdbDb, mobiDoc *Mobi, cover *book.Resource, content []book.Resource, maxResourceSize, maxRecordSize int64) ([]*book.Resource, []book.ReadingOrderItem, *book.Resource) {
	// Content pointers (for reading order)
	var contentPtrs []*book.Resource
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
		ptr := new(book.Resource)
		*ptr = c
		contentPtrs = append(contentPtrs, ptr)
	}

	readingOrder := buildMobiReadingOrder(contentPtrs)

	images := imageResources(b, pdbDb, mobiDoc, maxResourceSize, maxRecordSize)

	// Deduplicate cover vs images by PDB record index — ResolvedHref differs (cover.jpg vs images/00042.jpg).
	coverIdx := mobiDoc.CoverRecordIdx()
	if cover != nil && coverIdx != 0 {
		imagesByRecord := make(map[uint32]*book.Resource, len(images))
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
	all := make([]*book.Resource, 0, len(contentPtrs)+len(images)+1)
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
//
// A KF8 parse failure never fails the whole read silently: the returned
// resource carries the error on Open, so the caller sees it explicitly. The
// only exception is a dual MOBI6+KF8 file with KF8FallbackToMOBI6 enabled
// (the default), where the intact MOBI6 content is served instead.
func (r *MobiReader) content(pdbDb *PdbDb, mobiDoc *Mobi) []book.Resource {
	maxResourceSize := r.cfg.MaxResourceSize

	// Try the MOBI8 path first. It only returns an error when the KF8 content
	// could not be produced at all (DRM, unusable indices, limit exceeded);
	// readMobi8Indices already degrades to a single raw-ML resource when the
	// index tables are broken.
	if isMobi8(mobiDoc) {
		resources, err := r.contentMobi8(pdbDb, mobiDoc, maxResourceSize)
		if err == nil && resources == nil {
			// No KF8 content at all: treat it as a parse failure so the
			// fallback/strict handling below applies.
			err = fmt.Errorf("no KF8 content produced")
		}
		if err == nil {
			return resources
		}
		switch {
		case errors.Is(err, book.ErrLimitExceeded):
			// Surface the limit on Open instead of hiding it behind the MOBI6
			// fallback, which would silently return different content.
			return []book.Resource{failingResource("part0000.html", err)}
		case errors.Is(err, book.ErrDRM):
			// A DRM-protected KF8 part cannot be read, and the MOBI6 part of a
			// dual file is DRM protected as well; never fall back here.
			return []book.Resource{failingResource("part0000.html", err)}
		case !r.cfg.KF8Fallback() || mobiDoc.KF8 == nil:
			// Fallback disabled, or no MOBI6 content to fall back to: a
			// standalone KF8 file has no second rendering to serve.
			return []book.Resource{failingResource("part0000.html", kf8ContentError(err))}
		}
		// A joint MOBI6+KF8 file with the fallback enabled: serve the intact
		// MOBI6 content when it exists, otherwise report the KF8 failure.
		if mobiDoc.TextRecordCount == 0 || 1 >= len(pdbDb.PdbRecords) {
			return []book.Resource{failingResource("part0000.html", kf8ContentError(err))}
		}
	}

	// MOBI6 text records always start at record 1 (calibre:
	// range(offset, min(records+offset, len)) with offset=1). The
	// FirstTextRecord header field is ignored because real-world files
	// may leave it as the 0xFFFF sentinel.
	if mobiDoc.TextRecordCount == 0 || 1 >= len(pdbDb.PdbRecords) {
		return nil
	}
	return []book.Resource{{
		Name:      "index.html",
		MediaType: "application/x-mobipocket-html",
		Size:      int64(mobiDoc.TextLength),
		Open: func() (io.ReadCloser, error) {
			data, err := extractText(pdbDb, mobiDoc, maxResourceSize)
			if err != nil {
				return nil, err
			}
			return io.NopCloser(bytes.NewReader(data)), nil
		},
	}}
}

func isMobi8(mobiDoc *Mobi) bool {
	if mobiDoc.KF8 != nil {
		return true
	}
	if mobiDoc.MobiVersion == 8 && mobiDoc.SkelIdx != NullIndex {
		return true
	}
	if mobiDoc.MobiVersion == 8 && mobiDoc.DivIdx != NullIndex {
		return true
	}
	return false
}

func (r *MobiReader) contentMobi8(pdbDb *PdbDb, mobiDoc *Mobi, maxResourceSize int64) (resources []book.Resource, err error) {
	err = withRecover("contentMobi8", func() error {
		rawML, kf8, offset, err := extractMobi8Raw(pdbDb, mobiDoc, -1)
		if err != nil || len(rawML) == 0 {
			return err
		}
		sections, err := loadSections(pdbDb)
		if err != nil {
			return err
		}
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
			if len(rawML) == 0 {
				return err
			}
			// The index tables are unusable but the raw markup was decompressed:
			// serve it as a single resource instead of dropping the content.
			// This message names a KF8 part rather than a manifest resource, so
			// it does not use book.LimitError.
			if maxResourceSize > 0 && int64(len(rawML)) > maxResourceSize {
				return fmt.Errorf("%w: part %q %d bytes exceeds MaxResourceSize %d", book.ErrLimitExceeded, "part0000.html", len(rawML), maxResourceSize)
			}
			resources = []book.Resource{htmlResource("part0000.html", rawML)}
			return nil
		}

		parts, partInfos, err := buildMobi8Parts(rawML, flowTable, files, elems)
		if err != nil || len(parts) == 0 {
			return err
		}

		// Same wording as above: these name synthetic KF8 parts, not manifest
		// resources, so book.LimitError does not apply.
		for i, p := range parts {
			if maxResourceSize > 0 && int64(len(p)) > maxResourceSize {
				name := partInfos[i].Filename
				if name == "" {
					name = fmt.Sprintf("part%04d.html", i)
				}
				return fmt.Errorf("%w: part %q %d bytes exceeds MaxResourceSize %d", book.ErrLimitExceeded, name, len(p), maxResourceSize)
			}
		}

		resources = make([]book.Resource, 0, len(parts))
		for i, part := range parts {
			info := partInfos[i]
			name := info.Filename
			if name == "" {
				name = fmt.Sprintf("part%04d.html", i)
			}
			resources = append(resources, htmlResource(name, part))
		}
		return nil
	})
	return
}

// htmlResource builds a KF8 content resource that serves data on Open.
// KF8 parts are bounded at parse time (MaxResourceSize is checked before the
// resource is handed out), so Open simply returns the bytes.
func htmlResource(name string, data []byte) book.Resource {
	return book.Resource{
		Name:      name,
		MediaType: "application/x-mobipocket-html",
		Size:      int64(len(data)),
		Open: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(data)), nil
		},
	}
}

// failingResource builds a KF8 content resource that reports err on Open, so a
// parse failure reaches the caller through the normal resource API.
func failingResource(name string, err error) book.Resource {
	return book.Resource{
		Name:      name,
		MediaType: "application/x-mobipocket-html",
		Size:      0,
		Open: func() (io.ReadCloser, error) {
			return nil, err
		},
	}
}
