package mobi

import (
	"fmt"
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
	n, err := f.ReadAt(buf, 0)
	if err != nil && n < 68 {
		return false
	}

	// Check whether bytes 60-67 contain the "BOOKMOBI" string.
	return string(buf[60:68]) == "BOOKMOBI"
}

func (r *MobiReader) ReadMetadata(path string) (*model.Metadata, error) {
	fmt.Printf("[mobi-reader] reading metadata from: %s\n", path)

	return &model.Metadata{
		Title:    "Dummy MOBI Book",
		FileType: model.MOBI,
	}, nil
}
