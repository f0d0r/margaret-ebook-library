// Package tools provides ebook hashing, content fingerprinting, and ISBN
// handling for deduplication and similarity.
//
// CalculateFileHash computes hex-encoded SHA-256 over a file or Blob, while
// FingerprintContent and OpenReadingOrderWithFingerprint derive streaming
// MinHash signatures with Jaccard similarity and SimHash with Hamming
// distance over the normalized plain-text reading order. FingerprintText and
// FingerprintReader apply the same normalization and hashers to arbitrary
// plain text. The fingerprint hashers implement io.Writer so they can be
// attached with io.TeeReader/io.MultiWriter in a single pass.
//
// IsValidISBN and ParseISBN normalize and validate ISBN-10/ISBN-13
// identifiers in common URN, labelled, spaced, and hyphenated forms.
// Parsed values expose their Kind and can be converted with ToISBN10 and
// ToISBN13.
package tools
