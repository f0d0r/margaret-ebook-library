package mobi

import (
	"fmt"
	"io"
	"os"

	"faun.projects/margaret/margaret-ebook-library/pkg/model"
)

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

	return &model.Metadata{
		Title:    mobi.Title(),
		Authors:  mobi.Authors(),
		FileType: model.MOBI,
	}, nil
}
