# Changelog

## [Unreleased]

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

[Unreleased]: https://github.com/f0d0r/margaret-ebook-library/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/f0d0r/margaret-ebook-library/releases/tag/v0.1.0
