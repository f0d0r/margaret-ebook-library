package ebook

import (
	"github.com/f0d0r/margaret-ebook-library/internal/config"
)

type options struct {
	maxResourceSize    int64
	maxRecordSize      int64
	maxExthRecords     int
	kf8FallbackToMOBI6 *bool // nil means not set, use default
}

// Option configures an ebook read.
type Option func(*options)

// WithMaxResourceSize sets the maximum decompressed size in bytes of a single
// resource (a cover image, a content document, etc.).
func WithMaxResourceSize(size int64) Option {
	return func(o *options) {
		o.maxResourceSize = size
	}
}

// WithMaxRecordSize sets the maximum size of a single MOBI (PDB) record in bytes.
func WithMaxRecordSize(size int64) Option {
	return func(o *options) {
		o.maxRecordSize = size
	}
}

// WithMaxExthRecords sets the maximum number of EXTH records parsed from a MOBI file.
func WithMaxExthRecords(n int) Option {
	return func(o *options) {
		o.maxExthRecords = n
	}
}

// WithKF8FallbackToMOBI6 controls whether KF8 parsing falls back to MOBI6
// on parse failures. Default (nil) uses the library default (true).
// Set to false for Calibre-like strict behavior (error on KF8 parse failure).
// Set to true for fallback to MOBI6 content on KF8 parse failure.
func WithKF8FallbackToMOBI6(enabled bool) Option {
	return func(o *options) {
		o.kf8FallbackToMOBI6 = &enabled
	}
}

// resolveOptions returns the effective configuration for the given options,
// falling back to the defaults of [config.DefaultConfig] for fields that are
// not overridden.
func resolveOptions(opts []Option) config.Config {
	d := config.DefaultConfig()
	o := options{
		maxResourceSize: d.MaxResourceSize,
		maxRecordSize:   d.MaxRecordSize,
		maxExthRecords:  d.MaxExthRecords,
	}
	for _, opt := range opts {
		opt(&o)
	}
	fallback := d.KF8FallbackToMOBI6
	if o.kf8FallbackToMOBI6 != nil {
		fallback = *o.kf8FallbackToMOBI6
	}
	return config.Config{
		MaxResourceSize:    o.maxResourceSize,
		MaxRecordSize:      o.maxRecordSize,
		MaxExthRecords:     o.maxExthRecords,
		KF8FallbackToMOBI6: fallback,
	}
}
