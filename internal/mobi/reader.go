package mobi

import (
	"fmt"
	"io"

	"github.com/f0d0r/margaret-ebook-library/internal/util"
	"github.com/f0d0r/margaret-ebook-library/pkg/model"
)

const MAX_EXPECTED_COVER_SIZE = 50 * 1024 * 1024 // 50 MB safety limit for cover image

type MobiReader struct{}

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

// ReadMetadata reads ebook metadata from a random-access source. It
// never modifies the source's position and does not take ownership of it.
func (r *MobiReader) ReadMetadata(b model.Blob) (*model.Metadata, error) {
	pdbDb, err := ReadPdbDb(b)
	if err != nil {
		return nil, fmt.Errorf("failed to read PDB database: %w", err)
	}

	mobi, err := ReadMobi(pdbDb)
	if err != nil {
		return nil, fmt.Errorf("failed to read MOBI file: %w", err)
	}

	var languages []string
	if mobi.Language() != "" {
		languages = []string{mobi.Language()}
	}
	return &model.Metadata{
		Title:       mobi.Title(),
		Authors:     mobi.Authors(),
		Description: mobi.Description(),
		Languages:   languages,
		Cover:       r.cover(b, pdbDb, mobi),
		FileType:    model.MOBI,
	}, nil
}

func (r *MobiReader) cover(b model.Blob, pdbDb *PdbDb, mobi *Mobi) *model.Resource {
	coverIdx := mobi.CoverRecordIdx()
	if coverIdx == 0 || int(coverIdx) >= len(pdbDb.PdbRecords) {
		return nil
	}
	coverRecord := pdbDb.PdbRecords[coverIdx]
	coverLength := coverRecord.Length
	if coverLength == 0 || coverLength > MAX_EXPECTED_COVER_SIZE {
		return nil
	}
	magicData, err := coverRecord.DataSlice(8)
	if err != nil {
		return nil
	}
	media := util.DetectImageMedia(magicData)
	if media == nil {
		return nil
	}
	coverOffset := coverRecord.Offset
	return &model.Resource{
		Name:      "cover." + media.Extension,
		MediaType: media.Type,
		Size:      int(coverLength),
		Data: func() ([]byte, error) {
			return readRecordData(b, coverOffset, coverLength)
		},
	}
}