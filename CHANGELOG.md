# Changelog

## [Unreleased]

### Added

- FB2 reading (`.fb2`, `.fbz`, `.fb2.zip`, plain or zipped XML):
  normalized metadata, raw body XML content resources (`application/x-fictionbook-body+xml`),
  spine-like reading order (notes bodies marked non-linear), transformer
  to plain text, embedded images and cover image. Detection and metadata
  follow calibre's `get_fb2_data` and `metadata/fb2.py` rules.

## [0.1.1] - 2026-09-18

### Fixed

- MOBI6 books that leave `FirstTextRecord` as the `0xFFFF` sentinel no
  longer yield an empty reading order; text records are read from record
  1 like calibre does.
- Truncated MOBI/PRC files (record table running past EOF) now return
  the available content instead of failing the whole book, mirroring
  calibre's lenient section slicing; `MaxRecordSize` limits still apply.

## [0.1.0] - 2026-09-11

### Added

- EPUB (`.epub`) reading: normalized metadata, manifest resources, spine
  reading order and cover image.
- MOBI reading: MOBI6 (PalmDOC) is stable; KF8/MOBI8 is experimental
  best-effort with silent fallback to the MOBI6 content (see README).
- Single stable `Book` abstraction with normalized metadata and a unified
  `ResourceSet` (O(1) lookup by id and resolved href, duplicate handling).
- Lazy streaming resource access (`Resource.Open`) without loading the whole
  file into memory.
- On-the-fly text conversion (`OpenAs`/`DataAs`) to plain text for search
  indexing, LLM input and TTS.
- Content fingerprinting in `tools/`: streaming MinHash signatures with
  Jaccard similarity and SimHash with Hamming distance.
- Conservative safety limits (max resource/record sizes, max EXTH records)
  with functional-option overrides.

[Unreleased]: https://github.com/f0d0r/margaret-ebook-library/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/f0d0r/margaret-ebook-library/releases/tag/v0.1.1
[0.1.0]: https://github.com/f0d0r/margaret-ebook-library/releases/tag/v0.1.0
