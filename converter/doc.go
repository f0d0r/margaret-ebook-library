// Package converter provides a streaming MIME-to-MIME transformer registry for ebook resource conversion.
//
// The default registry ships with text to plain-text converters, and custom
// transformers can be registered or used through isolated registries.
//
// A Transformer is a single From to To edge over MIME types. Registries
// resolve direct edges and multi-hop chains via shortest-path search, and
// RegisterAliases shares one transformer logic across several source types.
// Registration is thread-safe.
package converter
