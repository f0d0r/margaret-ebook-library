package book

// FileType represents a supported e-book file format.
type FileType string

const (
	// EPUB represents the Electronic Publication format (.epub).
	EPUB FileType = "epub"

	// MOBI represents the Mobipocket e-book format (.mobi).
	MOBI FileType = "mobi"

	// FB represents the FictionBook e-book format family (.fb2, .fb3).
	FB FileType = "fb"

	// LIT represents the Microsoft Reader e-book format (.lit).
	LIT FileType = "lit"

	// PALMDOC represents the PalmDOC e-book format (.prc, .pdb).
	PALMDOC FileType = "palmdoc"

	// EREADER represents the Peanut Press eReader format (.pdb).
	// It shares the PDB container with MOBI/PalmDOC but carries the
	// "PNRdPPrs"/"PNPdPPrs" type/creator ident and PML payloads
	// (Makebook 116/202-byte vs Dropbook 132-byte record 0 headers).
	EREADER FileType = "ereader"

	// CHM represents the Microsoft Compiled HTML Help format (.chm).
	CHM FileType = "chm"

	// LRF represents the Sony BBeB LRF e-book format (.lrf).
	LRF FileType = "lrf"
)
