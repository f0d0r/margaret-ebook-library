package lit

import (
	"bytes"
	"fmt"
	"io"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
	"github.com/f0d0r/margaret-ebook-library/internal/opf"
)

// litMagic is the 8-byte LIT signature at offset 0 ("ITOLITLS").
// See litheaders.c: lit_magic_string in the convertlit reference.
const litMagic = "ITOLITLS"

type LitReader struct {
	cfg config.Config
}

// NewLitReader creates a new LIT reader instance with the given
// safety limits. A zero Config is normalized to [config.DefaultConfig] so
// that a hand-built struct cannot silently disable the limits.
func NewLitReader(cfg config.Config) *LitReader {
	return &LitReader{cfg: cfg.Normalize()}
}

// Supports reports whether b looks like a LIT file. It checks only the
// 8-byte "ITOLITLS" magic at offset 0 (plus the primary-header minimum
// size), so detection is a single Size call and a single 8-byte ReadAt with
// no decompression. Stricter validation (version, header lengths) is left to
// Read.
func (r *LitReader) Supports(b book.Blob) bool {
	size, err := b.Size()
	if err != nil || size < priSize {
		return false
	}

	var buf [8]byte
	n, err := b.ReadAt(buf[:], 0)
	if err != nil && err != io.EOF {
		return false
	}
	if n < len(buf) {
		return false
	}

	return string(buf[:]) == litMagic
}

// splicePackageTail repairs MS-era packages whose </package> closes early
// with manifest/spine siblings trailing behind (one unbalanced close
// upstream shifts every close one level up). It moves the first
// </package> past the trailing elements. Balanced documents have nothing
// after </package> and pass through untouched; text content can never forge
// the marker since literal angle brackets are doubled by the decoder.
func splicePackageTail(decoded []byte) []byte {
	const close = "</package>"
	idx := bytes.Index(decoded, []byte(close))
	if idx < 0 {
		return decoded
	}
	tail := bytes.TrimSpace(decoded[idx+len(close):])
	if len(tail) == 0 {
		return decoded
	}
	if !bytes.Contains(tail, []byte("<manifest")) && !bytes.Contains(tail, []byte("<spine")) {
		return decoded
	}
	out := make([]byte, 0, len(decoded))
	out = append(out, decoded[:idx]...)
	out = append(out, tail...)
	out = append(out, close...)
	return out
}

// Read parses the LIT container into metadata and lazily-opened resources.
func (r *LitReader) Read(b book.Blob) (book.Book, error) {
	c, err := openContainer(b, r.cfg.MaxResourceSize)
	if err != nil {
		return nil, err
	}

	raw, err := c.getFile(metaEntry)
	if err != nil {
		return nil, err
	}
	opfText, err := decodeUnbinary(raw, &opfTables, c.opfPaths, nil, "")
	if err != nil {
		return nil, fmt.Errorf("lit: failed to decode OPF: %w", err)
	}
	opfText = splicePackageTail(opfText)

	p, err := opf.Parse(bytes.NewReader(opfText))
	if err != nil {
		return nil, fmt.Errorf("lit: failed to parse OPF: %w", err)
	}
	p.OpfPath = "content.opf"

	all, order, cover := c.buildResources(p, r.cfg.MaxResourceSize)

	return &litBook{
		metadata: book.Metadata{
			Title:       p.Title(),
			Authors:     p.Authors(),
			Description: p.Description(),
			Languages:   p.Languages(),
		},
		resources: book.NewResourceSet(all, order, cover),
		version:   litVersionString,
	}, nil
}
