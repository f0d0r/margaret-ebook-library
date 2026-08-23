package model

import "github.com/f0d0r/margaret-ebook-library/internal/config"

// Config holds library-wide safety limits shared by all format readers.
// Deprecated: use internal/config.Config for internal code; public callers
// should use ebook.Option. This alias is kept for compatibility.
type Config = config.Config

// DefaultConfig returns the library-wide default safety limits.
// Deprecated: use internal/config.DefaultConfig.
func DefaultConfig() Config {
	return config.DefaultConfig()
}
