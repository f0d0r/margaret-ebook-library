package mobi

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/f0d0r/margaret-ebook-library/internal/util"
)

const (
	AUTHOR           = 100
	PUBLISHER        = 101
	DESCRIPTION      = 103
	ISBN             = 104
	SUBJECT          = 105
	PUBLICATION_DATE = 106
	SOURCE           = 112
	ASIN             = 113
	KF8_HEADER_INDEX = 121
	COVER_OFFSET     = 201
	THUMBNAIL_OFFSET = 202
	UPDATED_TITLE    = 503
	ASIN_5XX         = 504
	LANGUAGE         = 524
)

type Exth struct {
	Identifier   string
	HeaderLength uint32
	RecordCount  uint32
	Records      map[uint32][][]byte // Indexed by type; one type can hold multiple values.
	textEncoding TextEncodingType
}

var errInvalidOffset = errors.New("invalid offset")

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
func (e *Exth) UpdatedTitle() string {
	titleData := e.Records[UPDATED_TITLE]
	if len(titleData) == 0 {
		return ""
	}
	return decodeString(titleData[0], e.textEncoding)
}

// KF8HeaderIndex returns the KF8 header offset (type 121), or 0 if absent.
func (e *Exth) KF8HeaderIndex() uint32 {
	d := e.Records[KF8_HEADER_INDEX]
	if len(d) == 0 || len(d[0]) < 4 {
		return 0
	}
	return binary.BigEndian.Uint32(d[0])
}

// Authors returns the author names from the EXTH record.
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

// Subjects returns the subject values from the EXTH record.
func (e *Exth) Subjects() []string {
	raw := e.Records[SUBJECT]
	if len(raw) == 0 {
		return nil
	}
	values := make([]string, 0, len(raw))
	for _, v := range raw {
		values = append(values, decodeString(v, e.textEncoding))
	}
	return util.SplitSubjects(values)
}

// Publisher returns the publisher from the EXTH record.
func (e *Exth) Publisher() string {
	return e.firstRecordString(PUBLISHER)
}

// PublicationDate returns the raw publication date from the EXTH record.
// The value is returned as declared, without parsing or validation.
func (e *Exth) PublicationDate() string {
	return e.firstRecordString(PUBLICATION_DATE)
}

// firstRecordString returns the first non-empty decoded value of a record type.
func (e *Exth) firstRecordString(recordType uint32) string {
	for _, raw := range e.Records[recordType] {
		if value := decodeString(raw, e.textEncoding); value != "" {
			return value
		}
	}
	return ""
}

// Description returns the description from the EXTH record.
func (e *Exth) Description() string {
	d := e.Records[DESCRIPTION]
	if len(d) == 0 {
		return ""
	}
	return decodeString(d[0], e.textEncoding)
}

// Language returns the language code from the EXTH record.
func (e *Exth) Language() string {
	d := e.Records[LANGUAGE]
	if len(d) == 0 {
		return ""
	}
	return decodeString(d[0], e.textEncoding)
}

// CoverOffset returns the cover image record offset (type 201), or 0 if absent.
func (e *Exth) CoverOffset() uint32 {
	d := e.Records[COVER_OFFSET]
	if len(d) == 0 || len(d[0]) < 4 {
		return 0
	}
	return binary.BigEndian.Uint32(d[0])
}

// ThumbnailOffset returns the thumbnail record offset (type 202), or 0 if absent.
func (e *Exth) ThumbnailOffset() uint32 {
	d := e.Records[THUMBNAIL_OFFSET]
	if len(d) == 0 || len(d[0]) < 4 {
		return 0
	}
	return binary.BigEndian.Uint32(d[0])
}
