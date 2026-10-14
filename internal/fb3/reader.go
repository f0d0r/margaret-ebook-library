package fb3

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/f0d0r/margaret-ebook-library/book"
	compressutil "github.com/f0d0r/margaret-ebook-library/internal/compress"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
)

// Fb3Reader detects FictionBook 3 (.fb3) books: OPC ZIP containers
// holding a description part and a body part.
type Fb3Reader struct {
	cfg config.Config
}

// NewFb3Reader creates an FB3 reader with the given safety limits.
func NewFb3Reader(cfg config.Config) *Fb3Reader {
	return &Fb3Reader{
		cfg: cfg.Normalize(),
	}
}

// probeSize bounds per-entry decompression during detection.
const probeSize = 32 * 1024

// maxSniffEntries bounds the fallback entry sniff.
const maxSniffEntries = 5

func (r *Fb3Reader) Supports(b book.Blob) bool {
	size, err := b.Size()
	if err != nil || size == 0 {
		return false
	}

	head := make([]byte, min(int64(4), size))
	n, err := b.ReadAt(head, 0)
	if err != nil && err != io.EOF {
		return false
	}
	if !compressutil.IsZip(head[:n]) {
		return false
	}

	zr, err := compressutil.Open(b)
	if err != nil {
		return false
	}

	// 1. [Content_Types].xml declaring FB3 parts.
	if f := findContentTypes(zr); f != nil {
		head, err := compressutil.ReadHead(f, probeSize)
		if err == nil && isFb3ContentTypes(head) {
			return true
		}
	}

	// 2. Root relationships pointing at the description part.
	if f := findRootRels(zr); f != nil {
		head, err := compressutil.ReadHead(f, probeSize)
		if err == nil && bytes.Contains(head, []byte("FictionBook3/relationships/Book")) {
			return true
		}
	}

	// 3. Fallback for damaged containers: sniff entry heads.
	return sniffFb3Roots(zr)
}

func (r *Fb3Reader) Read(b book.Blob) (book.Book, error) {
	maxSize := r.cfg.MaxResourceSize
	if maxSize <= 0 {
		maxSize = config.DefaultConfig().MaxResourceSize
	}

	zr, err := compressutil.Open(b)
	if err != nil {
		return nil, fmt.Errorf("failed to open fb3: %w", err)
	}

	descFile, err := findDescriptionPart(zr)
	if err != nil {
		return nil, err
	}
	rc, err := compressutil.OpenLimited(descFile, maxSize)
	if err != nil {
		return nil, fmt.Errorf("failed to open fb3 description: %w", err)
	}
	raw, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read fb3 description: %w", err)
	}

	pm, err := parseDescription(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to read fb3 metadata: %w", err)
	}

	bodyFile, err := resolveBodyPart(zr, descFile.Name)
	if err != nil {
		return nil, err
	}

	images := bodyImageRels(zr, bodyFile.Name)
	all, readingOrder, cover := buildFb3Resources(zr, bodyFile, images, parseMediaMap(zr), rootThumbnailPath(zr), maxSize)

	return &fb3Book{
		metadata:  pm.metadata,
		resources: book.NewResourceSet(all, readingOrder, cover),
	}, nil
}

// fb3Version is the FictionBook generation reported by Version. FB3 files
// declare no format version (the description version attribute counts
// document revisions), so the reader reports the generation implied by
// the container instead: a deliberate exception to declared-as-is.
const fb3Version = "3.0"

type fb3Book struct {
	metadata  book.Metadata
	resources *book.ResourceSet
}

func (b *fb3Book) Metadata() book.Metadata {
	return b.metadata
}

func (b *fb3Book) Resources() *book.ResourceSet {
	if b.resources == nil {
		return book.NewResourceSet(nil, nil, nil)
	}
	return b.resources
}

func (b *fb3Book) FileType() book.FileType {
	return book.FB
}

func (b *fb3Book) Version() string {
	return fb3Version
}

// findContentTypes locates [Content_Types].xml (case-insensitive).
func findContentTypes(zr *zip.Reader) *zip.File {
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if strings.EqualFold(path.Base(f.Name), "[content_types].xml") {
			return f
		}
	}
	return nil
}

// findRootRels locates _rels/.rels (case-insensitive).
func findRootRels(zr *zip.Reader) *zip.File {
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		dir, base := path.Split(strings.TrimSuffix(f.Name, "/"))
		if strings.EqualFold(strings.TrimSuffix(dir, "/"), "_rels") && strings.EqualFold(base, ".rels") {
			return f
		}
	}
	return nil
}

// isFb3ContentTypes reports FB3 part declarations in [Content_Types].xml.
func isFb3ContentTypes(head []byte) bool {
	return bytes.Contains(head, []byte("fb3-description")) ||
		bytes.Contains(head, []byte("fb3-body"))
}

// sniffFb3Roots reports FB3 root tags in entry heads,
// matching case-insensitively.
func sniffFb3Roots(zr *zip.Reader) bool {
	checked := 0
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || isNoiseEntry(f.Name) {
			continue
		}
		if checked >= maxSniffEntries {
			break
		}
		checked++
		head, err := compressutil.ReadHead(f, probeSize)
		if err != nil {
			continue
		}
		if isFb3RootHead(head) {
			return true
		}
	}
	return false
}

// isFb3RootHead reports FB3 description/body markers in decompressed bytes.
func isFb3RootHead(head []byte) bool {
	lowered := bytes.ToLower(head)
	return bytes.Contains(lowered, []byte("<fb3-description")) ||
		bytes.Contains(lowered, []byte("<fb3-body")) ||
		bytes.Contains(head, []byte("FictionBook3/"))
}

// isNoiseEntry reports packaging noise (macOS forks, .DS_Store).
func isNoiseEntry(name string) bool {
	if strings.HasPrefix(name, "__MACOSX/") {
		return true
	}
	return strings.EqualFold(path.Base(name), ".DS_Store")
}
