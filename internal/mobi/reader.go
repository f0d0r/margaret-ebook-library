package mobi

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/charset"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
	"github.com/f0d0r/margaret-ebook-library/internal/opf"
	"github.com/f0d0r/margaret-ebook-library/internal/pdb"
	"github.com/f0d0r/margaret-ebook-library/internal/sections"
	"github.com/f0d0r/margaret-ebook-library/internal/util"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

type MobiReader struct {
	cfg config.Config
}

// NewMobiReader creates a new MOBI reader with the given safety limits.
// A zero Config normalizes to [config.DefaultConfig].
func NewMobiReader(cfg config.Config) *MobiReader {
	return &MobiReader{cfg: cfg.Normalize()}
}

func (r *MobiReader) Supports(b book.Blob) bool {
	// PDB type/creator ident at bytes 60-67, uppercased like calibre.
	buf := make([]byte, 68)
	n, err := b.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		return false
	}
	if n < 68 {
		return false
	}

	switch strings.ToUpper(string(buf[60:68])) {
	case pdbIdentMobi, pdbIdentPalmDoc:
		return true
	default:
		return false
	}
}

func (r *MobiReader) Read(b book.Blob) (book.Book, error) {
	maxRecordSize := r.cfg.MaxRecordSize
	pdbDb, err := pdb.ReadPdbDb(b, maxRecordSize)
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
// preserved as-is; a prefix sniff decides only the media type and, for HTML,
// the OEB metadata source. PalmDOC has no cover or image records.
func (r *MobiReader) readPalmDoc(pdbDb *pdb.PdbDb) (book.Book, error) {
	mobiDoc, err := readPalmDocHeader(pdbDb)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to read PalmDOC file: %w", book.ErrParseFailed, err)
	}

	if isDRMProtected(mobiDoc) {
		return nil, fmt.Errorf("palmdoc is DRM protected: %w", book.ErrDRM)
	}

	// Best-effort sniff: a prefix failure surfaces from Open instead.
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
		Charset:      charsetForEncoding(mobiDoc.TextEncoding),
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

func (r *MobiReader) cover(b book.Blob, pdbDb *pdb.PdbDb, mobiDoc *Mobi) *book.Resource {
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
	// DataSlice honors record limits, so oversized records need a direct read.
	magicData, err := coverRecord.DataSlice(8)
	var media *util.Media
	if err == nil {
		media = util.DetectImageMedia(magicData)
	}
	if media == nil {
		if coverLength > uint32(maxCoverSize) || coverLength > uint32(maxRecordSize) {
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
		Charset:      book.CharsetBinary,
		Properties:   "cover-image",
		Size:         int64(coverLength),
		Open: func() (io.ReadCloser, error) {
			return sections.Open(b, int64(coverOffset), int64(coverLength), maxCoverSize, maxRecordSize, href)
		},
	}
}

func (r *MobiReader) buildMobiResources(b book.Blob, pdbDb *pdb.PdbDb, mobiDoc *Mobi, cover *book.Resource, content []book.Resource, maxResourceSize, maxRecordSize int64) ([]*book.Resource, []book.ReadingOrderItem, *book.Resource) {
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

	// Merge in order, collapsing duplicates by canonical ResolvedHref.
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

// charsetForEncoding maps a MOBI text encoding to the Resource.Charset
// label, mirroring decodeString: only CP1252 decodes as windows-1252,
// everything else is stored UTF-8.
func charsetForEncoding(enc TextEncodingType) string {
	if enc == CP1252 {
		return "windows-1252"
	}
	return book.CharsetUTF8
}

// content returns the book's text as HTML resources: a single index.html for
// MOBI6, ordered parts for MOBI8/KF8. A KF8 parse failure surfaces from Open,
// except in dual files with KF8FallbackToMOBI6 where MOBI6 is served instead.
func (r *MobiReader) content(pdbDb *pdb.PdbDb, mobiDoc *Mobi) []book.Resource {
	maxResourceSize := r.cfg.MaxResourceSize

	// Broken indices degrade to a single raw-ML resource; only DRM, unusable
	// content or limit errors reach the switch below.
	if isMobi8(mobiDoc) {
		resources, err := r.contentMobi8(pdbDb, mobiDoc, maxResourceSize)
		if err == nil && resources == nil {
			// Empty success counts as failure for the fallback handling below.
			err = fmt.Errorf("no KF8 content produced")
		}
		if err == nil {
			return resources
		}
		switch {
		case errors.Is(err, book.ErrLimitExceeded):
			// Report limits from Open; falling back would silently serve different content.
			return []book.Resource{failingResource("part0000.html", err)}
		case errors.Is(err, book.ErrDRM):
			// A dual file's MOBI6 part is DRM protected too; never fall back.
			return []book.Resource{failingResource("part0000.html", err)}
		case !r.cfg.KF8Fallback() || mobiDoc.KF8 == nil:
			// No MOBI6 rendering to fall back to.
			return []book.Resource{failingResource("part0000.html", kf8ContentError(err))}
		}
		// Otherwise serve the intact MOBI6 content, or report the KF8 failure.
		if mobiDoc.TextRecordCount == 0 || 1 >= len(pdbDb.PdbRecords) {
			return []book.Resource{failingResource("part0000.html", kf8ContentError(err))}
		}
	}

	// Text records start at record 1; FirstTextRecord may hold the 0xFFFF
	// sentinel in real-world files and must be ignored.
	if mobiDoc.TextRecordCount == 0 || 1 >= len(pdbDb.PdbRecords) {
		return nil
	}
	return []book.Resource{{
		Name:      "index.html",
		MediaType: "application/x-mobipocket-html",
		Charset:   charsetForEncoding(mobiDoc.TextEncoding),
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

func (r *MobiReader) contentMobi8(pdbDb *pdb.PdbDb, mobiDoc *Mobi, maxResourceSize int64) (resources []book.Resource, err error) {
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
		contentCharset := charset.Normalize(codec)
		flowTable, files, elems, err := readMobi8Indices(kf8Sections, kf8, codec)
		if err != nil {
			if len(rawML) == 0 {
				return err
			}
			// Broken indices still yield the decompressed raw markup as one
			// resource. It names a synthetic part, so book.LimitError (for
			// manifest resources) does not apply.
			if maxResourceSize > 0 && int64(len(rawML)) > maxResourceSize {
				return fmt.Errorf("%w: part %q %d bytes exceeds MaxResourceSize %d", book.ErrLimitExceeded, "part0000.html", len(rawML), maxResourceSize)
			}
			resources = []book.Resource{htmlResource("part0000.html", rawML, contentCharset)}
			return nil
		}

		parts, partInfos, err := buildMobi8Parts(rawML, flowTable, files, elems)
		if err != nil || len(parts) == 0 {
			return err
		}

		// Synthetic part names again, not manifest resources.
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
			resources = append(resources, htmlResource(name, part, contentCharset))
		}
		return nil
	})
	return
}

// htmlResource serves already-bounded KF8 bytes; Open just returns them.
func htmlResource(name string, data []byte, charset string) book.Resource {
	if charset == "" {
		charset = book.CharsetUTF8
	}
	return book.Resource{
		Name:      name,
		MediaType: "application/x-mobipocket-html",
		Charset:   charset,
		Size:      int64(len(data)),
		Open: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(data)), nil
		},
	}
}

// failingResource reports err from Open, routing parse failures through the resource API.
func failingResource(name string, err error) book.Resource {
	return book.Resource{
		Name:      name,
		MediaType: "application/x-mobipocket-html",
		Charset:   book.CharsetUTF8,
		Size:      0,
		Open: func() (io.ReadCloser, error) {
			return nil, err
		},
	}
}
