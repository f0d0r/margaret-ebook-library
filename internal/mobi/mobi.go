package mobi

import (
	"encoding/binary"
	"fmt"

	"github.com/f0d0r/margaret-ebook-library/internal/pdb"
)

const PALM_DOC_HEADER_SIZE = 16

const NullIndex uint32 = 0xFFFFFFFF

// PDB idents recognized on the MOBI/PalmDOC path (uppercased like calibre).
const (
	pdbIdentMobi    = "BOOKMOBI"
	pdbIdentPalmDoc = "TEXTREAD"
)

type CompressionType uint16

const (
	CompressionNone    CompressionType = 1
	CompressionPalmDOC CompressionType = 2
	// CompressionPalmDOCAlt is also PalmDOC: calibre accepts compression in {2, 258}.
	CompressionPalmDOCAlt CompressionType = 258
	CompressionHUFF       CompressionType = 17480
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
)

type TextEncodingType uint32

const (
	CP1252 TextEncodingType = 1252
	UTF8   TextEncodingType = 65001
)

type Mobi struct {
	Compression         CompressionType
	TextLength          uint32
	TextRecordCount     uint16
	MaxTextRecordSize   uint16
	Encryption          EncryptionType
	Identifier          string
	HeaderLength        uint32
	Type                MobiType
	TextEncoding        TextEncodingType
	MobiVersion         uint32
	FirstNonTextRecord  uint32
	FullNameOffset      uint32 // Offset in record 0 (not from start of file)
	FullNameLength      uint32
	Locale              uint32
	InputLanguage       uint32
	OutputLanguage      uint32
	MinVersion          uint32
	FirstImageRecord    uint32
	HuffmanRecordOffset uint32
	HuffmanRecordCount  uint32
	HuffmanTableOffset  uint32
	HuffmanTableLength  uint32
	EXTHFlags           uint32 // bit 6 (0x40) set means an EXTH record is present
	DRMOffset           uint32 // 0xFFFFFFFF if no DRM
	DRMCount            uint32 // 0xFFFFFFFF if no DRM
	DRMSize             uint32
	DRMFlags            uint32
	FirstTextRecord     uint16
	LastContentRecord   uint16
	FCISRecordOffset    uint32
	FCISRecordCount     uint32
	FLISRecordOffset    uint32
	FLISRecordCount     uint32
	ExtraRecordFlags    uint32
	IndxRecordOffset    uint32 // Alias for NcxIdx, kept for compatibility.
	NcxIdx              uint32 // 0xFFFFFFFF if not present
	DivIdx              uint32 // 0xFFFFFFFF if not present
	SkelIdx             uint32 // 0xFFFFFFFF if not present
	DatpIdx             uint32 // 0xFFFFFFFF if not present
	OthIdx              uint32 // 0xFFFFFFFF if not present
	FdstIdx             uint32 // 0xFFFFFFFF if not present or count <=1
	FdstCount           uint32
	Codec               string // Go codec name derived from TextEncoding: "cp1252" or "utf-8"
	Kf8FirstImageIndex  uint32 // Absolute first image index (KF8 FirstImageRecord + KF8HeaderIndex)
	Mobi6Records        uint16 // Number of MOBI6 text records in joint files

	EXTH        *Exth // EXTH record, if present
	title       string
	name        string
	KF8         *Mobi  // In case this is a dual MOBI file
	recordCount uint16 // Number of records in the MOBI file
}

func ReadMobi(pdbDb *pdb.PdbDb, maxExthRecords int) (*Mobi, error) {
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

// newMobiDefaults sets absent-optional fields to their "not present" sentinels.
func newMobiDefaults() *Mobi {
	return &Mobi{
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
}

// readBaseHeader reads the PalmDOC base header shared by MOBI and pure-PalmDOC record 0.
func (m *Mobi) readBaseHeader(data []byte) {
	m.Compression = CompressionType(binary.BigEndian.Uint16(data[0:2]))
	m.TextLength = binary.BigEndian.Uint32(data[4:8])
	m.TextRecordCount = binary.BigEndian.Uint16(data[8:10])
	m.MaxTextRecordSize = binary.BigEndian.Uint16(data[10:12])
	m.Encryption = EncryptionType(binary.BigEndian.Uint16(data[12:14]))
}

func readMobiHeader(data []byte, maxExthRecords int) (*Mobi, error) {
	if len(data) < 96 {
		return nil, fmt.Errorf("record 0 is too short")
	}

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

	mobi := newMobiDefaults()
	mobi.readBaseHeader(data)
	mobi.Identifier = identifier
	mobi.HeaderLength = headerLength
	mobi.Type = MobiType(mobiType)
	mobi.TextEncoding = TextEncodingType(textEncoding)
	mobi.MobiVersion = mobiVersion
	mobi.FirstNonTextRecord = firstNonTextRecord
	mobi.FullNameOffset = fullNameOffset
	mobi.FullNameLength = fullNameLength
	mobi.Locale = locale
	mobi.Codec = codec

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
		// 16-bit field at 0xF2, honored only for BOOKMOBI with header length >= 0xE4.
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

// HasEXTH reports whether the file contains an EXTH record.
func (m *Mobi) HasEXTH() bool {
	return (m.EXTHFlags & 0x40) != 0
}

// HasDRM reports whether the file is DRM protected.
func (m *Mobi) HasDRM() bool {
	return m.DRMOffset != 0xFFFFFFFF
}

// isDRMProtected treats a present DRM offset or non-zero encryption as DRM, like calibre.
func isDRMProtected(m *Mobi) bool {
	if m == nil {
		return false
	}
	if m.HasDRM() || m.Encryption != EncryptionNone {
		return true
	}
	if m.KF8 != nil && (m.KF8.HasDRM() || m.KF8.Encryption != EncryptionNone) {
		return true
	}
	return false
}

// Title returns the book title, preferring the EXTH updated title.
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

// Version returns the declared format version, e.g. "6/8" for dual files.
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

// CoverRecordIdx returns the record index of the cover image, or 0 if none.
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
