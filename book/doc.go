// Package book defines the stable Book, metadata and resource abstractions shared by all formats.
//
// It provides the Book interface with normalized metadata, the ResourceSet
// with manifest resources in manifest order and spine-derived reading order,
// and random-access Blob sources over local files, archive entries and
// remote data.
//
// Book does not contain concrete transformation logic; its Resource and
// ResourceSet OpenAs methods delegate to the converter package's
// DefaultRegistry for ergonomic convenience.
package book
