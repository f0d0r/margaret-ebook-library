// Charset handling for Resource content.
//
// Contract:
//   - Resource.Open returns the stored bytes byte-identically (after
//     decompression/decryption, but before any character-set transcoding).
//     Resource.Charset names the encoding of those bytes.
//   - Resource.Charset is never empty on resources returned by Read:
//     textual resources carry an explicit IANA label ("utf-8" when nothing
//     else is declared), binary resources carry "binary".
//   - Transcoding happens only in OpenAs/DataAs (input decoded to UTF-8
//     centrally, output optionally re-encoded to a requested charset) and
//     in Metadata (Go strings are UTF-8 by definition).
//
// The label normalization, media-type classification, declaration sniffing
// and stream transcode helpers live in internal/charset; this file carries
// only the Resource-level charset semantics.
package book

import "github.com/f0d0r/margaret-ebook-library/internal/charset"

// Charset labels used in Resource.Charset.
const (
	// CharsetUTF8 marks textual content stored as UTF-8 (explicit default
	// when no other encoding is declared).
	CharsetUTF8 = charset.UTF8
	// CharsetBinary marks non-textual content (images, octet-stream, ...)
	// for which a character set is meaningless. It is a sentinel, not a
	// real encoding: never passed to a decoder.
	CharsetBinary = charset.Binary
)

// effectiveCharset returns the charset governing r's stored bytes:
// the normalized Resource.Charset, or the media-type default when unset
// (hand-built Resources, older callers). It never returns "".
func (r *Resource) effectiveCharset() string {
	if r == nil {
		return charset.Binary
	}
	if c := charset.Normalize(r.Charset); c != "" {
		return c
	}
	return charset.Default(r.MediaType)
}

// normalizeCharsetInPlace fills an empty Charset from the media type and
// canonicalizes a set one. Library readers should set Charset explicitly;
// this is the backstop (applied by NewResourceSet) so "" never escapes.
func (r *Resource) normalizeCharsetInPlace() {
	if r == nil {
		return
	}
	if r.Charset == "" {
		r.Charset = charset.Default(r.MediaType)
		return
	}
	r.Charset = charset.Normalize(r.Charset)
	if r.Charset == "" {
		r.Charset = charset.Default(r.MediaType)
	}
}
