package util

// IsXMLNameChar reports whether c may appear in an XML tag or
// namespace-prefix name after the first character (ASCII subset; the
// name-matching call sites only ever need to recognize ASCII names).
func IsXMLNameChar(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	case c == '_' || c == '-' || c == '.' || c == ':':
		return true
	default:
		return false
	}
}
