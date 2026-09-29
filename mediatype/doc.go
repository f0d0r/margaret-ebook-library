// Package mediatype defines canonical ebook MIME types and normalization helpers.
//
// It is the single source of truth for MIME constants such as plain text,
// HTML variants and image types, mapping them to canonical file extensions
// where applicable.
//
// Normalize lowercases a MIME type and strips parameters and surrounding
// whitespace so lookups stay case-insensitive and charset-tolerant.
package mediatype
