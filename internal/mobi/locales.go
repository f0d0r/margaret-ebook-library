package mobi

import (
	"bytes"

	"golang.org/x/text/encoding/charmap"
)

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
