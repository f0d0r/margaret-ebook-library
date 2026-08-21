package model

// Config holds library-wide safety limits shared by all format readers.
// A zero value means "use the default"; clients that need different limits
// can build a Config from DefaultConfig and override individual fields.
type Config struct {
	// MaxResourceSize bounds how many bytes may be decompressed for a single
	// resource (a cover image, a content document, etc.) in any format,
	// protecting against zip-bomb style e-books.
	MaxResourceSize int64

	// MaxRecordSize bounds the size of a single MOBI (PDB) record.
	MaxRecordSize int64

	// MaxExthRecords bounds the number of EXTH records parsed from a MOBI file.
	MaxExthRecords int
}

// DefaultConfig returns the library-wide default safety limits.
func DefaultConfig() Config {
	return Config{
		MaxResourceSize: 100 * 1024 * 1024, // 100 MB
		MaxRecordSize:   100 * 1024 * 1024, // 100 MB
		MaxExthRecords:  256,
	}
}
