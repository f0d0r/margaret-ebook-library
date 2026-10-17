package fb3

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	compressutil "github.com/f0d0r/margaret-ebook-library/internal/compress"
	"github.com/f0d0r/margaret-ebook-library/internal/xml"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

// OPC relationship types and the description content type.
const (
	relBook                = "http://www.fictionbook.org/FictionBook3/relationships/Book"
	relBody                = "http://www.fictionbook.org/FictionBook3/relationships/body"
	relImage               = "http://www.fictionbook.org/FictionBook3/relationships/image"
	relThumbnail           = "http://schemas.openxmlformats.org/package/2006/relationships/metadata/thumbnail"
	contentTypeDescription = "application/fb3-description+xml"
)

// relationships maps an OPC .rels part (local names only).
type relationships struct {
	Items []relationship `xml:"Relationship"`
}

type relationship struct {
	ID     string `xml:"Id,attr"`
	Target string `xml:"Target,attr"`
	Type   string `xml:"Type,attr"`
}

// contentTypes maps [Content_Types].xml entries.
type contentTypes struct {
	Defaults  []contentDefault  `xml:"Default"`
	Overrides []contentOverride `xml:"Override"`
}

type contentDefault struct {
	Extension   string `xml:"Extension,attr"`
	ContentType string `xml:"ContentType,attr"`
}

type contentOverride struct {
	PartName    string `xml:"PartName,attr"`
	ContentType string `xml:"ContentType,attr"`
}

// findDescriptionPart locates the description part: root .rels Book
// target, [Content_Types] override, then root-tag scan. Later layers
// run only when earlier ones resolve nothing.
func findDescriptionPart(zr *zip.Reader, maxSize int64) (*zip.File, error) {
	if f := findRootRels(zr); f != nil {
		if target := bookTarget(f, maxSize); target != "" {
			if resolved := resolvePart(zr, "", target); resolved != nil {
				return resolved, nil
			}
		}
	}
	if f := findContentTypes(zr); f != nil {
		if part := descriptionOverride(f, maxSize); part != "" {
			if resolved := resolvePart(zr, "", part); resolved != nil {
				return resolved, nil
			}
		}
	}
	if f := findPartWithRoot(zr, elFb3Description); f != nil {
		return f, nil
	}
	return nil, fmt.Errorf("fb3 description part not found")
}

// bookTarget returns the first Book relationship target, or "".
func bookTarget(f *zip.File, maxSize int64) string {
	return relTarget(f, relBook, maxSize)
}

// descriptionOverride returns the PartName declaring the description content type, or "".
func descriptionOverride(f *zip.File, maxSize int64) string {
	head, err := readPartFull(f, maxSize)
	if err != nil {
		return ""
	}
	var types contentTypes
	if err := parseXMLPart(head, &types); err != nil {
		return ""
	}
	for _, o := range types.Overrides {
		if strings.EqualFold(strings.TrimSpace(o.ContentType), contentTypeDescription) {
			return strings.TrimSpace(o.PartName)
		}
	}
	return ""
}

// parseXMLPart decodes a small OPC part, honoring a declared non-UTF-8
// encoding (and UTF-16 input) via the shared xml.NewDecoder.
func parseXMLPart(data []byte, v any) error {
	return xml.NewDecoder(bytes.NewReader(data)).Decode(v)
}

// resolvePart resolves a .rels Target or PartName to an entry, tolerating
// slashes, dot segments, URL-encoding and case differences. A leading
// slash resolves against the package root, else against base.
func resolvePart(zr *zip.Reader, base, target string) *zip.File {
	if strings.HasPrefix(target, "/") {
		base = ""
	}
	cleaned := cleanPartPath(path.Join(base, strings.TrimPrefix(target, "/")))
	if cleaned == "" {
		return nil
	}
	candidates := []string{cleaned}
	if unescaped, err := url.PathUnescape(cleaned); err == nil && unescaped != cleaned {
		candidates = append(candidates, unescaped)
	}
	for _, c := range candidates {
		if f := compressutil.Find(zr, c); f != nil {
			return f
		}
		for _, f := range zr.File {
			if f.FileInfo().IsDir() {
				continue
			}
			if strings.EqualFold(f.Name, c) {
				return f
			}
		}
	}
	return nil
}

// findPartWithRoot returns the first entry containing the given root open tag.
func findPartWithRoot(zr *zip.Reader, root string) *zip.File {
	want := []byte("<" + strings.ToLower(root))
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		head, err := compressutil.ReadHead(f, probeSize)
		if err != nil {
			continue
		}
		if bytes.Contains(bytes.ToLower(head), want) {
			return f
		}
	}
	return nil
}

// resolveBodyPart locates the body part via the description's sibling
// rels, then by root-tag scan.
func resolveBodyPart(zr *zip.Reader, descPath string, maxSize int64) (*zip.File, error) {
	if f := siblingRels(zr, descPath); f != nil {
		if target := relTarget(f, relBody, maxSize); target != "" {
			if resolved := resolvePart(zr, path.Dir(descPath), target); resolved != nil {
				return resolved, nil
			}
		}
	}
	if f := findPartWithRoot(zr, elFb3Body); f != nil {
		return f, nil
	}
	return nil, fmt.Errorf("fb3 body part not found")
}

// siblingRels locates <dir>/_rels/<base>.rels (case-insensitive).
func siblingRels(zr *zip.Reader, partPath string) *zip.File {
	dir, base := path.Split(partPath)
	relsPath := path.Join(dir, "_rels", base+".rels")
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if strings.EqualFold(f.Name, relsPath) {
			return f
		}
	}
	return nil
}

// relTarget returns the first relationship target of the given type, or "".
func relTarget(f *zip.File, relType string, maxSize int64) string {
	head, err := readPartFull(f, maxSize)
	if err != nil {
		return ""
	}
	var rels relationships
	if err := parseXMLPart(head, &rels); err != nil {
		return ""
	}
	for _, r := range rels.Items {
		if r.Type == relType {
			return strings.TrimSpace(r.Target)
		}
	}
	return ""
}

// imageRef is one body image relationship (rels Id and target).
type imageRef struct {
	id     string
	target string
}

// bodyImageRels returns the body image relationships in order.
// A missing part means no images, not an error.
func bodyImageRels(zr *zip.Reader, bodyPath string, maxSize int64) []imageRef {
	f := siblingRels(zr, bodyPath)
	if f == nil {
		return nil
	}
	head, err := readPartFull(f, maxSize)
	if err != nil {
		return nil
	}
	var rels relationships
	if err := parseXMLPart(head, &rels); err != nil {
		return nil
	}
	var out []imageRef
	for _, r := range rels.Items {
		if r.Type != relImage {
			continue
		}
		id := strings.TrimSpace(r.ID)
		target := strings.TrimSpace(r.Target)
		if id == "" || target == "" {
			continue
		}
		out = append(out, imageRef{id: id, target: target})
	}
	return out
}

// rootThumbnailPath resolves the root thumbnail relationship, or "".
// Only an explicit thumbnail yields a cover.
func rootThumbnailPath(zr *zip.Reader, maxSize int64) string {
	f := findRootRels(zr)
	if f == nil {
		return ""
	}
	target := relTarget(f, relThumbnail, maxSize)
	if target == "" {
		return ""
	}
	if resolved := resolvePart(zr, "", target); resolved != nil {
		return resolved.Name
	}
	return ""
}

// mediaMap maps [Content_Types] overrides and extension defaults.
type mediaMap struct {
	byExt  map[string]string // lowercased extension without dot -> MIME
	byPart map[string]string // cleaned part path -> MIME (matched case-insensitively)
}

// parseMediaMap reads [Content_Types].xml; a missing or unreadable part
// yields an empty map.
func parseMediaMap(zr *zip.Reader, maxSize int64) *mediaMap {
	m := &mediaMap{byExt: map[string]string{}, byPart: map[string]string{}}
	f := findContentTypes(zr)
	if f == nil {
		return m
	}
	head, err := readPartFull(f, maxSize)
	if err != nil {
		return m
	}
	var types contentTypes
	if err := parseXMLPart(head, &types); err != nil {
		return m
	}
	for _, d := range types.Defaults {
		ext := strings.ToLower(strings.TrimSpace(d.Extension))
		ct := strings.TrimSpace(d.ContentType)
		if ext == "" || ct == "" {
			continue
		}
		m.byExt[ext] = ct
	}
	for _, o := range types.Overrides {
		part := cleanPartPath(o.PartName)
		ct := strings.TrimSpace(o.ContentType)
		if part == "" || ct == "" {
			continue
		}
		m.byPart[part] = ct
	}
	return m
}

// mediaFor returns the media type for a part path: override, extension,
// built-in image map, else octet-stream. Nothing is sniffed.
func (m *mediaMap) mediaFor(partPath string) string {
	cleaned := cleanPartPath(partPath)
	for part, ct := range m.byPart {
		if strings.EqualFold(part, cleaned) {
			return ct
		}
	}
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(cleaned), "."))
	if ext != "" {
		if ct, ok := m.byExt[ext]; ok {
			return ct
		}
		if ct, ok := imageExtMedia[ext]; ok {
			return ct
		}
	}
	return mediatype.OctetStream
}

// imageExtMedia maps image extensions when [Content_Types].xml is missing.
var imageExtMedia = map[string]string{
	"jpg":  mediatype.JPEG,
	"jpeg": mediatype.JPEG,
	"png":  mediatype.PNG,
	"gif":  mediatype.GIF,
	"svg":  mediatype.SVG,
	"webp": mediatype.WEBP,
	"bmp":  mediatype.BMP,
}

// readPartFull reads a small OPC part (rels, content types) in full,
// bounded by maxSize. Unlike the probeSize head sniff used for detection,
// resolution needs complete documents: truncated XML would silently drop
// entries.
func readPartFull(f *zip.File, maxSize int64) ([]byte, error) {
	rc, err := compressutil.OpenLimited(f, maxSize)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	return io.ReadAll(rc)
}

// cleanPartPath normalizes a part reference to a slash path.
func cleanPartPath(p string) string {
	cleaned := path.Clean("/" + strings.TrimPrefix(p, "/"))
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "." {
		return ""
	}
	return cleaned
}
