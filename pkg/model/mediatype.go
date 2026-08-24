package model

import "github.com/f0d0r/margaret-ebook-library/pkg/mediatype"

// Canonical MIME type constants — compile-time safe aliases for callers that
// work with model.Resource.MediaType. The canonical definitions live in
// pkg/mediatype to avoid import cycles between model and converter.
const (
	MediaTypePlainText   = mediatype.PlainText
	MediaTypeHTML        = mediatype.HTML
	MediaTypeXHTML       = mediatype.XHTML
	MediaTypeMobiHTML    = mediatype.MobiHTML
	MediaTypeJPEG        = mediatype.JPEG
	MediaTypePNG         = mediatype.PNG
	MediaTypeGIF         = mediatype.GIF
	MediaTypeSVG         = mediatype.SVG
	MediaTypeOctetStream = mediatype.OctetStream
	MediaTypeCSS         = mediatype.CSS
	MediaTypeNCX         = mediatype.NCX
	MediaTypeOPF         = mediatype.OPF
)

// NormalizeMediaType is a convenience alias for mediatype.Normalize.
func NormalizeMediaType(m string) string {
	return mediatype.Normalize(m)
}
