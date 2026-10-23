package lrf

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/compress"
	"github.com/f0d0r/margaret-ebook-library/internal/util"
	xmlx "github.com/f0d0r/margaret-ebook-library/internal/xml"
)

type Info struct {
	BookInfo BookInfo `xml:"BookInfo"`
	DocInfo  DocInfo  `xml:"DocInfo"`
	Keywords []string `xml:"Keyword"`
}

type BookInfo struct {
	Title          string   `xml:"Title"`
	Author         string   `xml:"Author"`
	Publisher      string   `xml:"Publisher"`
	Categories     []string `xml:"Category"`
	Classification string   `xml:"Classification"`
	Description    string   `xml:"FreeText"`
}

type DocInfo struct {
	Language string `xml:"Language"`
}

// lrfAuthorSep splits authors on common conjunction separators.
var lrfAuthorSep = regexp.MustCompile(`(?i),?\s+(and|with)\s+`)

// readMetadata decompresses and parses the info block into book.Metadata.
func (r *LrfReader) readMetadata(b book.Blob, header *LrfHeader) (book.Metadata, error) {
	if header.CompressedInfoSize == 0 {
		return book.Metadata{}, nil
	}
	if header.CompressedInfoSize < 4 {
		return book.Metadata{}, fmt.Errorf("%w: bad compressed info size %d", book.ErrCorrupt, header.CompressedInfoSize)
	}

	xmlData, err := r.readBookInfoXml(b, header)
	if err != nil {
		return book.Metadata{}, err
	}
	var info Info
	if err := xmlx.NewDecoder(bytes.NewReader(xmlData)).Decode(&info); err != nil {
		return book.Metadata{}, fmt.Errorf("%w: parse info xml: %w", book.ErrCorrupt, err)
	}

	md := book.Metadata{
		Title:       strings.TrimSpace(info.BookInfo.Title),
		Authors:     splitAuthors(info.BookInfo.Author),
		Description: strings.TrimSpace(info.BookInfo.Description),
		Publisher:   strings.TrimSpace(info.BookInfo.Publisher),
		Subjects:    lrfSubjects(info),
	}
	if lang := strings.TrimSpace(info.DocInfo.Language); lang != "" {
		md.Languages = []string{lang}
	}
	return md, nil
}

// lrfSubjects combines the repeatable Category elements, the Classification
// value and the Keyword elements in source order. Category and Classification
// are split on commas and semicolons; Keyword elements are kept whole.
func lrfSubjects(info Info) []string {
	var subjects []string
	for _, category := range info.BookInfo.Categories {
		subjects = append(subjects, splitCommaSemicolon(category)...)
	}
	subjects = append(subjects, splitCommaSemicolon(info.BookInfo.Classification)...)
	for _, keyword := range info.Keywords {
		if keyword = strings.TrimSpace(keyword); keyword != "" {
			subjects = append(subjects, keyword)
		}
	}
	return util.DedupeSubjects(subjects)
}

func splitCommaSemicolon(raw string) []string {
	var parts []string
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';'
	}) {
		if part = strings.TrimSpace(part); part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

// splitAuthors splits an author string: '&&' escapes a literal '&' and
// ', and'/' and'/', with'/' with' are separators.
func splitAuthors(raw string) []string {
	raw = strings.ReplaceAll(raw, "&&", "\uFFFF")
	raw = lrfAuthorSep.ReplaceAllString(raw, "&")

	var authors []string
	for _, a := range strings.Split(raw, "&") {
		a = strings.TrimSpace(strings.ReplaceAll(a, "\uFFFF", "&"))
		if a != "" {
			authors = append(authors, a)
		}
	}
	return authors
}

func (r *LrfReader) readBookInfoXml(b book.Blob, header *LrfHeader) ([]byte, error) {
	zdata := make([]byte, int64(header.CompressedInfoSize)-4)
	if err := util.ReadAtFull(b, zdata, header.InfoStart); err != nil {
		return nil, fmt.Errorf("%w: read info block: %w", book.ErrCorrupt, err)
	}

	remain := r.cfg.MaxResourceSize
	if header.Version > 800 && int64(header.UncompressedInfo) > 0 && int64(header.UncompressedInfo) < remain {
		remain = int64(header.UncompressedInfo)
	}
	xmlData, err := compress.DecompressZlib(zdata, remain)
	if err != nil {
		return nil, err
	}
	if header.Version > 800 && header.UncompressedInfo > 0 && uint32(len(xmlData)) != header.UncompressedInfo {
		return nil, fmt.Errorf("%w: metadata size mismatch: got %d, want %d",
			book.ErrCorrupt, len(xmlData), header.UncompressedInfo)
	}

	return bytes.TrimPrefix(xmlData, []byte{0xEF, 0xBB, 0xBF}), nil
}
