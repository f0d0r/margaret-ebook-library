package config

// Config holds library-wide safety limits shared by all format readers.
// The zero value means "use the default"; callers configure reads through
// Option, which falls back to DefaultConfig for fields that are not overridden.
type Config struct {
	// MaxResourceSize bounds how many bytes may be decompressed for a single
	// resource (a cover image, a content document, etc.) in any format,
	// protecting against zip-bomb style e-books.
	MaxResourceSize int64

	// MaxRecordSize bounds the size of a single MOBI (PDB) record.
	MaxRecordSize int64

	// MaxExthRecords bounds the number of EXTH records parsed from a MOBI file.
	MaxExthRecords int

	// KF8FallbackToMOBI6 controls whether KF8 parsing falls back to MOBI6
	// on parse failures (corrupt, malformed, etc.). Default is true to
	// preserve existing behavior. Set to false to match Calibre's behavior
	// where KF8 parse failures return an error instead of silently falling back.
	KF8FallbackToMOBI6 bool
}

// DefaultConfig returns the library-wide default safety limits.
func DefaultConfig() Config {
	return Config{
		MaxResourceSize:    100 * 1024 * 1024, // 100 MB
		MaxRecordSize:      100 * 1024 * 1024, // 100 MB
		MaxExthRecords:     256,
		KF8FallbackToMOBI6: true,
	}
}
