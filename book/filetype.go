package book

// FileType represents a supported e-book file format.
type FileType string

const (
	// EPUB represents the Electronic Publication format (.epub).
	EPUB FileType = "epub"

	// MOBI represents the Mobipocket e-book format (.mobi).
	MOBI FileType = "mobi"

	// FB represents the FictionBook e-book format family (.fb2, .fb3).
	// Generations share the brand but not the container: FB2 is a
	// single-XML document, FB3 an OPC ZIP container. Use Version to tell
	// them apart ("2.x" for FB2, "3.0" for FB3).
	FB FileType = "fb"

	// LIT represents the Microsoft Reader e-book format (.lit).
	LIT FileType = "lit"

	// PALMDOC represents the PalmDOC e-book format (.prc, .pdb).
	// It shares the PDB container with MOBI but carries a 16-byte
	// record 0 header and plain-text or OEB 1.0 HTML payloads.
	PALMDOC FileType = "palmdoc"

	// EREADER represents the Peanut Press eReader format (.pdb).
	// It shares the PDB container with MOBI/PalmDOC but carries the
	// "PNRdPPrs"/"PNPdPPrs" type/creator ident and PML payloads
	// (Makebook 116/202-byte vs Dropbook 132-byte record 0 headers).
	EREADER FileType = "ereader"
)
