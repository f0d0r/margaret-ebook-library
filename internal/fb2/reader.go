package fb2

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/f0d0r/margaret-ebook-library/book"
	compressutil "github.com/f0d0r/margaret-ebook-library/internal/compress"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
	xmlx "github.com/f0d0r/margaret-ebook-library/internal/xml"
)

type Fb2Reader struct {
	cfg config.Config
}

func NewFb2Reader(cfg config.Config) *Fb2Reader {
	return &Fb2Reader{
		cfg: cfg,
	}
}

// probeSize bounds detection reads from the blob and zip entries.
const probeSize = 32 * 1024

// maxZipEntries bounds sniffing a zip with no .fb2 entry.
const maxZipEntries = 5

func (r *Fb2Reader) Supports(b book.Blob) bool {
	size, err := b.Size()
	if err != nil || size == 0 {
		return false
	}

	headLen := min(int64(probeSize), size)
	head := make([]byte, headLen)
	n, err := b.ReadAt(head, 0)
	if err != nil && err != io.EOF {
		return false
	}
	head = head[:n]

	if compressutil.IsZip(head) {
		return r.supportsZipped(b)
	}
	return hasFictionBookRoot(head)
}

func (r *Fb2Reader) Read(b book.Blob) (book.Book, error) {
	maxSize := r.cfg.MaxResourceSize
	if maxSize <= 0 {
		maxSize = config.DefaultConfig().MaxResourceSize
	}
	size, err := b.Size()
	if err != nil {
		return nil, fmt.Errorf("failed to stat fb2: %w", err)
	}

	head := make([]byte, min(int64(4), size))
	if _, err := b.ReadAt(head, 0); err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to read fb2: %w", err)
	}

	// The source stream is capped at maxSize, so materializing it below is
	// bounded.
	var raw io.ReadCloser
	if compressutil.IsZip(head) {
		zr, err := compressutil.Open(b)
		if err != nil {
			return nil, fmt.Errorf("failed to open fb2 archive: %w", err)
		}
		// Same bounded fallback as Supports: first .fb2 entry, else first
		// sniff-matching entry.
		f := findFb2Entry(zr)
		if f == nil {
			return nil, fmt.Errorf("no readable entry in fb2 archive")
		}
		raw, err = compressutil.OpenLimited(f, maxSize)
		if err != nil {
			return nil, fmt.Errorf("failed to open fb2 entry: %w", err)
		}
	} else {
		if size > maxSize {
			return nil, fmt.Errorf("%w: fb2 file size %d exceeds limit %d", book.ErrLimitExceeded, size, maxSize)
		}
		raw = io.NopCloser(io.NewSectionReader(b, 0, size))
	}
	rawBytes, err := io.ReadAll(raw)
	_ = raw.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to decode fb2: %w", err)
	}

	// The storage encoding is recorded on every body resource; Open serves
	// the raw bytes byte-identically (BOM, declaration and original
	// encoding preserved). Transcoding happens only for structure
	// discovery/metadata (normalized copy) and in OpenAs converters.
	charset, err := detectFB2Charset(rawBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to decode fb2: %w", err)
	}
	normBuf, err := normalizeFB2(rawBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to decode fb2: %w", err)
	}

	// Locate body ranges on the normalized copy first (encoding/xml based):
	// on syntax errors retry with bare ampersands fixed.
	ranges, err := findBodyRanges(normBuf)
	if err != nil {
		var syntaxErr *xml.SyntaxError
		if !errors.As(err, &syntaxErr) {
			return nil, fmt.Errorf("failed to read fb2 content: %w", err)
		}
		fixed := bytes.ReplaceAll(normBuf, []byte("& "), []byte("&amp; "))
		ranges, err = findBodyRanges(fixed)
		if err != nil {
			return nil, fmt.Errorf("failed to read fb2 content: %w", err)
		}
	}

	// Then locate the same bodies in the raw document so Open can serve
	// the original bytes. The body count must agree; a mismatch fails
	// loudly instead of serving misaligned slices.
	rawRanges, err := locateRawBodyRanges(rawBytes, charset, len(ranges))
	if err != nil {
		return nil, fmt.Errorf("failed to read fb2 content: %w", err)
	}

	pm, err := parseNormalized(normBuf)
	if err != nil {
		return nil, fmt.Errorf("failed to read fb2 metadata: %w", err)
	}
	all, readingOrder, cover, err := buildFb2Resources(rawBytes, normBuf, rawRanges, charset, pm, maxSize)
	if err != nil {
		return nil, fmt.Errorf("failed to read fb2 content: %w", err)
	}
	return &fb2Book{
		metadata:  pm.metadata,
		resources: book.NewResourceSet(all, readingOrder, cover),
		version:   pm.version,
	}, nil
}

// supportsZipped sniffs a zipped FB2 (.fbz / .fb2.zip) blob in two phases:
//  1. the first .fb2 entry (case-insensitive) in archive order;
//     a broken first .fb2 rejects the blob;
//  2. else up to maxZipEntries other entries in archive order.
func (r *Fb2Reader) supportsZipped(b book.Blob) bool {
	zr, err := compressutil.Open(b)
	if err != nil {
		return false
	}

	if f := findFb2Entry(zr); f != nil {
		return sniffEntry(f)
	}
	return false
}

// findFb2Entry selects the archive entry to open, shared by Supports and
// Read so the two can never disagree.
func findFb2Entry(zr *zip.Reader) *zip.File {
	if f := firstFb2Entry(zr); f != nil {
		return f
	}
	checked := 0
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if checked >= maxZipEntries {
			break
		}
		checked++
		if sniffEntry(f) {
			return f
		}
	}
	return nil
}

// firstFb2Entry returns the first .fb2 entry (case-insensitive) in archive order, or nil.
func firstFb2Entry(zr *zip.Reader) *zip.File {
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(f.Name), ".fb2") {
			return f
		}
	}
	return nil
}

// sniffEntry reports whether a zip entry starts a FictionBook document.
func sniffEntry(f *zip.File) bool {
	buf, err := compressutil.ReadHead(f, probeSize)
	if err != nil {
		return false
	}
	return hasFictionBookRoot(buf)
}

// hasFictionBookRoot reports whether buf contains an opening FictionBook tag.
// The scan is tolerant: NUL bytes are ignored, leading garbage (BOMs,
// declarations, comments, DOCTYPEs, PIs) is skipped, and an optional
// namespace prefix with either namespace (2.0 / 2.1) or none is accepted.
func hasFictionBookRoot(buf []byte) bool {
	stripped := make([]byte, 0, len(buf))
	for _, c := range buf {
		if c != 0x00 {
			stripped = append(stripped, c)
		}
	}
	return scanFictionBookTag(stripped)
}

// scanFictionBookTag scans for an opening tag whose local name is exactly
// "FictionBook" (XML is case-sensitive). Closing tags, PIs and markup
// declarations are skipped, and the next character must delimit the tag.
func scanFictionBookTag(buf []byte) bool {
	return xmlx.ScanOpenTag(buf, elFictionBook, false, true)
}
