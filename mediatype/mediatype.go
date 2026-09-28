package mediatype

import "strings"

// Canonical MIME type constants.
const (
	PlainText   = "text/plain"
	HTML        = "text/html"
	XHTML       = "application/xhtml+xml"
	MobiHTML    = "application/x-mobipocket-html"
	FB2Body     = "application/x-fictionbook-body+xml"
	JPEG        = "image/jpeg"
	JPG         = "image/jpg"
	PNG         = "image/png"
	GIF         = "image/gif"
	SVG         = "image/svg+xml"
	WEBP        = "image/webp"
	BMP         = "image/bmp"
	MSBMP       = "image/x-ms-bmp"
	OctetStream = "application/octet-stream"
	CSS         = "text/css"
	NCX         = "application/x-dtbncx+xml"
	OPF         = "application/oebps-package+xml"
)

// extByType is the single source of truth mapping MIME types to canonical
// file extensions. Keys are canonical types plus known aliases (which get no
// dedicated constants). octet-stream maps to "bin" by library convention for
// otherwise unidentifiable binary payloads.
var extByType = map[string]string{
	JPEG:        "jpg",
	JPG:         "jpg",
	PNG:         "png",
	GIF:         "gif",
	SVG:         "svg",
	WEBP:        "webp",
	BMP:         "bmp",
	MSBMP:       "bmp",
	OctetStream: "bin",
}

// Extension returns the canonical file extension for a MIME type, or false
// when unknown. The input is normalized first (case, parameters and
// surrounding whitespace are ignored), so "IMAGE/JPEG; charset=x" yields
// "jpg".
func Extension(mime string) (string, bool) {
	ext, ok := extByType[Normalize(mime)]
	return ext, ok
}

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
