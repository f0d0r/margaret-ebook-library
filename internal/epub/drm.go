package epub

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/f0d0r/margaret-ebook-library/book"
	compressutil "github.com/f0d0r/margaret-ebook-library/internal/compress"
)

// Adobe/IDPF font obfuscation algorithms. These are the only
// META-INF/encryption.xml entries that are NOT DRM — they only obfuscate
// embedded fonts and Calibre decrypts them on the fly.
// See calibre src/calibre/ebooks/conversion/plugins/epub_input.py:
// ADOBE_OBFUSCATION, IDPF_OBFUSCATION.
const (
	adobeObfuscation = "http://ns.adobe.com/pdf/enc#RC"
	idpfObfuscation  = "http://www.idpf.org/2008/embedding"
)

// Content-document media types examined for the Kobo DRM sniff.
// Mirrors calibre OEB_DOCS in src/calibre/ebooks/oeb/base.py.
var oebDocs = map[string]bool{
	"application/xhtml+xml": true,
	"text/html":             true,
	"text/x-oeb1-document":  true,
	"text/x-oeb-document":   true,
}

// koboDRMProbeSize mirrors calibre's 8192-byte window in
// check_for_kobo_drm (kepubify.py).
const koboDRMProbeSize = 8192

// checkDRM rejects DRM-protected EPUB/KEPUB archives with book.ErrDRM.
// It mirrors Calibre's two independent checks:
//
//  1. Adobe ADEPT: META-INF/encryption.xml with any EncryptionMethod
//     algorithm outside the font-obfuscation allowlist (or an
//     unparseable encryption.xml) means DRM.
//  2. Kobo kdrm: a non-empty rights.xml plus an encrypted first spine
//     content document (first 8 KiB contains neither <?xml, <html nor
//     koboSpan) means DRM. A bare or leftover rights.xml alone is NOT
//     definitive, exactly like in Calibre.
func checkDRM(zr *zip.Reader, p Package) error {
	if err := checkAdobeDRM(zr); err != nil {
		return err
	}
	return checkKoboDRM(zr, p)
}

func checkAdobeDRM(zr *zip.Reader) error {
	f := compressutil.Find(zr, "META-INF/encryption.xml")
	if f == nil {
		return nil
	}
	// encryption.xml is tiny; cap the parse input.
	raw, err := compressutil.ReadHead(f, 1<<20)
	if err != nil {
		return fmt.Errorf("epub is DRM protected (META-INF/encryption.xml unreadable): %w", book.ErrDRM)
	}
	// An empty file carries no encrypted content (e.g. left behind emptied
	// by a DeDRM tool), so it is not DRM — unlike Calibre, which rejects
	// it on parse failure.
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	algos, err := encryptionAlgorithms(raw)
	if err != nil {
		return fmt.Errorf("epub is DRM protected (META-INF/encryption.xml unparseable): %w", book.ErrDRM)
	}
	for _, algo := range algos {
		if algo != adobeObfuscation && algo != idpfObfuscation {
			return fmt.Errorf("epub is DRM protected (META-INF/encryption.xml algorithm %q): %w", algo, book.ErrDRM)
		}
	}
	return nil
}

// encryptionAlgorithms returns every EncryptionMethod Algorithm attribute
// in an encryption.xml document, regardless of XML namespace. Zero methods
// means font-obfuscation-only or empty metadata, i.e. no DRM. Malformed
// XML reports an error so the caller can reject it as DRM, like Calibre.
func encryptionAlgorithms(raw []byte) ([]string, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, fmt.Errorf("empty encryption.xml")
	}
	dec := xml.NewDecoder(bytes.NewReader(raw))
	var algos []string
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "EncryptionMethod" {
			continue
		}
		for _, attr := range start.Attr {
			if attr.Name.Local == "Algorithm" {
				algos = append(algos, strings.TrimSpace(attr.Value))
			}
		}
	}
	return algos, nil
}

func checkKoboDRM(zr *zip.Reader, p Package) error {
	rights := findRightsXML(zr)
	if rights == nil {
		return nil
	}
	// Non-empty check mirrors container.has_name_and_is_not_empty.
	if rights.UncompressedSize64 == 0 {
		return nil
	}
	first := firstSpineContentFile(zr, p)
	if first == nil {
		return nil
	}
	raw, err := compressutil.ReadHead(first, koboDRMProbeSize)
	if err != nil {
		return fmt.Errorf("kepub is DRM protected (rights.xml present, spine unreadable): %w", book.ErrDRM)
	}
	// Markup markers are matched case-insensitively: <HTML> is legal
	// HTML5, unlike Calibre's byte-exact check. The koboSpan class name
	// stays case-sensitive — it is defined verbatim by Kobo.
	lower := bytes.ToLower(raw)
	if !bytes.Contains(lower, []byte("<?xml")) &&
		!bytes.Contains(lower, []byte("<html")) &&
		!bytes.Contains(raw, []byte("koboSpan")) {
		return fmt.Errorf("kepub is DRM protected (rights.xml with encrypted content): %w", book.ErrDRM)
	}
	return nil
}

// findRightsXML locates the Kobo rights.xml entry. Calibre addresses it by
// the bare canonical name 'rights.xml'; ZIP entries are matched by base
// name to tolerate leading ./ prefixes while staying case-sensitive on
// the file name itself.
func findRightsXML(zr *zip.Reader) *zip.File {
	if f := compressutil.Find(zr, "rights.xml"); f != nil {
		return f
	}
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "/") {
			continue
		}
		if path.Base(f.Name) == "rights.xml" {
			return f
		}
	}
	return nil
}

// firstSpineContentFile returns the ZIP entry for the first spine item
// whose manifest media type is a content document (OEB_DOCS). Missing
// entries are skipped, like readResources does.
func firstSpineContentFile(zr *zip.Reader, p Package) *zip.File {
	r := OpfReader{}
	for _, itemRef := range p.Spine.ItemRefs {
		item := r.ItemById(p, itemRef.IdRef)
		if item == nil {
			continue
		}
		if !oebDocs[strings.ToLower(strings.TrimSpace(item.MediaType))] {
			continue
		}
		resolved := p.ResolvePath(item.Href)
		if f := compressutil.Find(zr, resolved); f != nil {
			return f
		}
	}
	return nil
}
