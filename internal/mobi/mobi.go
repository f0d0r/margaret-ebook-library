package mobi

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"golang.org/x/text/encoding/charmap"
)

const PALM_DOC_HEADER_SIZE = 16

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
	IndxRecordOffset    uint32 // Offset to INDX record in the file.

	EXTH        *Exth // EXTH record, if present
	title       string
	name        string
	KF8         *Mobi  // In case this is a dual MOBI file
	recordCount uint16 // Number of records in the MOBI file
}

func ReadMobi(pdbDb *PdbDb) (*Mobi, error) {
	data, err := pdbDb.PdbRecords[0].Data()
	if err != nil {
		return nil, fmt.Errorf("failed to reda Record 0: %w", err)
	}
	if len(data) < 96 {
		return nil, fmt.Errorf("record 0 is too short")
	}

	mobi, err := readMobiHeader(data)
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
				kf8, err := readMobiHeader(kf8Data)
				if err != nil {
					return nil, fmt.Errorf("failed to read KF8 header: %w", err)
				}
				
				kf8.recordCount = uint16(uint32(pdbDb.NumberOfRecords) - kf8HeaderIdx + 1)
				mobi.KF8 = kf8
			}
		}
	}

	return mobi, nil
}

func readMobiHeader(data []byte) (*Mobi, error) {
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

		DRMOffset:        0xFFFFFFFF,
		FCISRecordOffset: 0xFFFFFFFF,
		FLISRecordOffset: 0xFFFFFFFF,
		ExtraRecordFlags: 0,
		IndxRecordOffset: 0xFFFFFFFF,
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

	if len(data) >= 244 {
		mobi.ExtraRecordFlags = binary.BigEndian.Uint32(data[240:244])
	}

	if len(data) >= 248 {
		mobi.IndxRecordOffset = binary.BigEndian.Uint32(data[244:248])
	}

	if mobi.HasEXTH() {
		exthOffset := mobi.HeaderLength + PALM_DOC_HEADER_SIZE
		exth, err := ReadExth(exthOffset, data, mobi.TextEncoding)
		if err != nil {
			return nil, fmt.Errorf("failed to read EXTH record: %w", err)
		}
		mobi.EXTH = exth
	}

	if len(data) >= int(fullNameOffset+fullNameLength) {
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

func (m *Mobi) CoverRecordIdx() uint32 {
	if m.KF8 != nil {
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

// localeCode returns the BCP 47-style language tag derived from the MOBI header Locale field.
// The locale is stored as a Microsoft LCID (Language Code Identifier).
// Returns the language-region tag (e.g. "en-US", "hu", "de-DE") or empty string if unknown.
func (m *Mobi) localeCode() string {
	lcid := uint16(m.Locale & 0xFFFF)
	switch lcid {
	// Afrikaans
	case 0x0436:
		return "af"
	// Albanian
	case 0x041C:
		return "sq"
	// Arabic
	case 0x0401:
		return "ar-SA"
	case 0x0801:
		return "ar-IQ"
	case 0x0C01:
		return "ar-EG"
	case 0x1001:
		return "ar-LY"
	case 0x1401:
		return "ar-DZ"
	case 0x1801:
		return "ar-MA"
	case 0x1C01:
		return "ar-TN"
	case 0x2001:
		return "ar-OM"
	case 0x2401:
		return "ar-YE"
	case 0x2801:
		return "ar-SY"
	case 0x2C01:
		return "ar-JO"
	case 0x3001:
		return "ar-LB"
	case 0x3401:
		return "ar-KW"
	case 0x3801:
		return "ar-AE"
	case 0x3C01:
		return "ar-BH"
	case 0x4001:
		return "ar-QA"
	// Armenian
	case 0x042B:
		return "hy"
	// Azeri (Cyrillic)
	case 0x082C:
		return "az-Cyrl"
	// Azeri (Latin)
	case 0x042C:
		return "az-Latn"
	// Basque
	case 0x042D:
		return "eu"
	// Belarusian
	case 0x0423:
		return "be"
	// Bosnian (Cyrillic)
	case 0x201A:
		return "bs-Cyrl"
	// Bosnian (Latin)
	case 0x141A:
		return "bs-Latn"
	// Bulgarian
	case 0x0402:
		return "bg"
	// Catalan
	case 0x0403:
		return "ca"
	// Chinese
	case 0x0004:
		return "zh-Hans"
	case 0x0404:
		return "zh-TW"
	case 0x0804:
		return "zh-CN"
	case 0x0C04:
		return "zh-HK"
	case 0x1004:
		return "zh-SG"
	case 0x1404:
		return "zh-MO"
	case 0x7C04:
		return "zh-Hant"
	// Croatian
	case 0x041A:
		return "hr"
	// Croatian (Latin, Bosnia and Herzegovina)
	case 0x101A:
		return "hr-BA"
	// Czech
	case 0x0405:
		return "cs"
	// Danish
	case 0x0406:
		return "da"
	// Divehi
	case 0x0465:
		return "dv"
	// Dutch
	case 0x0813:
		return "nl-BE"
	case 0x0413:
		return "nl-NL"
	// English
	case 0x1009:
		return "en-CA"
	case 0x2009:
		return "en-JM"
	case 0x2409:
		return "en-029"
	case 0x2809:
		return "en-BZ"
	case 0x2C09:
		return "en-TT"
	case 0x0809:
		return "en-GB"
	case 0x1809:
		return "en-IE"
	case 0x1C09:
		return "en-ZA"
	case 0x3009:
		return "en-ZW"
	case 0x0C09:
		return "en-AU"
	case 0x1409:
		return "en-NZ"
	case 0x3409:
		return "en-PH"
	case 0x0409:
		return "en-US"
	// Estonian
	case 0x0425:
		return "et"
	// Faroese
	case 0x0438:
		return "fo"
	// Filipino
	case 0x0464:
		return "fil"
	// Finnish
	case 0x040B:
		return "fi"
	// French
	case 0x0C0C:
		return "fr-CA"
	case 0x040C:
		return "fr-FR"
	case 0x180C:
		return "fr-MC"
	case 0x100C:
		return "fr-CH"
	case 0x080C:
		return "fr-BE"
	case 0x140C:
		return "fr-LU"
	// Frisian
	case 0x0462:
		return "fy"
	// Galician
	case 0x0456:
		return "gl"
	// Georgian
	case 0x0437:
		return "ka"
	// German
	case 0x0407:
		return "de-DE"
	case 0x0807:
		return "de-CH"
	case 0x0C07:
		return "de-AT"
	case 0x1407:
		return "de-LI"
	case 0x1007:
		return "de-LU"
	// Greek
	case 0x0408:
		return "el"
	// Gujarati
	case 0x0447:
		return "gu"
	// Hebrew
	case 0x040D:
		return "he"
	// Hindi
	case 0x0439:
		return "hi"
	// Hungarian
	case 0x040E:
		return "hu"
	// Icelandic
	case 0x040F:
		return "is"
	// Indonesian
	case 0x0421:
		return "id"
	// Inuktitut (Latin)
	case 0x085D:
		return "iu-Latn"
	// Irish
	case 0x083C:
		return "ga"
	// isiXhosa
	case 0x0434:
		return "xh"
	// isiZulu
	case 0x0435:
		return "zu"
	// Italian
	case 0x0410:
		return "it-IT"
	case 0x0810:
		return "it-CH"
	// Japanese
	case 0x0411:
		return "ja"
	// Kannada
	case 0x044B:
		return "kn"
	// Kazakh
	case 0x043F:
		return "kk"
	// Kiswahili
	case 0x0441:
		return "sw"
	// Konkani
	case 0x0457:
		return "kok"
	// Korean
	case 0x0412:
		return "ko"
	// Kyrgyz
	case 0x0440:
		return "ky"
	// Latvian
	case 0x0426:
		return "lv"
	// Lithuanian
	case 0x0427:
		return "lt"
	// Luxembourgish
	case 0x046E:
		return "lb"
	// North Macedonian
	case 0x042F:
		return "mk"
	// Malay
	case 0x043E:
		return "ms-MY"
	case 0x083E:
		return "ms-BN"
	// Maltese
	case 0x043A:
		return "mt"
	// Maori
	case 0x0481:
		return "mi"
	// Mapudungun
	case 0x047A:
		return "arn"
	// Marathi
	case 0x044E:
		return "mr"
	// Mohawk
	case 0x047C:
		return "moh"
	// Mongolian (Cyrillic)
	case 0x0450:
		return "mn"
	// Nepali
	case 0x0461:
		return "ne"
	// Norwegian (Bokmål)
	case 0x0414:
		return "nb"
	// Norwegian (Nynorsk)
	case 0x0814:
		return "nn"
	// Pashto
	case 0x0463:
		return "ps"
	// Persian
	case 0x0429:
		return "fa"
	// Polish
	case 0x0415:
		return "pl"
	// Portuguese
	case 0x0416:
		return "pt-BR"
	case 0x0816:
		return "pt-PT"
	// Punjabi (Gurmukhi)
	case 0x0446:
		return "pa"
	// Quechua
	case 0x046B:
		return "qu-BO"
	case 0x086B:
		return "qu-EC"
	case 0x0C6B:
		return "qu-PE"
	// Romanian
	case 0x0418:
		return "ro"
	// Romansh
	case 0x0417:
		return "rm"
	// Russian
	case 0x0419:
		return "ru"
	// Sami, Inari
	case 0x243B:
		return "smn"
	// Sami, Lule
	case 0x143B:
		return "smj-SE"
	case 0x103B:
		return "smj-NO"
	// Sami, Northern
	case 0x043B:
		return "se-NO"
	case 0x083B:
		return "se-SE"
	case 0x0C3B:
		return "se-FI"
	// Sami, Skolt
	case 0x203B:
		return "sms"
	// Sami, Southern
	case 0x183B:
		return "sma-NO"
	case 0x1C3B:
		return "sma-SE"
	// Sanskrit
	case 0x044F:
		return "sa"
	// Serbian (Cyrillic) - Serbia (also used for Montenegro, same LCID)
	case 0x0C1A:
		return "sr-Cyrl-RS"
	// Serbian (Cyrillic, Bosnia and Herzegovina)
	case 0x1C1A:
		return "sr-Cyrl-BA"
	// Serbian (Latin) - Serbia (also used for Montenegro, same LCID)
	case 0x081A:
		return "sr-Latn-RS"
	// Serbian (Latin, Bosnia and Herzegovina)
	case 0x181A:
		return "sr-Latn-BA"
	// Sesotho sa Leboa
	case 0x046C:
		return "nso"
	// Setswana
	case 0x0432:
		return "tn"
	// Slovak
	case 0x041B:
		return "sk"
	// Slovenian
	case 0x0424:
		return "sl"
	// Spanish
	case 0x080A:
		return "es-MX"
	case 0x100A:
		return "es-GT"
	case 0x140A:
		return "es-CR"
	case 0x180A:
		return "es-PA"
	case 0x1C0A:
		return "es-DO"
	case 0x200A:
		return "es-VE"
	case 0x240A:
		return "es-CO"
	case 0x280A:
		return "es-PE"
	case 0x2C0A:
		return "es-AR"
	case 0x300A:
		return "es-EC"
	case 0x340A:
		return "es-CL"
	case 0x3C0A:
		return "es-PY"
	case 0x400A:
		return "es-BO"
	case 0x440A:
		return "es-SV"
	case 0x480A:
		return "es-HN"
	case 0x4C0A:
		return "es-NI"
	case 0x500A:
		return "es-PR"
	case 0x380A:
		return "es-UY"
	case 0x0C0A:
		return "es-ES"
	case 0x040A:
		return "es-ES"
	// Swedish
	case 0x041D:
		return "sv-SE"
	case 0x081D:
		return "sv-FI"
	// Syriac
	case 0x045A:
		return "syr"
	// Tamil
	case 0x0449:
		return "ta"
	// Tatar
	case 0x0444:
		return "tt"
	// Telugu
	case 0x044A:
		return "te"
	// Thai
	case 0x041E:
		return "th"
	// Turkish
	case 0x041F:
		return "tr"
	// Ukrainian
	case 0x0422:
		return "uk"
	// Urdu
	case 0x0420:
		return "ur"
	// Uzbek (Cyrillic)
	case 0x0843:
		return "uz-Cyrl"
	// Uzbek (Latin)
	case 0x0443:
		return "uz-Latn"
	// Vietnamese
	case 0x042A:
		return "vi"
	// Welsh
	case 0x0452:
		return "cy"
	}
	return ""
}

func decodeString(data []byte, textEncoding TextEncodingType) string {
	if data == nil {
		return ""
	}

	trimmedData := bytes.TrimSpace(data)

	if textEncoding == CP1252 {
		decodedData, err := charmap.Windows1252.NewDecoder().Bytes(trimmedData)
		if err != nil {
			return string(trimmedData)
		}
		return string(decodedData)
	}
	return string(trimmedData)
}
