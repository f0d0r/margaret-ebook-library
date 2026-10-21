// Package ebook reads EPUB, MOBI, KF8/AZW3, PalmDOC, eReader, FB
// (FictionBook 2/3), LIT, CHM and LRF ebooks through a single stable Book
// abstraction.
//
// Instead of surfacing format-specific internals such as EPUB manifests and
// spines or MOBI PDB records, it hides those details behind normalized
// metadata and a unified ResourceSet.
//
// Entry points are Read, ReadFromBlob and ReadFromFile. Resources stream
// lazily without loading the whole file into memory, and conservative safety
// limits apply unless overridden with functional options.
package ebook
