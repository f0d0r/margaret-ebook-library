# Changelog

## [Unreleased]

### Added

- Series extraction from EPUB/KEPUB, LIT, PalmDOC, FB2 and FB3 metadata:
  `Metadata.Series` and `Metadata.SeriesIndex`. EPUB 3
  `belongs-to-collection` with a `collection-type=series` refinement and
  `group-position` is preferred, with `calibre:series`/`calibre:series_index`
  as fallback; FB2 uses the first non-empty `title-info/sequence` with
  `publish-info/sequence` fallback; FB3 uses the outermost direct
  `fb3-description/sequence` with `paper-publish-info/sequence` fallback.
  Values are trimmed, the index is kept verbatim (no float conversion or
  Calibre-style `1.0` default), and the first non-empty series wins. Canonical
  EPUB 3 series without `group-position` yield an empty index. MOBI/KF8,
  eReader, CHM and LRF expose no series metadata and leave both fields empty.
- ISBN-10/ISBN-13 handling in `tools`: `IsValidISBN` and `ParseISBN`
  normalize and validate common URN (`urn:isbn:`), labelled, spaced, and
  hyphenated forms. Parsed values report their `Kind` and convert with
  `ToISBN10`/`ToISBN13`; 979-prefixed ISBN-13 values have no ISBN-10
  equivalent.
- Subject extraction from EPUB/KEPUB, LIT, PalmDOC, MOBI, KF8, FB2, FB3 and
  LRF metadata: `Metadata.Subjects` exposes subject values in source order.
  Semicolon-separated values are split into separate entries; parts are trimmed
  and empty entries are dropped. Commas and duplicates are preserved, and source
  order is kept. OEB 1.x `dc-metadata/Subject` values are included. For
  MOBI/KF8, EXTH record 105 (subject) is used, including multiple records,
  with a non-empty KF8 subject list preferred over the MOBI6 EXTH values.
  For FB2, `title-info/genre` elements are combined with comma-separated
  `title-info/keywords`, falling back to `src-title-info` as a whole; keyword
  values are also comma-split. For FB3, `fb3-classification/subject` elements
  are used with whitespace collapsed; `fb3-relations/subject` values are not
  subjects. For LRF, repeatable `BookInfo/Category` elements and
  `BookInfo/Classification` are split on commas and semicolons, while
  `Info/Keyword` elements are used whole; `BookInfo/Label` is ignored. FB2, FB3
  and LRF subject lists are deduplicated case-insensitively, keeping the first
  occurrence. Elsewhere values are not deduplicated or mapped to any
  vocabulary.
- Publisher and raw publication-date extraction from EPUB, LIT, MOBI/KF8,
  eReader, LRF, FB2 and FB3 metadata: `Metadata.Publisher` and
  `Metadata.PublicationDate`. The date is returned as declared, without parsing
  or validation. For EPUB and LIT, only an EPUB 2 date with
  `event="publication"` or a date without an event is considered; dates with
  any other event are ignored, and an explicit publication date is preferred.
  For MOBI/KF8 the EXTH 101 (publisher) and 106 (publication date) records are
  used; PalmDOC HTML metadata is parsed the same way. For FB2 the
  `<publish-info><publisher>` and `<year>`/`<date>` elements are used. For FB3
  the `<paper-publish-info>` `publisher` and `year` attributes are used.
  eReader and LRF expose only the publisher; no publication date is read for
  them. CHM reads declared default-topic meta tags and calibre-style
  `span`/table heuristics; publisher is exposed, ISBN-specific candidates are
  reduced to digits, checksum-validated, and stored as canonical digits.
- Identifier extraction from formats that declare them (EPUB, MOBI/KF8,
  eReader, PalmDOC, FB2/FB3 and LIT): `Metadata.Identifiers` exposes raw
  scheme/value pairs and `Metadata.Identifier` provides case-insensitive
  lookup.
- eReader reading (`.pdb`, Peanut Press `PNRdPPrs`/`PNPdPPrs`): new
  `book.EREADER` file type (`ebook.EREADER`) with title/author metadata from
  the Dropbook (132-byte record 0) metadata record, PDB name fallback for
  Makebook files, and `book.ErrDRM` for encrypted books. Lazily-opened
  content (`index.pml`), non-linear footnote/sidebar resources and images
  with `cover.png` cover detection; PML bytes are served as stored and a
  `converter` transformer renders them to plain text on demand.
- PalmDOC reading (`.prc`, `.pdb`, `TEXtREAd`): normalized metadata, single
  lazily-opened content resource and linear reading order from the 16-byte
  record 0 header. The decompressed prefix decides only the media type
  (`text/html` for OEB 1.0 payloads via the new `opf.ParseMetadata`, else
  `text/plain` with the PDB name as title); payload bytes are preserved
  as-is for the existing transformers. New `book.PALMDOC` file type
  (`ebook.PALMDOC`); encrypted books report `book.ErrDRM`.
- LIT reading (`.lit`, MS Reader): normalized metadata, spine reading order
  and lazily-opened content/cover resources from OEB 1.x packages (ITSF
  container, LZX sections, binary OPF/content decoding). Sealed content stays listed but fails with
  `ErrDRM` on `Open` (section decryption is out of scope); passport-locked
  (`/DRMStorage/Licenses/EUL`) files are rejected outright. MS-era files
  with unbalanced binary markup are tolerated like MS Reader.
- Shared `internal/opf` package: OPF parsing, metadata extraction and
  OPF-driven resource assembly (`BuildResources`) used by both the EPUB and
  the LIT readers; `internal/zip` renamed to `internal/compress` hosting it
  alongside a streaming LZX decompressor.
- FB2 reading (`.fb2`, `.fbz`, `.fb2.zip`, plain or zipped XML):
  normalized metadata, raw body XML content resources (`application/x-fictionbook-body+xml`),
  spine-like reading order (notes bodies marked non-linear), transformer
  to plain text, embedded images and cover image.
- FB3 reading (`.fb3`, OPC ZIP container): format detection from
  `[Content_Types].xml`, root relationships and `fb3-description`
  sniffing; normalized metadata (title, authors, description, languages)
  from the description part; lazily-opened body (`application/fb3-body+xml`,
  with a plain-text transformer) and image resources plus thumbnail cover
  from the body/root relationships.
- `WithKF8FallbackToMOBI6` option controlling whether a KF8 parse failure in a
  joint MOBI6+KF8 file falls back to the MOBI6 content (default `true`).
- CHM reading (`.chm`, Microsoft Compiled HTML Help): content-based ITSF
  detection, normalized metadata (title from `#SYSTEM` with default-topic
  HTML and `#TOPICS` fallbacks, authors/description from topic meta tags,
  languages from the LCID), HHC/`#TOPICS` reading order and lazily-opened
  manifest resources (raw bytes, `text/html` for topics) with `alt=cover`
  cover detection. Truncated tails fail loudly with `ErrCorrupt` instead
  of serving short data. New `book.CHM` file type (`ebook.CHM`).
- LRF reading (`.lrf`, Sony BBeB): normalized metadata from the compressed
  info block, a lazily synthesized UTF-16LE text flow as the single linear
  reading-order resource (`application/x-lrf-text`, plain-text transformer),
  embedded images and fonts, and the header thumbnail as cover. New
  `book.LRF` file type (`ebook.LRF`).
- `Resource.Charset` reporting the storage encoding of every resource's
  bytes (`book.CharsetUTF8`/`book.CharsetBinary`, re-exported as
  `ebook.CharsetUTF8`/`ebook.CharsetBinary`); never empty on resources
  returned by `Read`. Format readers fill it from their container knowledge
  (FB2 document declaration, MOBI `TextEncoding`, eReader's fixed CP1252)
  or a bounded declaration sniff (EPUB/FB3 entries, CHM topics with LCID
  fallback); the media-type default decides the rest.
- `Resource.OpenAsWithCharset` / `DataAsWithCharset` /
  `ResourceSet.OpenReadingOrderAsWithCharset` (and the `...WithRegistryCharset`
  variants) to request a non-UTF-8 output charset for converted content;
  unknown labels error instead of producing mojibake.
- Shared `internal/xml` package: one `encoding/xml` CharsetReader
  (UTF-8/UTF-16 pass-through, x/text decoding, unknown labels rejected)
  replacing the per-reader copies in FB2, FB3, OPF and EPUB, plus the
  `IsNameChar`/`ScanOpenTag` sniffing helpers (formerly in `internal/util`).

### Changed

- `Resource.Open`/`Data` now guarantee byte-identical stored bytes across
  every format: the filesystem/container bytes after decompression (or DRM
  decryption) are served untouched, with XML declarations, BOMs and the
  original encoding preserved. This fixes FB2 bodies, MOBI/PalmDOC content
  and other text resources that were previously normalized to UTF-8.
  LIT spine documents remain the documented exception: they are synthesized
  by the reader and always carry a UTF-8 declaration.
- Charset handling is centralized in the `book` layer: `OpenAs`/`DataAs`
  decode the stored bytes to UTF-8 before the transformer chain and
  optionally re-encode the result. Transformers (including
  `converter/pmltotext`, which now expects UTF-8 input) no longer need
  per-format encoding logic. `OpenAs` output is UTF-8 unless a charset is
  requested explicitly.
- Text media classification treats any `+xml`/`+json` media type plus
  `application/xml`/`application/json` as text, so EPUB 3 SMIL/page-map and
  JSON resources get a charset instead of `"binary"`.
- FictionBook generations share one file type: `book.FB2` and `book.FB3`
  are replaced by a single `book.FB` (`ebook.FB`); `Version` tells the
  generations apart ("2.x" from the FB2 namespace, "3.0" for FB3 — FB3
  files declare no format version, so the reader reports the family
  generation implied by the container).
- KF8/MOBI8 reading is no longer experimental: parse failures no longer fall
  back silently, they are reported through the content resource's `Open` as
  `ErrCorrupt` / `ErrParseFailed` unless the MOBI6 fallback is enabled.
- Panic recovery for malformed KF8 input is centralized in one helper and
  classifies recovered panics as `ErrCorrupt`.
- A zero-value `Config` now keeps the documented defaults (limits and KF8
  fallback) instead of silently disabling the fallback.

### Fixed

- EPUB/OPF/FB3 containers stored as UTF-16 (BOM-carrying
  `encoding="UTF-16"` files, as written by Windows tools) failed with
  "invalid UTF-8" on the first XML parse: `encoding/xml` cannot scan
  UTF-16 bytes and never reaches the encoding declaration. The shared
  `internal/xml` decoder now recognizes a UTF-16 BOM (or a
  BOM-less UTF-16 XML prolog) and transcodes to UTF-8 before scanning.
  UTF-16 content documents are also labeled correctly (`Charset`
  "utf-16be"/"utf-16le"): declarations are detected with or without BOM,
  decoding consumes the BOM, and requesting a UTF-16 output charset
  writes a BOM.
- `OpenAsWithCharset`/`DataAsWithCharset` with an explicit `"utf-8"` output
  on an identity conversion no longer returned the stored bytes unconverted
  when the resource charset was not UTF-8; the content is transcoded now.
  `""` (and the stored charset itself) still means raw passthrough.
- Charset sniffing no longer treats `encoding="..."` appearing in HTML text
  or attributes as an XML declaration: the prolog pattern now requires a
  literal `<?xml`.
- FB2 documents stored as BOM-less UTF-16 (declared `utf-16`/`utf-16le`/
  `utf-16be`) are read correctly: the declaration is sniffed through the
  NUL-interleaved prolog, the declared label is resolved to the concrete
  byte order, and raw body ranges honor the detected BOM/byte order instead
  of assuming a 2-byte BOM.
- KF8 content is no longer dropped when the index tables cannot be read: the
  decompressed markup is served as a single resource.

## [0.1.1] - 2026-09-18

### Fixed

- MOBI6 books that leave `FirstTextRecord` as the `0xFFFF` sentinel no
  longer yield an empty reading order; text records are read from record 1.
- Truncated MOBI/PRC files (record table running past EOF) now return the
  available content instead of failing the whole book; `MaxRecordSize`
  limits still apply.

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
