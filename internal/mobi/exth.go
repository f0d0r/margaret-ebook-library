package mobi

import (
	"encoding/binary"

	"faun.projects/margaret/margaret-ebook-library/pkg/errs"
)

const (
	KF8_HEADER_INDEX = 121
	UPDATED_TITLE    = 503
)

type Exth struct {
	Indentifier  string // Identifier for the EXTH record: "EXTH"
	HeaderLength uint32 // the length of the EXTH header, including the previous 4 bytes - but not including the final padding.
	RecordCount  uint32 // the number of records in the EXTH block
	Records      map[uint32][]byte
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

	exth.Records = make(map[uint32][]byte)
	offset += 12
	for i := 0; i < int(exth.RecordCount); i++ {
		recordType := binary.BigEndian.Uint32(data[offset : offset+4])
		recordLength := binary.BigEndian.Uint32(data[offset+4 : offset+8])
		recordData := data[offset+8 : offset+recordLength]

		exth.Records[recordType] = recordData
		offset += recordLength
	}

	return exth, nil
}

// UpdatedTitle returns the updated title from the EXTH record.
// The title is decoded according to the text encoding specified in the MOBI header.
// If the updated title record is not present, an empty string is returned.
func (e *Exth) UpdatedTitle() string {
	titleData := e.Records[UPDATED_TITLE]
	return decodeString(titleData, e.textEncoding)
}

func (e *Exth) KF8HeaderIndex() uint32 {
	d := e.Records[KF8_HEADER_INDEX]
	if d == nil {
		return 0
	}
	return binary.BigEndian.Uint32(d[:4])
}
