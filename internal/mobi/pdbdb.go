package mobi

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"
)

const PDB_HEADER_SIZE = 78
const PALM_EPOCH_OFFSET = 2082844800

const (
	AttrReadOnly     uint16 = 0x0002
	AttrDirtyAppInfo uint16 = 0x0004
	AttrBackup       uint16 = 0x0008
	AttrAllowNewer   uint16 = 0x0010
	AttrResetAfter   uint16 = 0x0020
	AttrNoBeam       uint16 = 0x0040
)

const (
	RecordAttrSecret  uint8 = 0x10
	RecordAttrBusy    uint8 = 0x20
	RecordAttrDirty   uint8 = 0x40
	RecordAttrDelete  uint8 = 0x80
	RecordAttrCatMask uint8 = 0x0F // Az alsó 4 bit maszkja
)

type PdbAttributes struct {
	Raw        uint16 // The original raw attribute value
	ReadOnly   bool
	Dirty      bool
	Backup     bool
	AllowNewer bool
	ResetAfter bool
	NoBeam     bool
}

type RecordAttributes struct {
	Raw      uint8
	Secret   bool
	Busy     bool
	Dirty    bool
	Delete   bool
	Category uint8 // value between 0 and 15
}

type PdbRecord struct {
	Offset     uint32
	Length     uint32
	Attributes RecordAttributes
	UniqueId   uint32
	Data    func() ([]byte, error)
	DataSlice func(len uint32) ([]byte, error)
}

type PdbDb struct {
	Name               string
	Attributes         PdbAttributes
	FileVersion        uint16
	CreatedAt          time.Time
	UpdatedAt          time.Time
	BackupAt           time.Time
	ModificationNumber uint32
	AppInfoOffset      uint32 // offset to start of Application Info (if present) or null
	SortInfoOffset     uint32 // offset to start of Sort Info (if present) or null
	Type               string // type of the database (e.g. "BOOK", "appl", "TEXt")
	Creator            string // creator of the database (e.g, "MOBI", "REAd", "zTXT, "Pdoc", "PRC)
	UniqueIdSeed       uint32 // used internally to identify record
	NextRecordListId   uint32 // Only used when in-memory on Palm OS. Always set to zero in stored files.
	NumberOfRecords    uint16 // number of records in the database
	PdbRecords         []PdbRecord
}

func ReadPdbDb(f *os.File) (*PdbDb, error) {
	fileSize, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, fmt.Errorf("seek to end of file: %w", err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek to start of file: %w", err)
	}

	header := make([]byte, PDB_HEADER_SIZE)
	n, err := io.ReadFull(f, header)
	if err != nil {
		return nil, fmt.Errorf("read pdb header: %w", err)
	}
	if n != PDB_HEADER_SIZE {
		return nil, fmt.Errorf("read pdb header: expected %d bytes, got %d", PDB_HEADER_SIZE, n)
	}

	nameBytes := header[0:32]
	name := string(bytes.TrimRight(nameBytes, "\x00"))

	rawAttributes := binary.BigEndian.Uint16(header[32:34])
	attributes := parseAttributes(rawAttributes)

	fileVersion := binary.BigEndian.Uint16(header[34:36])

	createdAt := parsePalmDate(binary.BigEndian.Uint32(header[36:40]))
	updatedAt := parsePalmDate(binary.BigEndian.Uint32(header[40:44]))
	backupAt := parsePalmDate(binary.BigEndian.Uint32(header[44:48]))

	modificationNumber := binary.BigEndian.Uint32(header[48:52])
	appInfoOffset := binary.BigEndian.Uint32(header[52:56])
	sortInfoOffset := binary.BigEndian.Uint32(header[56:60])
	t := string(header[60:64])
	creator := string(header[64:68])
	uniqueIdSeed := binary.BigEndian.Uint32(header[68:72])
	nextRecordListId := binary.BigEndian.Uint32(header[72:76])
	numberOfRecords := binary.BigEndian.Uint16(header[76:78])

	pdbRecords, err := parsePdbRecords(uint32(fileSize), numberOfRecords, f)
	if err != nil {
		return nil, fmt.Errorf("parse pdb records: %w", err)
	}

	return &PdbDb{
		Name:               name,
		Attributes:         attributes,
		FileVersion:        fileVersion,
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
		BackupAt:           backupAt,
		ModificationNumber: modificationNumber,
		AppInfoOffset:      appInfoOffset,
		SortInfoOffset:     sortInfoOffset,
		Type:               t,
		Creator:            creator,
		UniqueIdSeed:       uniqueIdSeed,
		NextRecordListId:   nextRecordListId,
		NumberOfRecords:    numberOfRecords,
		PdbRecords:         pdbRecords,
	}, nil
}

func parseAttributes(rawAttributes uint16) PdbAttributes {
	return PdbAttributes{
		Raw:        rawAttributes,
		ReadOnly:   rawAttributes&AttrReadOnly != 0,
		Dirty:      rawAttributes&AttrDirtyAppInfo != 0,
		Backup:     rawAttributes&AttrBackup != 0,
		AllowNewer: rawAttributes&AttrAllowNewer != 0,
		ResetAfter: rawAttributes&AttrResetAfter != 0,
		NoBeam:     rawAttributes&AttrNoBeam != 0,
	}
}

func parsePalmDate(rawDate uint32) time.Time {
	if rawDate == 0 {
		return time.Time{}
	}
	unixDate := int64(rawDate) - PALM_EPOCH_OFFSET
	return time.Unix(unixDate, 0)
}

func parseRecordAttributes(raw uint8) RecordAttributes {
	return RecordAttributes{
		Raw:      raw,
		Secret:   (raw & RecordAttrSecret) != 0,
		Busy:     (raw & RecordAttrBusy) != 0,
		Dirty:    (raw & RecordAttrDirty) != 0,
		Delete:   (raw & RecordAttrDelete) != 0,
		Category: raw & RecordAttrCatMask,
	}
}

func parseRecordInfo(raw []byte) (*PdbRecord, error) {
	offset := binary.BigEndian.Uint32(raw[0:4])
	attributes := parseRecordAttributes(raw[4])
	idBuf := []byte{0, raw[5], raw[6], raw[7]}
	uniqueId := binary.BigEndian.Uint32(idBuf)

	return &PdbRecord{
		Offset:     offset,
		Attributes: attributes,
		UniqueId:   uniqueId,
	}, nil
}

func parsePdbRecords(fileSize uint32, numberOfRecords uint16, f *os.File) ([]PdbRecord, error) {
	pdbRecords := make([]PdbRecord, numberOfRecords)
	for i := 0; i < int(numberOfRecords); i++ {
		recordInfo := make([]byte, 8)
		n, err := io.ReadFull(f, recordInfo)
		if err != nil {
			return nil, fmt.Errorf("read record info: %w", err)
		}
		if n != 8 {
			return nil, fmt.Errorf("expected 8 bytes, got %d", n)
		}
		record, err := parseRecordInfo(recordInfo)
		if err != nil {
			return nil, fmt.Errorf("parse record info: %w", err)
		}
		if i > 0 {
			prevRecord := &pdbRecords[i-1]
			prevRecord.Length = record.Offset - prevRecord.Offset
			offset := prevRecord.Offset
			length := prevRecord.Length

			prevRecord.Data = func() ([]byte, error) {
				return readRecordData(f, offset, length)
			}
			prevRecord.DataSlice = func(len uint32) ([]byte, error) {
				return readRecordData(f, offset, len)
			}
		}
		pdbRecords[i] = *record
	}

	if numberOfRecords > 0 {
		lastRecord := &pdbRecords[numberOfRecords-1]
		lastRecord.Length = uint32(fileSize) - lastRecord.Offset
		lastRecord.Data = func() ([]byte, error) {
			return readRecordData(f, lastRecord.Offset, lastRecord.Length)
		}
		lastRecord.DataSlice = func(len uint32) ([]byte, error) {
			return readRecordData(f, lastRecord.Offset, len)
		}
	}
	return pdbRecords, nil
}

func readRecordData(f *os.File, offset uint32, length uint32) ([]byte, error) {
	currentMetaOffset, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, fmt.Errorf("failed to get current position: %w", err)
	}

	defer func() { _, _ = f.Seek(currentMetaOffset, io.SeekStart) }()
	
	if _, err = f.Seek(int64(offset), io.SeekStart); err != nil {
		return nil, fmt.Errorf("read pdb record: %w", err)
	}

	data := make([]byte, length)
	n, err := io.ReadFull(f, data)
	if err != nil {
		return nil, fmt.Errorf("read pdb record: %w", err)
	}
	if n != int(length) {
		return nil, fmt.Errorf("read pdb record: expected %d bytes, got %d", length, n)
	}

	return data, nil
}
