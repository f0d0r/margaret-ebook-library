package util

import (
	"bytes"
	"strings"

	"github.com/f0d0r/margaret-ebook-library/mediatype"
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
		return &Media{Type: mediatype.JPEG, Extension: mustExtension(mediatype.JPEG)}
	}
	if len(data) >= 8 && data[0] == 0x89 && string(data[1:4]) == "PNG" {
		return &Media{Type: mediatype.PNG, Extension: mustExtension(mediatype.PNG)}
	}
	if len(data) >= 3 && string(data[0:3]) == "GIF" {
		return &Media{Type: mediatype.GIF, Extension: mustExtension(mediatype.GIF)}
	}

	headerStr := string(bytes.TrimSpace(data))
	if strings.HasPrefix(headerStr, "<svg") || strings.Contains(headerStr, "<?xml") {
		return &Media{Type: mediatype.SVG, Extension: mustExtension(mediatype.SVG)}
	}

	return nil
}

// UnknownMedia returns the fallback Media for binary payloads whose type
// cannot be identified (octet-stream with "bin" extension by library
// convention, see the central mediatype table).
func UnknownMedia() *Media {
	return &Media{Type: mediatype.OctetStream, Extension: mustExtension(mediatype.OctetStream)}
}

// mustExtension resolves a known mediatype constant to its canonical
// extension. It panics on unknown input, so it must only be called with
// types covered by the central mediatype table.
func mustExtension(mime string) string {
	ext, ok := mediatype.Extension(mime)
	if !ok {
		panic("mediatype: missing extension for " + mime)
	}
	return ext
}
