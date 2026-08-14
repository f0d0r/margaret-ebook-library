package ebook

import "github.com/f0d0r/margaret-ebook-library/pkg/model"

type options struct {
	maxCoverSize   int64
	maxRecordSize  int64
	maxExthRecords int
}

// Option configures a metadata read.
type Option func(*options)

// WithMaxCoverSize sets the maximum decompressed cover size in bytes.
func WithMaxCoverSize(size int64) Option {
	return func(o *options) {
		o.maxCoverSize = size
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
		maxCoverSize:   d.MaxCoverSize,
		maxRecordSize:  d.MaxRecordSize,
		maxExthRecords: d.MaxExthRecords,
	}
	for _, opt := range opts {
		opt(&o)
	}
	return model.Config{
		MaxCoverSize:   o.maxCoverSize,
		MaxRecordSize:  o.maxRecordSize,
		MaxExthRecords: o.maxExthRecords,
	}
}
