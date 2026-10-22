// Package opf provides Open Publication Format (OPF) parsing and
// metadata extraction shared by the e-book format readers (EPUB, LIT, …).
//
// Besides modern OPF it tolerates OEB 1.x packages as found in MS Reader
// .LIT books: metadata nested in dc-metadata/x-metadata wrappers with
// capitalized element names.
package opf

import (
	"bytes"
	"encoding/xml"
	"io"
	"path"
	"strings"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/util"
	xmlx "github.com/f0d0r/margaret-ebook-library/internal/xml"
	"golang.org/x/text/language"
)

type Package struct {
	OpfPath  string   // the location of the OPF file
	XMLName  xml.Name `xml:"package"`
	Version  string   `xml:"version,attr"`
	Metadata Metadata `xml:"metadata"`
	Manifest Manifest `xml:"manifest"`
	Spine    Spine    `xml:"spine"`
}

func (p *Package) ResolvePath(href string) string {
	opfDir := path.Dir(p.OpfPath)
	return path.Join(opfDir, href)
}

type Metadata struct {
	Title        []Title      `xml:"title"`
	Creators     []Creator    `xml:"creator"`
	Descriptions []string     `xml:"description"`
	Languages    []string     `xml:"language"`
	Identifiers  []Identifier `xml:"identifier"`
	Sources      []Identifier `xml:"source"`
	Publishers   []string     `xml:"publisher"`
	Dates        []Date       `xml:"date"`
	Metas        []Meta       `xml:"meta"`

	// DirectLanguages holds capitalized language elements placed directly
	// under metadata, as found in unbalanced MS-era packages where they
	// escape the dc-metadata wrapper.
	DirectLanguages []string `xml:"Language"`

	// Dc holds OEB 1.x metadata nested in a dc-metadata wrapper (LIT).
	Dc *DcMetadata `xml:"dc-metadata"`
	// XMeta holds OEB 1.x metadata nested in an x-metadata wrapper (LIT).
	XMeta *XMetadata `xml:"x-metadata"`
}

// DcMetadata carries OEB 1.x Dublin Core elements with capitalized names.
type DcMetadata struct {
	Titles       []Title      `xml:"Title"`
	Creators     []Creator    `xml:"Creator"`
	Descriptions []string     `xml:"Description"`
	Languages    []string     `xml:"Language"`
	Identifiers  []Identifier `xml:"Identifier"`
	Sources      []Identifier `xml:"Source"`
	Publishers   []string     `xml:"Publisher"`
	Dates        []Date       `xml:"Date"`
}

// XMetadata carries OEB 1.x extended metadata elements.
type XMetadata struct {
	Metas []Meta `xml:"meta"`
}

// Identifier is a raw identifier-bearing Dublin Core element
// (dc:identifier or dc:source).
type Identifier struct {
	Value  string `xml:",chardata"`
	ID     string `xml:"id,attr"`
	Scheme string `xml:"scheme,attr"`
}

// Date is a raw date-bearing Dublin Core element (dc:date). Event is the
// optional OPF 2 event attribute (e.g. "publication", "creation").
type Date struct {
	Value string `xml:",chardata"`
	Event string `xml:"event,attr"`
}

type Title struct {
	Value string `xml:",chardata"`
	ID    string `xml:"id,attr"`
	Lang  string `xml:"lang,attr"`
}

type Creator struct {
	Value  string `xml:",chardata"`    // the name of the creator (Rev. Dr. Martin Luther King Jr.)
	Role   string `xml:"role,attr"`    // 3 character long MARC value ("aut", "ill", etc)
	FileAs string `xml:"file-as,attr"` //  normalized form of the name (King, Martin Luther Jr.)
}

type Meta struct {
	Id       string `xml:"id,attr"`
	Refines  string `xml:"refines,attr"`
	Property string `xml:"property,attr"`
	Name     string `xml:"name,attr"`
	Content  string `xml:"content,attr"`
	Scheme   string `xml:"scheme,attr"`
	Value    string `xml:",chardata"`
}

type Item struct {
	ID           string `xml:"id,attr"`
	MediaType    string `xml:"media-type,attr"`
	Href         string `xml:"href,attr"`
	Properties   string `xml:"properties,attr"`
	Fallback     string `xml:"fallback,attr"`
	MediaOverlay string `xml:"media-overlay,attr"`
}

type Manifest struct {
	ID    string `xml:"id,attr"`
	Items []Item `xml:"item"`
}

type ItemRef struct {
	ID         string `xml:"id,attr"`
	IdRef      string `xml:"idref,attr"`
	Linear     string `xml:"linear,attr"`
	Properties string `xml:"properties,attr"`
}

type Spine struct {
	ID                       string    `xml:"id,attr"`
	Toc                      string    `xml:"toc,attr"`
	PageProgressionDirection string    `xml:"page-progression-direction,attr"`
	ItemRefs                 []ItemRef `xml:"itemref"`
}

// Parse decodes an OPF package document from r. A declared non-UTF-8
// encoding is decoded via x/text instead of failing; UTF-16 input is
// transcoded before scanning.
func Parse(r io.Reader) (Package, error) {
	var p Package
	if err := xmlx.NewDecoder(r).Decode(&p); err != nil {
		return Package{}, err
	}
	return p, nil
}

// ParseMetadata decodes only the <metadata> element from an OEB 1.0 XHTML
// payload as carried by PalmDOC text records (<HTML><HEAD><metadata>...
// with dc-metadata/x-metadata wrappers). A full Package cannot decode such
// a payload because its XMLName requires a <package> root.
//
// Before decoding, the first <metadata>...</metadata> fragment is sliced
// out of the input at the byte level (case-insensitive, attributes
// tolerated). This makes the parser immune to whatever surrounds the
// fragment: leading record padding (0x0E), NULs, BOM, <HTML><HEAD> markup,
// and trailing body markup with unclosed <BR>/<P>. Callers wrap the result
// as Package{Metadata: m} to reuse Title/Authors/Description/Languages.
//
// It returns an empty Metadata with a nil error when no complete
// <metadata>...</metadata> fragment is present; callers fall back to the
// PDB name then.
func ParseMetadata(r io.Reader) (Metadata, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return Metadata{}, err
	}
	frag, ok := sliceMetadataFragment(raw)
	if !ok {
		return Metadata{}, nil
	}
	dec := xmlx.NewDecoder(bytes.NewReader(frag))
	dec.Strict = false
	dec.Entity = xml.HTMLEntity
	var m Metadata
	if err := dec.Decode(&m); err != nil {
		return Metadata{}, err
	}
	return m, nil
}

// sliceMetadataFragment returns the first <metadata>...</metadata> range
// (closing tag included), matched case-insensitively. The opening tag may
// carry attributes; the character after the name must be a tag delimiter so
// that e.g. "<metadatax>" does not match.
func sliceMetadataFragment(buf []byte) ([]byte, bool) {
	lower := bytes.ToLower(buf)
	const open, close = "<metadata", "</metadata>"
	const closeLen = len(close)
	for start := bytes.Index(lower, []byte(open)); start >= 0; {
		after := start + len(open)
		if after >= len(buf) {
			return nil, false
		}
		if c := buf[after]; c != ' ' && c != '\t' && c != '\n' && c != '\r' && c != '>' && c != '/' {
			next := bytes.Index(lower[after:], []byte(open))
			if next < 0 {
				return nil, false
			}
			start = after + next
			continue
		}
		rest := lower[after:]
		end := bytes.Index(rest, []byte(close))
		if end < 0 {
			return nil, false
		}
		return buf[start : after+end+closeLen], true
	}
	return nil, false
}

// Title extracts the title from a Package.
func (p Package) Title() string {
	if len(p.Metadata.Title) == 0 && p.Metadata.Dc != nil {
		return firstNonEmptyTitle(p.Metadata.Dc.Titles)
	}
	if len(p.Metadata.Title) == 0 {
		return ""
	}
	return strings.TrimSpace(p.Metadata.Title[0].Value)
}

func firstNonEmptyTitle(titles []Title) string {
	if len(titles) == 0 {
		return ""
	}
	return strings.TrimSpace(titles[0].Value)
}

// Authors extracts the list of authors from a Package.
func (p Package) Authors() []string {
	authors := authorsOf(p.Metadata.Creators)
	if len(authors) == 0 && p.Metadata.Dc != nil {
		authors = authorsOf(p.Metadata.Dc.Creators)
	}
	return authors
}

func authorsOf(creators []Creator) []string {
	authors := make([]string, 0, len(creators))
	for _, creator := range creators {
		role := strings.ToLower(strings.TrimSpace(creator.Role))
		// Include creators with "aut" role or empty role (which defaults to author)
		if role != "aut" && role != "" {
			continue
		}
		name := strings.TrimSpace(creator.Value)
		fileAs := strings.TrimSpace(creator.FileAs)
		if name == "" && fileAs != "" {
			name = fileAs
		}
		if name != "" {
			authors = append(authors, name)
		}
	}
	return authors
}

// Description extracts the description from a Package.
func (p Package) Description() string {
	if d := firstNonEmpty(p.Metadata.Descriptions); d != "" {
		return d
	}
	if p.Metadata.Dc != nil {
		return firstNonEmpty(p.Metadata.Dc.Descriptions)
	}
	return ""
}

func firstNonEmpty(values []string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

// Languages extracts and validates the list of languages from a Package.
func (p Package) Languages() []string {
	if langs := validLanguages(p.Metadata.Languages); len(langs) > 0 {
		return langs
	}
	if p.Metadata.Dc != nil {
		if langs := validLanguages(p.Metadata.Dc.Languages); len(langs) > 0 {
			return langs
		}
	}
	return validLanguages(p.Metadata.DirectLanguages)
}

func validLanguages(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	languages := make([]string, 0, len(values))
	for _, lang := range values {
		lang = strings.TrimSpace(lang)
		if lang == "" {
			continue
		}
		tag, err := language.Parse(lang)
		if err != nil {
			continue
		}

		if tag.String() == "und" {
			continue
		}
		_, confidence := tag.Base()
		if confidence == language.No {
			continue
		}
		languages = append(languages, lang)
	}
	return languages
}

// Publisher extracts the first non-empty dc:publisher value from a Package.
// OEB 1.x wrapped metadata is used when the direct element is absent.
func (p Package) Publisher() string {
	if publisher := firstNonEmpty(p.Metadata.Publishers); publisher != "" {
		return publisher
	}
	if p.Metadata.Dc != nil {
		return firstNonEmpty(p.Metadata.Dc.Publishers)
	}
	return ""
}

// PublicationDate extracts the raw dc:date value from a Package. Dates
// carrying a non-publication OPF 2 event attribute are not publication dates
// and are ignored. An explicit "publication" event is preferred; otherwise
// the first non-empty date without an event wins. The value is returned as
// declared, without parsing or validation.
func (p Package) PublicationDate() string {
	dates := make([]Date, 0, len(p.Metadata.Dates))
	dates = append(dates, p.Metadata.Dates...)
	if p.Metadata.Dc != nil {
		dates = append(dates, p.Metadata.Dc.Dates...)
	}
	return publicationDate(dates)
}

func publicationDate(dates []Date) string {
	var fallback string
	for _, d := range dates {
		if value := strings.TrimSpace(d.Value); value != "" {
			event := strings.TrimSpace(d.Event)
			if strings.EqualFold(event, "publication") {
				return value
			}
			if event == "" && fallback == "" {
				fallback = value
			}
		}
	}
	return fallback
}

// metas returns all meta elements, including OEB 1.x wrapped ones.
func metas(p Package) []Meta {
	if p.Metadata.XMeta == nil {
		return p.Metadata.Metas
	}
	out := make([]Meta, 0, len(p.Metadata.Metas)+len(p.Metadata.XMeta.Metas))
	out = append(out, p.Metadata.Metas...)
	return append(out, p.Metadata.XMeta.Metas...)
}

// Identifiers extracts dc:identifier and dc:source values from a Package.
// dc:identifier values come first, followed by dc:source values; each group
// preserves its declared order. The scheme is resolved from the OPF 2 scheme
// attribute, an EPUB 3 identifier-type meta refinement, or a value prefix. A
// matching scheme prefix is removed from the value; values are not validated
// or normalized.
func (p Package) Identifiers() []book.Identifier {
	count := len(p.Metadata.Identifiers) + len(p.Metadata.Sources)
	if p.Metadata.Dc != nil {
		count += len(p.Metadata.Dc.Identifiers) + len(p.Metadata.Dc.Sources)
	}
	ids := make([]Identifier, 0, count)
	ids = append(ids, p.Metadata.Identifiers...)
	if p.Metadata.Dc != nil {
		ids = append(ids, p.Metadata.Dc.Identifiers...)
	}
	ids = append(ids, p.Metadata.Sources...)
	if p.Metadata.Dc != nil {
		ids = append(ids, p.Metadata.Dc.Sources...)
	}

	allMetas := metas(p)
	out := make([]book.Identifier, 0, len(ids))
	for _, id := range ids {
		explicit := strings.TrimSpace(id.Scheme)
		if explicit == "" {
			explicit = identifierTypeScheme(allMetas, id.ID)
		}
		scheme, value := util.SplitIdentifier(id.Value, explicit)
		if value == "" {
			continue
		}
		out = append(out, book.Identifier{Scheme: scheme, Value: value})
	}
	return out
}

// identifierTypeScheme resolves an EPUB 3 identifier-type meta refinement.
// Only the ONIX codelist 5 codes that map to a known scheme are interpreted.
func identifierTypeScheme(all []Meta, id string) string {
	if id == "" {
		return ""
	}
	refines := "#" + id
	for _, meta := range all {
		if !strings.EqualFold(strings.TrimSpace(meta.Property), "identifier-type") ||
			strings.TrimSpace(meta.Refines) != refines {
			continue
		}
		scheme := strings.TrimSpace(meta.Scheme)
		if strings.EqualFold(scheme, "onix:codelist5") {
			switch strings.TrimSpace(meta.Value) {
			case "02", "15", "24":
				return "ISBN"
			case "06":
				return "DOI"
			}
			return ""
		}
		if scheme != "" {
			return scheme
		}
		return strings.TrimSpace(meta.Value)
	}
	return ""
}

// ItemsByProperty returns all manifest items that have the specified property.
// It uses substring matching, so a property match succeeds if the search property
// is contained within an item's Properties field. Returns an empty slice if no
// items match or if the manifest is empty.
func (p Package) ItemsByProperty(property string) []Item {
	items := make([]Item, 0, len(p.Manifest.Items))
	for _, item := range p.Manifest.Items {
		if strings.Contains(item.Properties, property) {
			items = append(items, item)
		}
	}
	return items
}

// ItemById returns the manifest item with the specified ID, or nil if not found.
// IDs are matched exactly and are case-sensitive.
func (p Package) ItemById(id string) *Item {
	for _, item := range p.Manifest.Items {
		if item.ID == id {
			return &item
		}
	}
	return nil
}

// MetaByName returns the metadata element with the specified name attribute,
// or nil if not found. Names are matched exactly and are case-sensitive.
func (p Package) MetaByName(name string) *Meta {
	for _, meta := range metas(p) {
		if meta.Name == name {
			return &meta
		}
	}
	return nil
}
