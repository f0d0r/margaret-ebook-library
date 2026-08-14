package mobi

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/f0d0r/margaret-ebook-library/pkg/model"
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
	RecordAttrCatMask uint8 = 0x0F // The lower 4 bits mask for the category
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
	Data       func() ([]byte, error)
	DataSlice  func(len uint32) ([]byte, error)
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

func ReadPdbDb(b model.Blob, maxRecordSize int64) (*PdbDb, error) {
	fileSize, err := b.Size()
	if err != nil {
		return nil, fmt.Errorf("get file size: %w", err)
	}
	if fileSize > math.MaxUint32 {
		return nil, fmt.Errorf("file too large: %d", fileSize)
	}

	header, err := readFullAt(b, 0, PDB_HEADER_SIZE)
	if err != nil {
		return nil, fmt.Errorf("read pdb header: %w", err)
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

	pdbRecords, err := parsePdbRecords(uint32(fileSize), numberOfRecords, b, maxRecordSize)
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
	uniqueId := (uint32(raw[5]) << 16) | (uint32(raw[6]) << 8) | uint32(raw[7])

	return &PdbRecord{
		Offset:     offset,
		Attributes: attributes,
		UniqueId:   uniqueId,
	}, nil
}

func parsePdbRecords(fileSize uint32, numberOfRecords uint16, b model.Blob, maxRecordSize int64) ([]PdbRecord, error) {
	pdbRecords := make([]PdbRecord, numberOfRecords)

	// The record info table is contiguous: one 8-byte entry per record,
	// starting right after the PDB header. Read it in a single random-access
	// read to avoid one syscall (and, for path-based blobs, one file open)
	// per record.
	tableLen := int(numberOfRecords) * 8
	table, err := readFullAt(b, int64(PDB_HEADER_SIZE), tableLen)
	if err != nil {
		return nil, fmt.Errorf("read record info: %w", err)
	}

	for i := 0; i < int(numberOfRecords); i++ {
		record, err := parseRecordInfo(table[i*8 : i*8+8])
		if err != nil {
			return nil, fmt.Errorf("parse record info: %w", err)
		}
		if i > 0 {
			prevRecord := &pdbRecords[i-1]
			if record.Offset < prevRecord.Offset {
				return nil, fmt.Errorf("record %d offset %d is before record %d offset %d", i, record.Offset, i-1, prevRecord.Offset)
			}
			prevRecord.Length = record.Offset - prevRecord.Offset
			offset := prevRecord.Offset
			length := prevRecord.Length

			prevRecord.Data = func() ([]byte, error) {
				return readRecordData(b, offset, length, maxRecordSize)
			}
			prevRecord.DataSlice = func(len uint32) ([]byte, error) {
				return readRecordData(b, offset, len, maxRecordSize)
			}
		}
		pdbRecords[i] = *record
	}

	if numberOfRecords > 0 {
		lastRecord := &pdbRecords[numberOfRecords-1]
		if lastRecord.Offset >= fileSize {
			lastRecord.Length = 0
		} else {
			lastRecord.Length = uint32(fileSize) - lastRecord.Offset
		}
		lastOffset := lastRecord.Offset
		lastLength := lastRecord.Length
		lastRecord.Data = func() ([]byte, error) {
			return readRecordData(b, lastOffset, lastLength, maxRecordSize)
		}
		lastRecord.DataSlice = func(len uint32) ([]byte, error) {
			return readRecordData(b, lastOffset, len, maxRecordSize)
		}
	}
	return pdbRecords, nil
}

func readRecordData(b model.Blob, offset uint32, length uint32, maxRecordSize int64) ([]byte, error) {
	if uint64(length) > uint64(maxRecordSize) {
		return nil, fmt.Errorf("record length %d exceeds maximum %d", length, maxRecordSize)
	}

	data := make([]byte, length)
	n, err := b.ReadAt(data, int64(offset))
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("read pdb record: %w", err)
	}
	if n != int(length) {
		return nil, fmt.Errorf("read pdb record: expected %d bytes, got %d", length, n)
	}

	return data, nil
}

// readFullAt reads exactly length bytes at offset using random access.
func readFullAt(r io.ReaderAt, offset int64, length int) ([]byte, error) {
	data := make([]byte, length)
	n, err := r.ReadAt(data, offset)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if n != length {
		return nil, io.ErrUnexpectedEOF
	}
	return data, nil
}
