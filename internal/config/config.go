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
	// on parse failures (corrupt, malformed, etc.).
	//
	// The fallback is on by default and is stored as an explicit field rather
	// than a plain bool so that the zero value keeps the documented default:
	// with a plain bool a zero Config would silently mean "strict". Only
	// [WithKF8FallbackToMOBI6] sets it, everything else leaves it nil and the
	// reader applies the default via [Config.KF8Fallback].
	KF8FallbackToMOBI6 *bool
}

// DefaultConfig returns the library-wide default safety limits.
func DefaultConfig() Config {
	return Config{
		MaxResourceSize:    100 * 1024 * 1024, // 100 MB
		MaxRecordSize:      100 * 1024 * 1024, // 100 MB
		MaxExthRecords:     256,
		KF8FallbackToMOBI6: boolPtr(true),
	}
}

// KF8Fallback reports whether a KF8 parse failure should fall back to the
// MOBI6 content. An unset (nil) field means the default, true.
func (c Config) KF8Fallback() bool {
	if c.KF8FallbackToMOBI6 == nil {
		return true
	}
	return *c.KF8FallbackToMOBI6
}

// Normalize returns a copy of the config with zero values replaced by the
// library defaults. The reader constructors call it so that a zero Config is
// usable and behaves like [DefaultConfig].
func (c Config) Normalize() Config {
	d := DefaultConfig()
	if c.MaxResourceSize <= 0 {
		c.MaxResourceSize = d.MaxResourceSize
	}
	if c.MaxRecordSize <= 0 {
		c.MaxRecordSize = d.MaxRecordSize
	}
	if c.MaxExthRecords <= 0 {
		c.MaxExthRecords = d.MaxExthRecords
	}
	return c
}

func boolPtr(b bool) *bool { return &b }
