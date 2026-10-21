package converter

import "github.com/f0d0r/margaret-ebook-library/mediatype"

// DefaultRegistry is the global registry pre-populated with built-in
// transformers. Resource.OpenAs uses it automatically.
var DefaultRegistry = NewRegistry()

func init() {
	// HTML variants -> plain text
	RegisterAliases(DefaultRegistry, func(from, to string) Transformer {
		return &htmlToTextTransformer{from: from, to: to}
	}, mediatype.PlainText,
		mediatype.XHTML,
		mediatype.HTML,
		mediatype.MobiHTML,
		mediatype.OEB1Document,
	)
	// FictionBook body fragments -> plain text
	DefaultRegistry.Register(&fb2BodyToTextTransformer{from: mediatype.FB2Body, to: mediatype.PlainText})
	// FB3 body parts -> plain text
	DefaultRegistry.Register(&fb3BodyToTextTransformer{from: mediatype.FB3Body, to: mediatype.PlainText})
	// eReader PML -> plain text
	DefaultRegistry.Register(&pmlToTextTransformer{from: mediatype.EreaderPML, to: mediatype.PlainText})
	// LRF text flow -> plain text
	DefaultRegistry.Register(&lrfToTextTransformer{from: mediatype.LRFText, to: mediatype.PlainText})
}
