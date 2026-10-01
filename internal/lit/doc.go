// Package lit reads MS Reader (.lit) e-books: an ITSF-based container of
// directory entries and storage sections holding a binary OPF package
// (/meta) plus binary content documents.
//
// The reader follows the ConvertLIT and calibre LIT readers: primary and
// secondary header parsing, IFCM directory decoding, section assembly
// through DES/LZX transforms (LZX via internal/compress), UnBinary markup
// decoding with OPF/HTML vocabularies, and manifest parsing. OPF handling
// itself lives in internal/opf and is shared with the EPUB reader.
//
// Metadata comes from the binary OPF; content and cover resources open
// lazily through book.Resource. Only passport-locked containers
// (/DRMStorage/Licenses/EUL) are rejected outright with book.ErrDRM:
// sealed/inscribed sections whose keys don't verify stay listed but fail
// with book.ErrDRM when opened. MS-era files with unbalanced binary markup
// (stray closes, early package end) are tolerated the way MS Reader accepts
// them, instead of failing like the reference decoders.
package lit
