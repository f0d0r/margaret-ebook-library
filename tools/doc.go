// Package tools provides ebook hashing and content fingerprinting for deduplication and similarity.
//
// CalculateFileHash computes hex-encoded SHA-256 over a file or Blob, while
// FingerprintContent and OpenReadingOrderWithFingerprint derive streaming
// MinHash signatures with Jaccard similarity and SimHash with Hamming
// distance over the normalized plain-text reading order. The fingerprint
// hashers implement io.Writer so they can be attached with
// io.TeeReader/io.MultiWriter in a single pass.
package tools
