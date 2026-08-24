package mediatype

import "strings"

// Canonical MIME type constants.
const (
	PlainText   = "text/plain"
	HTML        = "text/html"
	XHTML       = "application/xhtml+xml"
	MobiHTML    = "application/x-mobipocket-html"
	JPEG        = "image/jpeg"
	PNG         = "image/png"
	GIF         = "image/gif"
	SVG         = "image/svg+xml"
	OctetStream = "application/octet-stream"
	CSS         = "text/css"
	NCX         = "application/x-dtbncx+xml"
	OPF         = "application/oebps-package+xml"
)

// Normalize lowercases the MIME type, strips parameters (e.g. "; charset=utf-8"),
// and trims surrounding whitespace. Empty input returns "".
func Normalize(m string) string {
	m = strings.TrimSpace(m)
	if m == "" {
		return ""
	}
	if idx := strings.Index(m, ";"); idx != -1 {
		m = strings.TrimSpace(m[:idx])
	}
	return strings.ToLower(m)
}
