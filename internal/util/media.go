package util

import (
	"bytes"
	"strings"

	"github.com/f0d0r/margaret-ebook-library/pkg/mediatype"
)

type Media struct {
	Type      string
	Extension string
}

func DetectImageMedia(data []byte) *Media {
	if len(data) == 0 {
		return nil
	}

	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return &Media{Type: "image/jpeg", Extension: "jpg"}
	}
	if len(data) >= 8 && data[0] == 0x89 && string(data[1:4]) == "PNG" {
		return &Media{Type: "image/png", Extension: "png"}
	}
	if len(data) >= 3 && string(data[0:3]) == "GIF" {
		return &Media{Type: "image/gif", Extension: "gif"}
	}

	headerStr := string(bytes.TrimSpace(data))
	if strings.HasPrefix(headerStr, "<svg") || strings.Contains(headerStr, "<?xml") {
		return &Media{Type: "image/svg+xml", Extension: "svg"}
	}

	return nil
}

// NormalizeMediaType normalizes a MIME type for comparison: lowercases,
// strips parameters (e.g. "; charset=utf-8"), and trims whitespace.
// Empty input returns "". This is a convenience re-export of
// pkg/mediatype.Normalize for internal callers that already import internal/util.
func NormalizeMediaType(m string) string {
	return mediatype.Normalize(m)
}

// Normalize is an alias for NormalizeMediaType.
func Normalize(m string) string {
	return mediatype.Normalize(m)
}
