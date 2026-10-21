package chm

// lcidLanguage maps a Windows LCID to a BCP47 language tag using the primary
// language ID (low 10 bits), with sublanguage cases where it matters.
// Unknown IDs yield "" so callers emit no Languages entry instead of a
// wrong one.
func lcidLanguage(lcid uint32) string {
	switch lcid {
	case 0x0804, 0x1004:
		return "zh-Hans"
	case 0x0404, 0x0C04, 0x1404:
		return "zh-Hant"
	case 0x081A, 0x0C1A:
		return "sr-Latn"
	}
	switch lcid & 0x3FF {
	case 0x01:
		return "ar"
	case 0x02:
		return "bg"
	case 0x03:
		return "ca"
	case 0x04:
		return "zh-Hans"
	case 0x05:
		return "cs"
	case 0x06:
		return "da"
	case 0x07:
		return "de"
	case 0x08:
		return "el"
	case 0x09:
		return "en"
	case 0x0A:
		return "es"
	case 0x0B:
		return "fi"
	case 0x0C:
		return "fr"
	case 0x0D:
		return "he"
	case 0x0E:
		return "hu"
	case 0x0F:
		return "is"
	case 0x10:
		return "it"
	case 0x11:
		return "ja"
	case 0x12:
		return "ko"
	case 0x13:
		return "nl"
	case 0x14:
		return "no"
	case 0x15:
		return "pl"
	case 0x16:
		return "pt"
	case 0x17:
		return "rm"
	case 0x18:
		return "ro"
	case 0x19:
		return "ru"
	case 0x1A:
		return "sr"
	case 0x1B:
		return "sk"
	case 0x1C:
		return "sq"
	case 0x1D:
		return "sv"
	case 0x1E:
		return "th"
	case 0x1F:
		return "tr"
	case 0x20:
		return "ur"
	case 0x21:
		return "id"
	case 0x22:
		return "uk"
	case 0x23:
		return "be"
	case 0x24:
		return "sl"
	case 0x25:
		return "et"
	case 0x26:
		return "lv"
	case 0x27:
		return "lt"
	case 0x2C:
		return "az"
	case 0x2D:
		return "eu"
	case 0x2E:
		return "hsb"
	case 0x2F:
		return "mk"
	case 0x36:
		return "af"
	case 0x37:
		return "ka"
	case 0x39:
		return "hi"
	case 0x3E:
		return "ms"
	case 0x41:
		return "sw"
	case 0x43:
		return "uz"
	case 0x44:
		return "tt"
	case 0x45:
		return "bn"
	case 0x46:
		return "pa"
	case 0x47:
		return "gu"
	case 0x48:
		return "or"
	case 0x49:
		return "ta"
	case 0x4A:
		return "te"
	case 0x4B:
		return "kn"
	case 0x4C:
		return "ml"
	case 0x4D:
		return "as"
	case 0x4E:
		return "mr"
	case 0x4F:
		return "sa"
	case 0x50:
		return "mn"
	case 0x52:
		return "cy"
	case 0x53:
		return "kh"
	case 0x54:
		return "lo"
	case 0x56:
		return "gl"
	case 0x57:
		return "kok"
	case 0x5E:
		return "vi"
	case 0x61:
		return "ne"
	case 0x62:
		return "fy"
	}
	return ""
}

// lcidCharset maps a Windows LCID to an IANA charset name for the ANSI
// #SYSTEM/#STRINGS strings, using the Windows encoding chain with a cp1252
// fallback.
func lcidCharset(lcid uint32) string {
	switch lcid {
	case 0x0411:
		return "shift_jis"
	case 0x0412:
		return "euc-kr"
	case 0x0804, 0x1004:
		return "gbk"
	case 0x0404, 0x0C04, 0x1404:
		return "big5"
	case 0x041E:
		return "windows-874"
	case 0x042A:
		return "windows-1258"
	}
	switch lcid & 0x3FF {
	case 0x05, 0x0E, 0x15, 0x1B, 0x1A, 0x18, 0x24, 0x25, 0x26, 0x27, 0x2E:
		return "windows-1250"
	case 0x02, 0x19, 0x22, 0x23, 0x43, 0x44, 0x2F:
		return "windows-1251"
	case 0x08:
		return "windows-1253"
	case 0x1F, 0x2C:
		return "windows-1254"
	case 0x0D:
		return "windows-1255"
	case 0x01, 0x20:
		return "windows-1256"
	case 0x3E:
		return "windows-1257"
	}
	return "windows-1252"
}
