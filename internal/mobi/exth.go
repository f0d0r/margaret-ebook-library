package mobi

import (
	"encoding/binary"

	"faun.projects/margaret/margaret-ebook-library/pkg/errs"
)

const (
	AUTHOR           = 100
	DESCRIPTION      = 103
	KF8_HEADER_INDEX = 121
	UPDATED_TITLE    = 503
	LANGUAGE         = 524
)

type Exth struct {
	Indentifier  string              // Identifier for the EXTH record: "EXTH"
	HeaderLength uint32              // the length of the EXTH header, including the previous 4 bytes - but not including the final padding.
	RecordCount  uint32              // the number of records in the EXTH block
	Records      map[uint32][][]byte // the records in the EXTH block, indexed by their type, the same type can have multiple values
	textEncoding TextEncodingType
}

func ReadExth(offset uint32, data []byte, textEncoding TextEncodingType) (*Exth, error) {
	if offset+12 > uint32(len(data)) {
		return nil, errs.ErrInvalidOffset
	}

	exth := &Exth{
		Indentifier:  string(data[offset : offset+4]),
		HeaderLength: binary.BigEndian.Uint32(data[offset+4 : offset+8]),
		RecordCount:  binary.BigEndian.Uint32(data[offset+8 : offset+12]),
		textEncoding: textEncoding,
	}

	exth.Records = make(map[uint32][][]byte)
	offset += 12
	for i := 0; i < int(exth.RecordCount); i++ {
		recordType := binary.BigEndian.Uint32(data[offset : offset+4])
		recordLength := binary.BigEndian.Uint32(data[offset+4 : offset+8])
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

