package mobi

import (
	"fmt"
	"io"
	"os"

	"github.com/f0d0r/margaret-ebook-library/internal/util"
	"github.com/f0d0r/margaret-ebook-library/pkg/model"
)

const MAX_EXPECTED_COVER_SIZE = 50 * 1024 * 1024 // 50 MB safety limit for cover image

type MobiReader struct{}

func (r *MobiReader) Supports(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	// The "BOOKMOBI" identifier starts at byte 60 and ends at byte 67.
	// Therefore reading exactly 68 bytes is sufficient.
	buf := make([]byte, 68)
	_, err = io.ReadAtLeast(f, buf, 68)
	if err != nil {
		return false
	}

	// Check whether bytes 60-67 contain the "BOOKMOBI" string.
	return string(buf[60:68]) == "BOOKMOBI"
}

func (r *MobiReader) ReadMetadata(path string) (*model.Metadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = f.Close() }()

	pdbDb, err := ReadPdbDb(f)
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
		Cover:       r.cover(path, pdbDb, mobi),
		FileType:    model.MOBI,
	}, nil
}

func (r *MobiReader) cover(path string, pdbDb *PdbDb, mobi *Mobi) *model.Resource {
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
			return r.loadRecord(path, coverOffset, coverLength)
		},
	}
}

func (r *MobiReader) loadRecord(path string, offset, length uint32) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = f.Close() }()

	data := make([]byte, length)
	_, err = f.ReadAt(data, int64(offset))
	if err != nil {
		return nil, fmt.Errorf("failed to read cover record: %w", err)
	}
	return data, nil
}
