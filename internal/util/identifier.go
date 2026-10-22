package util

import "strings"

// CanonicalScheme folds common identifier scheme aliases to a stable label
// while leaving unknown schemes untouched.
func CanonicalScheme(scheme string) string {
	scheme = strings.TrimSpace(scheme)
	upper := strings.ToUpper(scheme)
	switch {
	case strings.HasPrefix(upper, "ISBN"):
		return "ISBN"
	case upper == "DOI":
		return "DOI"
	case upper == "UUID":
		return "UUID"
	case upper == "ASIN":
		return "ASIN"
	case upper == "ISSN":
		return "ISSN"
	case upper == "URL", upper == "URI":
		return "URL"
	default:
		return scheme
	}
}

// SplitIdentifier resolves the scheme of an identifier and removes a matching
// scheme prefix from its value. explicitScheme, when non-empty, takes
// precedence; otherwise a prefix such as "isbn:", "calibre:" or "urn:uuid:"
// is recognized. URLs keep their full value. Values are not validated or
// normalized.
func SplitIdentifier(value, explicitScheme string) (scheme, id string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ""
	}

	explicit := CanonicalScheme(explicitScheme)
	if explicit != "" {
		if stripped, ok := stripSchemePrefix(value, explicit); ok {
			return explicit, strings.TrimSpace(stripped)
		}
		return explicit, value
	}

	if looksLikeURL(value) {
		return "URL", value
	}
	if isURN(value) {
		rest, prefix, ok := splitURN(value)
		if !ok {
			return "", value
		}
		return CanonicalScheme(prefix), strings.TrimSpace(rest)
	}
	if prefix, rest, ok := splitScheme(value); ok {
		return CanonicalScheme(prefix), strings.TrimSpace(rest)
	}
	return "", value
}

// stripSchemePrefix removes a prefix that matches scheme, including the
// urn:<scheme>: form. URL values are never rewritten.
func stripSchemePrefix(value, scheme string) (string, bool) {
	if looksLikeURL(value) {
		return "", false
	}
	if isURN(value) {
		rest, prefix, ok := splitURN(value)
		if !ok || !strings.EqualFold(CanonicalScheme(prefix), scheme) {
			return "", false
		}
		return rest, true
	}
	prefix, rest, ok := splitScheme(value)
	if !ok || !strings.EqualFold(CanonicalScheme(prefix), scheme) {
		return "", false
	}
	return rest, true
}

// splitURN splits "urn:<prefix>:<rest>" into its prefix and remainder.
func splitURN(value string) (rest, prefix string, ok bool) {
	rest = value[4:]
	i := strings.IndexByte(rest, ':')
	if i <= 0 {
		return "", "", false
	}
	prefix = rest[:i]
	if !validSchemeToken(prefix) {
		return "", "", false
	}
	return rest[i+1:], prefix, true
}

func isURN(value string) bool {
	return len(value) >= 4 && strings.EqualFold(value[:4], "urn:")
}

// splitScheme splits "<prefix>:<rest>" when prefix is a valid scheme token.
func splitScheme(value string) (prefix, rest string, ok bool) {
	i := strings.IndexByte(value, ':')
	if i <= 0 {
		return "", "", false
	}
	prefix = value[:i]
	if !validSchemeToken(prefix) {
		return "", "", false
	}
	return prefix, value[i+1:], true
}

// looksLikeURL reports whether value is a URL or a hierarchical URI.
func looksLikeURL(value string) bool {
	if i := strings.Index(value, "://"); i > 0 {
		return validSchemeToken(value[:i])
	}
	prefix, _, ok := splitScheme(value)
	if !ok {
		return false
	}
	switch strings.ToLower(prefix) {
	case "http", "https", "ftp":
		return true
	default:
		return false
	}
}

// validSchemeToken reports whether s matches the RFC 3986 scheme grammar.
func validSchemeToken(s string) bool {
	if s == "" || !isAlpha(s[0]) {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if !isAlpha(c) && !isDigit(c) && c != '+' && c != '-' && c != '.' {
			return false
		}
	}
	return true
}

func isAlpha(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
