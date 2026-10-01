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
)

type Fb2Reader struct {
	cfg config.Config
}

func NewFb2Reader(cfg config.Config) *Fb2Reader {
	return &Fb2Reader{
		cfg: cfg,
	}
}

// probeSize bounds how many bytes Supports inspects from the blob (or from
// each inspected zip entry). The FictionBook root tag normally appears within
// the first kilobyte, so 32KB leaves ample room for XML declarations,
// comments, DOCTYPEs and processing instructions while keeping detection cheap.
const probeSize = 32 * 1024

// maxZipEntries bounds how many unnamed entries Supports decompresses while
// sniffing a zip with no .fb2 entry. Detection reads at most probeSize bytes
// per entry.
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
	// bounded (transcoding may expand it slightly).
	var raw io.ReadCloser
	if compressutil.IsZip(head) {
		zr, err := compressutil.Open(b)
		if err != nil {
			return nil, fmt.Errorf("failed to open fb2 archive: %w", err)
		}
		// Calibre rule (get_fb2_data) with the same bounded fallback as
		// Supports: first .fb2 entry, else first sniff-matching entry.
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
	buf, err := canonicalizeFB2(raw)
	_ = raw.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to decode fb2: %w", err)
	}

	// Locate body ranges first: on syntax errors retry with bare ampersands
	// fixed, and keep using the fixed buffer downstream so offsets,
	// metadata and binaries stay consistent.
	ranges, err := findBodyRanges(buf)
	if err != nil {
		var syntaxErr *xml.SyntaxError
		if !errors.As(err, &syntaxErr) {
			return nil, fmt.Errorf("failed to read fb2 content: %w", err)
		}
		fixed := bytes.ReplaceAll(buf, []byte("& "), []byte("&amp; "))
		ranges, err = findBodyRanges(fixed)
		if err != nil {
			return nil, fmt.Errorf("failed to read fb2 content: %w", err)
		}
		buf = fixed
	}

	pm, err := parseMetadata(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read fb2 metadata: %w", err)
	}
	all, readingOrder, cover, err := buildFb2Resources(buf, pm, ranges, maxSize)
	if err != nil {
		return nil, fmt.Errorf("failed to read fb2 content: %w", err)
	}
	return &fb2Book{
		metadata:  pm.metadata,
		resources: book.NewResourceSet(all, readingOrder, cover),
		version:   pm.version,
	}, nil
}

// supportsZipped sniffs a zipped FB2 (.fbz / .fb2.zip) blob. It mirrors
// calibre's get_fb2_data in two phases:
//  1. Scan all entries (names only, no decompression) for the first .fb2
//     entry (case-insensitive) in archive order and sniff just that one.
//     Like calibre, a broken first .fb2 means rejection even if a later
//     .fb2 would parse, keeping Supports consistent with what Read opens.
//  2. If the archive contains no .fb2 entry at all, sniff up to
//     maxZipEntries other entries in archive order, hoping one holds FB2
//     content (more tolerant than calibre, which tries only the first).
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
// Read so the two can never disagree: the first .fb2 entry in archive order
// (calibre's get_fb2_data rule), else the first entry (of up to
// maxZipEntries scanned) whose decompressed head looks like FictionBook.
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

// firstFb2Entry returns the first .fb2 entry (case-insensitive) in archive
// order, or nil. Calibre's get_fb2_data opens exactly this entry.
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

// sniffEntry decompresses at most probeSize bytes of a zip entry and reports
// whether they look like the start of a FictionBook document.
func sniffEntry(f *zip.File) bool {
	buf, err := compressutil.ReadHead(f, probeSize)
	if err != nil {
		return false
	}
	return hasFictionBookRoot(buf)
}

// hasFictionBookRoot reports whether buf contains an opening FictionBook tag.
// The scan is deliberately tolerant, following calibre's lead:
//   - NUL bytes are ignored (calibre strips them before parsing, which also
//     makes UTF-16 encoded files detectable since their ASCII tags survive);
//   - leading garbage (BOMs, whitespace, XML declarations, comments,
//     DOCTYPEs, processing instructions) is skipped by scanning for '<';
//   - an optional namespace prefix (e.g. <fb:FictionBook) and either
//     namespace (2.0 / 2.1) or no namespace at all are accepted.
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
// "FictionBook" (XML is case-sensitive, matching calibre's lxml parsing).
// Closing tags, processing instructions and markup declarations are skipped,
// and the character following the tag name must be a tag delimiter so that
// e.g. "<FictionBookmark>" or prose mentioning FictionBook does not match.
func scanFictionBookTag(buf []byte) bool {
	for i := range len(buf) {
		if buf[i] != '<' {
			continue
		}
		j := i + 1
		if j < len(buf) && (buf[j] == '/' || buf[j] == '?' || buf[j] == '!') {
			continue
		}
		k := j
		for k < len(buf) && isNameChar(buf[k]) {
			k++
		}
		name := string(buf[j:k])
		if idx := strings.LastIndexByte(name, ':'); idx >= 0 {
			name = name[idx+1:]
		}
		if name != elFictionBook {
			continue
		}
		if k >= len(buf) {
			// Probe truncated mid-tag; the literal was still found.
			return true
		}
		if buf[k] == '>' || buf[k] == '/' || isSpace(buf[k]) {
			return true
		}
	}
	return false
}

// isNameChar reports whether c may appear in an XML tag/namespace-prefix
// name (ASCII subset; sufficient for matching the FictionBook literal).
func isNameChar(c byte) bool {
	switch {
	case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		return true
	case c == '_' || c == '-' || c == '.' || c == ':':
		return true
	default:
		return false
	}
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}
