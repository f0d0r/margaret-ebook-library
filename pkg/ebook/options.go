package ebook

import "github.com/f0d0r/margaret-ebook-library/pkg/model"

type options struct {
	maxResourceSize int64
	maxRecordSize   int64
	maxExthRecords  int
}

// Option configures a metadata read.
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

// resolveOptions returns the effective configuration for the given options,
// falling back to the defaults of [model.DefaultConfig] for fields that are
// not overridden.
func resolveOptions(opts []Option) model.Config {
	d := model.DefaultConfig()
	o := options{
		maxResourceSize: d.MaxResourceSize,
		maxRecordSize:   d.MaxRecordSize,
		maxExthRecords:  d.MaxExthRecords,
	}
	for _, opt := range opts {
		opt(&o)
	}
	return model.Config{
		MaxResourceSize: o.maxResourceSize,
		MaxRecordSize:   o.maxRecordSize,
		MaxExthRecords:  o.maxExthRecords,
	}
}
