package model

// Config holds library-wide safety limits shared by all format readers.
// A zero value means "use the default"; clients that need different limits
// can build a Config from DefaultConfig and override individual fields.
type Config struct {
	// MaxCoverSize bounds how many bytes may be decompressed for a cover
	// image in any format, protecting against zip-bomb style e-books.
	MaxCoverSize int64
}

// DefaultConfig returns the library-wide default safety limits.
func DefaultConfig() Config {
	return Config{
		MaxCoverSize: 50 * 1024 * 1024, // 50 MB
	}
}
