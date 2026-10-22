package ereader

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"golang.org/x/text/encoding/charmap"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
	"github.com/f0d0r/margaret-ebook-library/internal/pdb"
)

const (
	pdbIdentEreaderRelease = "PNRDPPRS"
	pdbIdentEreaderDebug   = "PNPDPPRS"
)

const (
	headerSizeDropbook = 132
	compressionDRM1    = 260
	compressionDRM2    = 272
)

type EreaderReader struct {
	cfg config.Config
}

// NewEreaderReader creates a new eReader reader with the given safety limits.
func NewEreaderReader(cfg config.Config) *EreaderReader {
	return &EreaderReader{cfg: cfg.Normalize()}
}

// Supports reports whether b is an eReader (Peanut Press) PDB file,
// identified by the type/creator at offset 60-67.
func (r *EreaderReader) Supports(b book.Blob) bool {
	buf := make([]byte, 68)
	n, err := b.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		return false
	}
	if n < 68 {
		return false
	}

	switch strings.ToUpper(string(buf[60:68])) {
	case pdbIdentEreaderRelease, pdbIdentEreaderDebug:
		return true
	default:
		return false
	}
}

// Read returns the book metadata and lazily-opened resources.
func (r *EreaderReader) Read(b book.Blob) (book.Book, error) {
	pdbDb, err := pdb.ReadPdbDb(b, r.cfg.MaxRecordSize)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to read PDB database: %w", book.ErrCorrupt, err)
	}
	hdr, err := parseHeader(pdbDb)
	if err != nil {
		return nil, err
	}
	if hdr.dropbook && (hdr.compression == compressionDRM1 || hdr.compression == compressionDRM2) {
		return nil, fmt.Errorf("ereader is DRM protected: %w", book.ErrDRM)
	}
	all, order, cover := r.resources(b, pdbDb, hdr)
	return &ereaderBook{
		metadata:  readMetadata(pdbDb, hdr),
		resources: book.NewResourceSet(all, order, cover),
		version:   hdr.version,
	}, nil
}

func readMetadata(pdbDb *pdb.PdbDb, hdr header) book.Metadata {
	meta := book.Metadata{Title: pdbDb.Name}
	if !hdr.dropbook || !hdr.hasMetadata || hdr.metadataIndex <= 0 || hdr.metadataIndex >= len(pdbDb.PdbRecords) {
		return meta
	}
	mraw, err := pdbDb.PdbRecords[hdr.metadataIndex].Data()
	if err != nil {
		return meta
	}
	fields := bytes.Split(mraw, []byte{0})
	if title := decodeField(fields, 0); title != "" {
		meta.Title = title
	}
	if author := decodeField(fields, 1); author != "" {
		meta.Authors = []string{author}
	}
	if isbn := decodeField(fields, 4); isbn != "" {
		meta.Identifiers = append(meta.Identifiers, book.Identifier{Scheme: "ISBN", Value: isbn})
	}
	return meta
}

func decodeField(fields [][]byte, i int) string {
	if i >= len(fields) {
		return ""
	}
	raw := bytes.TrimSpace(fields[i])
	dec, err := charmap.Windows1252.NewDecoder().Bytes(raw)
	if err != nil {
		return string(raw)
	}
	return string(dec)
}

func decodeName(raw []byte) string {
	raw = bytes.TrimRight(raw, "\x00")
	dec, err := charmap.Windows1252.NewDecoder().Bytes(raw)
	if err != nil {
		return string(raw)
	}
	return string(dec)
}

type ereaderBook struct {
	metadata  book.Metadata
	resources *book.ResourceSet
	version   string
}

func (b *ereaderBook) Metadata() book.Metadata {
	return b.metadata
}

func (b *ereaderBook) Resources() *book.ResourceSet {
	return b.resources
}

func (b *ereaderBook) FileType() book.FileType {
	return book.EREADER
}

func (b *ereaderBook) Version() string {
	return b.version
}
