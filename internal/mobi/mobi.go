package mobi

import (
	"encoding/binary"
	"fmt"
)

const PALM_DOC_HEADER_SIZE = 16

const NullIndex uint32 = 0xFFFFFFFF

type CompressionType uint16

const (
	CompressionNone    CompressionType = 1
	CompressionPalmDOC CompressionType = 2
	CompressionHUFF    CompressionType = 17480
)

type EncryptionType uint16

const (
	EncryptionNone    EncryptionType = 0
	EncryptionOldMobi EncryptionType = 1
	EncryptionMobi    EncryptionType = 2
)

type MobiType uint32

const (
	MobiTypeBook         MobiType = 2
	MobiTypePalmDoc      MobiType = 3
	MobiTypeAudio        MobiType = 4
	MobiTypeKindlegen12  MobiType = 232 // MOBI (Kindlegen 1.2)
	MobiTypeKF8          MobiType = 248 // KF8 / AZW3 (Kindlegen 2.0)
	MobiTypeNews         MobiType = 257
	MobiTypeNewsFeed     MobiType = 258
	MobiTypeNewsMagazine MobiType = 259
	// Office codes (513+) we don't support them
)

type TextEncodingType uint32

const (
	CP1252 TextEncodingType = 1252
	UTF8   TextEncodingType = 65001
)

type Mobi struct {
	Compression         CompressionType
	TextLength          uint32 // Uncompressed length of the entire text of the book
	TextRecordCount     uint16 // Number of PDB records used for the text of the book.
	MaxTextRecordSize   uint16 // Maximum size of each record containing text, always 4096
	Encryption          EncryptionType
	Identifier          string // the characters MOBI
	HeaderLength        uint32 // Length of the header
	Type                MobiType
	TextEncoding        TextEncodingType
	MobiVersion         uint32 // Version of the MOBI format, 4,5 very old, 6 most common, 8 newer KF8
	FirstNonTextRecord  uint32 // First record number (starting with 0) that's not the book's text
	FullNameOffset      uint32 // Offset in record 0 (not from start of file) of the full name of the book
	FullNameLength      uint32 // Length in bytes of the full name of the book
	Locale              uint32 // Book locale code. Low byte is main language 09= English, next byte is dialect, 08 = British, 04 = US. Thus US English is 1033, UK English is 2057.
	InputLanguage       uint32 // Input language for a dictionary
	OutputLanguage      uint32 // Output language for a dictionary
	MinVersion          uint32 // Minimum version of the MOBI format required to read this book
	FirstImageRecord    uint32 // First record number (starting with 0) that contains an image
	HuffmanRecordOffset uint32 // Offset in the file of the Huffman table
	HuffmanRecordCount  uint32 // Number of Huffman records in the file
	HuffmanTableOffset  uint32 // Offset in the file of the Huffman table
	HuffmanTableLength  uint32 // Length in bytes of the Huffman table
	EXTHFlags           uint32 // bitfield. if bit 6 (0x40) is set, then there's an EXTH record
	DRMOffset           uint32 // Offset to DRM key info in DRMed files. 0xFFFFFFFF if no DRM
	DRMCount            uint32 // Number of entries in DRM info. 0xFFFFFFFF if no DRM
	DRMSize             uint32 // Number of bytes in DRM info.
	DRMFlags            uint32 // Flags for DRM info
	FirstTextRecord     uint16 // Number of first text record. Normally 1.
	LastContentRecord   uint16 // Number of last image record or number of last text record if it contains no images. Includes Image, DATP, HUFF, DRM.
	FCISRecordOffset    uint32 // Offset to FCIS record in the file.
	FCISRecordCount     uint32 // Number of FCIS records in the file.
	FLISRecordOffset    uint32 // Offset to FLIS record in the file.
	FLISRecordCount     uint32 // Number of FLIS records in the file.
	ExtraRecordFlags    uint32 // A set of binary flags, some of which indicate extra data at the end of each text block.
	IndxRecordOffset    uint32 // Offset to INDX record in the file (alias for NcxIdx, kept for compatibility).
	NcxIdx              uint32 // NCX index (at 0xF4), 0xFFFFFFFF if not present
	DivIdx              uint32 // DIV table index (at 0xF8), 0xFFFFFFFF if not present
	SkelIdx             uint32 // Skeleton index (at 0xFC), 0xFFFFFFFF if not present
	DatpIdx             uint32 // DATP index (at 0x100), 0xFFFFFFFF if not present
	OthIdx              uint32 // OTH (guide) index (at 0x104), 0xFFFFFFFF if not present
	FdstIdx             uint32 // FDST index (at 0xC0), 0xFFFFFFFF if not present or count <=1
	FdstCount           uint32 // FDST count (at 0xC4)
	Codec               string // Go codec name derived from TextEncoding: "cp1252" or "utf-8"
	Kf8FirstImageIndex  uint32 // For KF8 header: absolute first image index (KF8 FirstImageRecord + KF8HeaderIndex)
	Mobi6Records        uint16 // For KF8 header in joint files: number of MOBI6 text records

	EXTH        *Exth // EXTH record, if present
	title       string
	name        string
	KF8         *Mobi  // In case this is a dual MOBI file
	recordCount uint16 // Number of records in the MOBI file
}

func ReadMobi(pdbDb *PdbDb, maxExthRecords int) (*Mobi, error) {
	if len(pdbDb.PdbRecords) == 0 {
		return nil, fmt.Errorf("failed to read Record 0: no records in PDB database")
	}

	data, err := pdbDb.PdbRecords[0].Data()
	if err != nil {
		return nil, fmt.Errorf("failed to read Record 0: %w", err)
	}

	mobi, err := readMobiHeader(data, maxExthRecords)
	if err != nil {
		return nil, fmt.Errorf("failed to read MOBI header: %w", err)
	}

	mobi.name = pdbDb.Name
	mobi.recordCount = pdbDb.NumberOfRecords

	if mobi.EXTH != nil {
		kf8HeaderIdx := mobi.EXTH.KF8HeaderIndex()
		if kf8HeaderIdx != 0 && kf8HeaderIdx-1 > 0 && kf8HeaderIdx < uint32(len(pdbDb.PdbRecords)) {
			boundaryRecordData, err := pdbDb.PdbRecords[kf8HeaderIdx-1].Data()
			if err == nil && len(boundaryRecordData) == 8 && string(boundaryRecordData) == "BOUNDARY" {
				kf8Data, err := pdbDb.PdbRecords[kf8HeaderIdx].Data()
				if err != nil {
					return nil, fmt.Errorf("failed to read KF8 data: %w", err)
				}
				kf8, err := readMobiHeader(kf8Data, maxExthRecords)
				if err != nil {
					return nil, fmt.Errorf("failed to read KF8 header: %w", err)
				}

				kf8.recordCount = uint16(uint32(pdbDb.NumberOfRecords) - kf8HeaderIdx + 1)
				// calibre: kf8_first_image_index = kf8.first_image_index + k8i
				kf8.Kf8FirstImageIndex = kf8.FirstImageRecord + kf8HeaderIdx
				kf8.Mobi6Records = mobi.TextRecordCount
				// calibre restores first_image_index to MOBI6's value on the KF8 header
				// so that generic code sees MOBI6's index, but keep absolute via Kf8FirstImageIndex
				kf8.FirstImageRecord = mobi.FirstImageRecord
				if kf8.HuffmanRecordCount > 0 {
					// calibre: huff_offset += k8i for joint files
					kf8.HuffmanRecordOffset += kf8HeaderIdx
				}
				mobi.KF8 = kf8
			}
		}
	}

	return mobi, nil
}

func readMobiHeader(data []byte, maxExthRecords int) (*Mobi, error) {

	if len(data) < 96 {
		return nil, fmt.Errorf("record 0 is too short")
	}

	compression := binary.BigEndian.Uint16(data[0:2])
	textLength := binary.BigEndian.Uint32(data[4:8])
	textRecordCount := binary.BigEndian.Uint16(data[8:10])
	maxTextRecordSize := binary.BigEndian.Uint16(data[10:12])
	encryption := binary.BigEndian.Uint16(data[12:14])
	identifier := string(data[16:20])
	headerLength := binary.BigEndian.Uint32(data[20:24])
	mobiType := binary.BigEndian.Uint32(data[24:28])
	textEncoding := binary.BigEndian.Uint32(data[28:32])
	mobiVersion := binary.BigEndian.Uint32(data[36:40])
	firstNonTextRecord := binary.BigEndian.Uint32(data[80:84])
	fullNameOffset := binary.BigEndian.Uint32(data[84:88])
	fullNameLength := binary.BigEndian.Uint32(data[88:92])
	locale := binary.BigEndian.Uint32(data[92:96])

	var codec string
	switch TextEncodingType(textEncoding) {
	case UTF8:
		codec = "utf-8"
	case CP1252:
		codec = "cp1252"
	default:
		// calibre: unknown codepage -> cp1252 fallback
		codec = "cp1252"
	}

	mobi := &Mobi{
		Compression:        CompressionType(compression),
		TextLength:         textLength,
		TextRecordCount:    textRecordCount,
		MaxTextRecordSize:  maxTextRecordSize,
		Encryption:         EncryptionType(encryption),
		Identifier:         identifier,
		HeaderLength:       headerLength,
		Type:               MobiType(mobiType),
		TextEncoding:       TextEncodingType(textEncoding),
		MobiVersion:        mobiVersion,
		FirstNonTextRecord: firstNonTextRecord,
		FullNameOffset:     fullNameOffset,
		FullNameLength:     fullNameLength,
		Locale:             locale,
		Codec:              codec,

		DRMOffset:        0xFFFFFFFF,
		FCISRecordOffset: 0xFFFFFFFF,
		FLISRecordOffset: 0xFFFFFFFF,
		ExtraRecordFlags: 0,
		IndxRecordOffset: 0xFFFFFFFF,
		NcxIdx:           NullIndex,
		DivIdx:           NullIndex,
		SkelIdx:          NullIndex,
		DatpIdx:          NullIndex,
		OthIdx:           NullIndex,
		FdstIdx:          NullIndex,
		FdstCount:        0,
	}

	if len(data) >= 184 {
		mobi.InputLanguage = binary.BigEndian.Uint32(data[96:100])
		mobi.OutputLanguage = binary.BigEndian.Uint32(data[100:104])
		mobi.MinVersion = binary.BigEndian.Uint32(data[104:108])
		mobi.FirstImageRecord = binary.BigEndian.Uint32(data[108:112])
		mobi.HuffmanRecordOffset = binary.BigEndian.Uint32(data[112:116])
		mobi.HuffmanRecordCount = binary.BigEndian.Uint32(data[116:120])
		mobi.HuffmanTableOffset = binary.BigEndian.Uint32(data[120:124])
		mobi.HuffmanTableLength = binary.BigEndian.Uint32(data[124:128])
		mobi.EXTHFlags = binary.BigEndian.Uint32(data[128:132])
		mobi.DRMOffset = binary.BigEndian.Uint32(data[168:172])
		mobi.DRMCount = binary.BigEndian.Uint32(data[172:176])
		mobi.DRMSize = binary.BigEndian.Uint32(data[176:180])
		mobi.DRMFlags = binary.BigEndian.Uint32(data[180:184])
	}

	if len(data) >= 216 {
		mobi.FirstTextRecord = binary.BigEndian.Uint16(data[192:194])
		mobi.LastContentRecord = binary.BigEndian.Uint16(data[194:196])
		mobi.FCISRecordOffset = binary.BigEndian.Uint32(data[200:204])
		mobi.FCISRecordCount = binary.BigEndian.Uint32(data[204:208])
		mobi.FLISRecordOffset = binary.BigEndian.Uint32(data[208:212])
		mobi.FLISRecordCount = binary.BigEndian.Uint32(data[212:216])
	}

	if len(data) >= 244 && mobi.Identifier == "MOBI" && mobi.HeaderLength >= 0xE4 && mobi.HeaderLength <= 500 {
		// Extra Data Flags is a 16-bit field at offset 242. calibre reads it
		// as raw[0xF2:0xF4] and only honors it for BOOKMOBI files whose
		// header length is at least 0xE4 (228).
		mobi.ExtraRecordFlags = uint32(binary.BigEndian.Uint16(data[242:244]))
	}

	if len(data) >= 248 {
		mobi.IndxRecordOffset = binary.BigEndian.Uint32(data[244:248])
		mobi.NcxIdx = binary.BigEndian.Uint32(data[244:248])
	}

	// calibre: BookHeader fdst/skel/div/oth only for MobiVersion==8, see headers.py:267
	if mobi.MobiVersion == 8 && len(data) >= 0xF8+16 {
		mobi.DivIdx = binary.BigEndian.Uint32(data[0xF8 : 0xF8+4])
		mobi.SkelIdx = binary.BigEndian.Uint32(data[0xFC : 0xFC+4])
		mobi.DatpIdx = binary.BigEndian.Uint32(data[0x100 : 0x100+4])
		mobi.OthIdx = binary.BigEndian.Uint32(data[0x104 : 0x104+4])
		mobi.FdstIdx = binary.BigEndian.Uint32(data[0xC0 : 0xC0+4])
		mobi.FdstCount = binary.BigEndian.Uint32(data[0xC4 : 0xC4+4])
		if mobi.FdstCount <= 1 {
			mobi.FdstIdx = NullIndex
		}
	}

	if mobi.HasEXTH() {
		exthOffset := mobi.HeaderLength + PALM_DOC_HEADER_SIZE
		exth, err := ReadExth(exthOffset, data, mobi.TextEncoding, maxExthRecords)
		if err != nil {
			return nil, fmt.Errorf("failed to read EXTH record: %w", err)
		}
		mobi.EXTH = exth
	}

	if fullNameOffset <= uint32(len(data)) && fullNameLength <= uint32(len(data))-fullNameOffset {
		mobi.title = decodeString(data[fullNameOffset:fullNameOffset+fullNameLength], mobi.TextEncoding)
	}
	return mobi, nil
}

// HasEXTH reports whether this MOBI file contains an EXTH record.
// EXTH (Extended Header) records contain metadata such as title, author, and other book information.
func (m *Mobi) HasEXTH() bool {
	return (m.EXTHFlags & 0x40) != 0
}

// HasDRM reports whether this MOBI file is protected with DRM (Digital Rights Management).
func (m *Mobi) HasDRM() bool {
	return m.DRMOffset != 0xFFFFFFFF
}

// Title returns the book title. It prefers the updated title from the EXTH record if available,
// otherwise falls back to the full name extracted from the MOBI header.
func (m *Mobi) Title() string {
	var title string
	if m.KF8 != nil {
		title = m.KF8.Title()
	}

	if title == "" && m.EXTH != nil {
		title = m.EXTH.UpdatedTitle()
	}

	if title == "" {
		title = m.title
	}

	if title == "" {
		title = m.name
	}
	return title
}

// Authors returns the author names from the book metadata.
// It first checks the KF8 header (if present in a dual MOBI file),
// then falls back to the EXTH record.
// If neither contains an author, an empty slice is returned.
func (m *Mobi) Authors() []string {
	var authors []string
	if m.KF8 != nil {
		authors = m.KF8.Authors()
	}
	if authors == nil && m.EXTH != nil {
		authors = m.EXTH.Authors()
	}
	return authors
}

// Description returns the description from the book metadata.
// It first checks the KF8 header (if present in a dual MOBI file),
// then falls back to the EXTH record.
// If neither contains a description, an empty string is returned.
func (m *Mobi) Description() string {
	var description string
	if m.KF8 != nil {
		description = m.KF8.Description()
	}
	if description == "" && m.EXTH != nil {
		description = m.EXTH.Description()
	}
	return description
}

// Language returns the language code from the book metadata.
// It first checks the KF8 header (if present in a dual MOBI file),
// then falls back to the EXTH record.
// If neither contains a language, an empty string is returned.
func (m *Mobi) Language() string {
	var language string
	if m.KF8 != nil {
		language = m.KF8.Language()
	}
	if language == "" && m.EXTH != nil {
		language = m.EXTH.Language()
	}
	if language == "" {
		language = m.localeCode()
	}
	return language
}

// Version returns the declared Mobipocket format version of the file. For a
// dual MOBI/KF8 file both versions are reported, e.g. "6/8". It returns an
// empty string if no version is declared.
func (m *Mobi) Version() string {
	ver := m.MobiVersion
	if m.KF8 != nil {
		kf8Ver := m.KF8.MobiVersion
		switch {
		case ver != 0 && kf8Ver != 0:
			return fmt.Sprintf("%d/%d", ver, kf8Ver)
		case kf8Ver != 0:
			return fmt.Sprintf("%d", kf8Ver)
		}
	}
	if ver == 0 {
		return ""
	}
	return fmt.Sprintf("%d", ver)
}

// CoverRecordIdx returns the record index of the cover image.
// In a dual MOBI file it checks the KF8 header first, then falls back to
// the EXTH CoverOffset combined with FirstImageRecord.
// Returns 0 if no cover image is found.
func (m *Mobi) CoverRecordIdx() uint32 {
	if m.KF8 != nil && m.EXTH != nil {
		absIdx := uint32(m.EXTH.KF8HeaderIndex()) + m.KF8.CoverRecordIdx()
		if absIdx < uint32(m.recordCount) {
			return absIdx
		}
	}
	if m.EXTH != nil &&
		m.FirstImageRecord != 0 &&
		m.FirstImageRecord != 0xFFFF &&
		m.FirstImageRecord != 0xFFFFFFFF {
		absIdx := m.FirstImageRecord + m.EXTH.CoverOffset()
		if absIdx < uint32(m.recordCount) {
			return absIdx
		}
	}
	return 0
}
