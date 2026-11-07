// Package tools provides ebook hashing, content fingerprinting, and ISBN
// handling for deduplication and similarity.
//
// CalculateFileHash computes hex-encoded SHA-256 over a file or Blob, while
// FingerprintContent and OpenReadingOrderWithFingerprint derive streaming
// MinHash signatures with Jaccard similarity and SimHash with Hamming
// distance over the normalized plain-text reading order. Normalization applies
// NFKC, Unicode full case folding, punctuation/symbol word boundaries, removal
// of invisible format characters and line-end hyphen joining. An ASCII hyphen
// at the end of a line is removed when the next line starts with a lowercase or
// caseless letter, so "balatoncsilla-\ngon" and "balatoncsillagon" yield the
// same tokens. Mid-line hyphens, uppercase/digit/punctuation continuations and
// blank-line boundaries keep their delimiter behaviour; other hyphen-like
// characters (U+2010, U+2011, U+2013, U+00AC) are not joined. FingerprintText
// and FingerprintReader apply the same normalization and hashers to arbitrary
// plain text. The
// fingerprint hashers implement io.Writer so they can be attached with
// io.TeeReader/io.MultiWriter in a single pass.
//
// ScanReadingOrder is the single-pass pipeline behind FingerprintContent and
// FindBookISBNs. Callers attach processors that implement the capabilities
// they need (TextProcessor, HeadProcessor, FallbackProcessor, FinishProcessor);
// the pipeline discovers the capabilities and feeds every processor from one
// plain-text reading order. Processors keep their own results, so callers read
// them from the processor instances after the scan.
//
// NormalizeISBN parses and checksum-validates one ISBN-10/ISBN-13 candidate in
// common URN, labelled, spaced, hyphenated and Unicode-digit forms. Normalized
// values expose their Kind and convert with ToISBN10 and ToISBN13.
// ValidateBookISBN and IsValidISBN add the book-level check that rejects known
// placeholder values; ISBN.PlaceholderKind reports the reason.
//
// FindISBNs finds book-valid ISBNs in arbitrary text. FindBookISBN returns the
// first book-valid ISBN from a book's reading order, while FindBookISBNs scans
// the linear reading-order head, non-linear spine items, and manifest-only
// textual resources.
package tools
