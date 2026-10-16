// Package chm reads Microsoft Compiled HTML Help (.chm) e-books: an ITSF
// container of HTML topics, images and internal control entries.
//
// Detection is content-based (the "ITSF" magic plus header version and
// length). Directory lookup walks the ITSP/PMGL index case-insensitively,
// following chmlib. Entry payloads serve lazily through bounded reads;
// LZX-compressed sections decode per reset interval via internal/compress.
package chm
