package mobi

import (
	"encoding/binary"
	"fmt"

	"github.com/f0d0r/margaret-ebook-library/pkg/errs"
)

const (
	AUTHOR           = 100
	DESCRIPTION      = 103
	KF8_HEADER_INDEX = 121
	COVER_OFFSET     = 201
	THUMBNAIL_OFFSET = 202
	UPDATED_TITLE    = 503
	LANGUAGE         = 524
)

type Exth struct {
	Identifier   string              // Identifier for the EXTH record: "EXTH"
	HeaderLength uint32              // the length of the EXTH header, including the previous 4 bytes - but not including the final padding.
	RecordCount  uint32              // the number of records in the EXTH block
	Records      map[uint32][][]byte // the records in the EXTH block, indexed by their type, the same type can have multiple values
	textEncoding TextEncodingType
}

var errInvalidOffset = errs.ErrInvalidOffset

func ReadExth(offset uint32, data []byte, textEncoding TextEncodingType, maxExthRecords int) (*Exth, error) {
	if offset+12 > uint32(len(data)) {
		return nil, errInvalidOffset
	}

	exth := &Exth{
		Identifier:   string(data[offset : offset+4]),
		HeaderLength: binary.BigEndian.Uint32(data[offset+4 : offset+8]),
		RecordCount:  binary.BigEndian.Uint32(data[offset+8 : offset+12]),
		textEncoding: textEncoding,
	}

	exth.Records = make(map[uint32][][]byte)
	offset += 12
	for i := 0; i < int(exth.RecordCount) && i < maxExthRecords; i++ {
		if offset+8 > uint32(len(data)) {
			return nil, fmt.Errorf("EXTH record %d header exceeds data bounds", i)
		}
		recordType := binary.BigEndian.Uint32(data[offset : offset+4])
		recordLength := binary.BigEndian.Uint32(data[offset+4 : offset+8])

		if recordLength < 8 {
			return nil, fmt.Errorf("EXTH record %d length %d is too small", i, recordLength)
		}
		if offset+recordLength > uint32(len(data)) {
			return nil, fmt.Errorf("EXTH record %d length %d exceeds data bounds", i, recordLength)
		}

		recordData := data[offset+8 : offset+recordLength]
		exth.Records[recordType] = append(exth.Records[recordType], recordData)
		offset += recordLength
	}

	return exth, nil
}

// UpdatedTitle returns the updated title from the EXTH record.
// The title is decoded according to the text encoding specified in the MOBI header.
// If the updated title record is not present, an empty string is returned.
func (e *Exth) UpdatedTitle() string {
	titleData := e.Records[UPDATED_TITLE]
	if len(titleData) == 0 {
		return ""
	}
	return decodeString(titleData[0], e.textEncoding)
}

// KF8HeaderIndex returns the offset of the KF8 header within the MOBI header.
// The offset is stored in the EXTH record type 121 (KF8_HEADER_INDEX).
// If the record is not present, 0 is returned.
func (e *Exth) KF8HeaderIndex() uint32 {
	d := e.Records[KF8_HEADER_INDEX]
	if len(d) == 0 || len(d[0]) < 4 {
		return 0
	}
	return binary.BigEndian.Uint32(d[0])
}

// Authors returns the author names from the EXTH record in a slice of strings.
// The author is decoded according to the text encoding specified in the MOBI header.
// If the author record is not present, nil is returned.
func (e *Exth) Authors() []string {
	d := e.Records[AUTHOR]
	if len(d) == 0 {
		return nil
	}
	authors := make([]string, 0, len(d))
	for _, v := range d {
		author := decodeString(v, e.textEncoding)
		if author != "" {
			authors = append(authors, author)
		}
	}
	return authors
}

// Description returns the description from the EXTH record.
// The description is decoded according to the text encoding specified in the MOBI header.
// If the description record is not present, an empty string is returned.
func (e *Exth) Description() string {
	d := e.Records[DESCRIPTION]
	if len(d) == 0 {
		return ""
	}
	return decodeString(d[0], e.textEncoding)
}

// Language returns the language code from the EXTH record.
// If the language record is not present, an empty string is returned.
func (e *Exth) Language() string {
	d := e.Records[LANGUAGE]
	if len(d) == 0 {
		return ""
	}
	return decodeString(d[0], e.textEncoding)
}

// CoverOffset returns the cover image record offset from the EXTH record.
// The offset is stored in the EXTH record type 201 (COVER_OFFSET).
// If the record is not present or contains less than 4 bytes, 0 is returned.
func (e *Exth) CoverOffset() uint32 {
	d := e.Records[COVER_OFFSET]
	if len(d) == 0 || len(d[0]) < 4 {
		return 0
	}
	return binary.BigEndian.Uint32(d[0])
}

// ThumbnailOffset returns the thumbnail image record offset from the EXTH record.
// The offset is stored in the EXTH record type 202 (THUMBNAIL_OFFSET).
// If the record is not present or contains less than 4 bytes, 0 is returned.
func (e *Exth) ThumbnailOffset() uint32 {
	d := e.Records[THUMBNAIL_OFFSET]
	if len(d) == 0 || len(d[0]) < 4 {
		return 0
	}
	return binary.BigEndian.Uint32(d[0])
}
