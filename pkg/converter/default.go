package converter

import "github.com/f0d0r/margaret-ebook-library/pkg/mediatype"

// DefaultRegistry is the global registry pre-populated with built-in
// transformers. Resource.OpenAs uses it automatically.
var DefaultRegistry = NewRegistry()

func init() {
	// HTML variants -> plain text share the same logic via aliases.
	RegisterAliases(DefaultRegistry, func(from, to string) Transformer {
		return &htmlToTextTransformer{from: from, to: to}
	}, mediatype.PlainText,
		mediatype.XHTML,
		mediatype.HTML,
		mediatype.MobiHTML,
	)
}
