package epub

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"strings"

	"faun.projects/margaret/margaret-ebook-library/internal/converter"
	"golang.org/x/text/language"
)

// OpfReader handles Open Publication Format (OPF) parsing and metadata extraction
type OpfReader struct{}

// NewOpfReader creates a new OPFReader instance
func NewOpfReader() OpfReader {
	return OpfReader{}
}

// Read reads and parses the OPF (package.xml) file from the EPUB
func (opf *OpfReader) Read(zr *zip.Reader, c Container) (Package, error) {
	if len(c.Rootfiles.RootfileList) == 0 {
		return Package{}, fmt.Errorf("no rootfile found in container.xml")
	}

	for _, rootfile := range c.Rootfiles.RootfileList {
		if rootfile.MediaType == "application/oebps-package+xml" {
			return opf.findPackageFile(zr, rootfile)
		}
	}

	return Package{}, fmt.Errorf("no opf file found")
}

// findPackageFile locates and parses the OPF file specified by the rootfile
func (opf *OpfReader) findPackageFile(zr *zip.Reader, rootfile Rootfile) (Package, error) {
	pkgFile := findFileInZip(zr, rootfile.FullPath)
	if pkgFile == nil {
		return Package{}, fmt.Errorf("opf file not found")
	}

	opfr, err := pkgFile.Open()
	if err != nil {
		return Package{}, fmt.Errorf("failed to open opf file: %w", err)
	}
	defer func() { _ = opfr.Close() }()

	var p Package
	if err := xml.NewDecoder(opfr).Decode(&p); err != nil {
		return Package{}, fmt.Errorf("failed to decode opf file: %w", err)
	}
	return p, nil
}

// GetTitle extracts the title from a Package
func (opf *OpfReader) GetTitle(p Package) string {
	if len(p.Metadata.Title) == 0 {
		return ""
	}
	return strings.TrimSpace(p.Metadata.Title[0].Value)
}

// GetAuthors extracts the list of authors from a Package
func (opf *OpfReader) GetAuthors(p Package) []string {
	authors := make([]string, 0, len(p.Metadata.Creators))
	for _, creator := range p.Metadata.Creators {
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

// GetDescription extracts the description from a Package
func (opf *OpfReader) GetDescription(p Package) string {
	if len(p.Metadata.Descriptions) == 0 {
		return ""
	}
	for _, d := range p.Metadata.Descriptions {
		if d = strings.TrimSpace(d); d != "" {
			return converter.HtmlToText(d)
		}
	}
	return ""
}

// GetLanguages extracts and validates the list of languages from a Package
func (opf *OpfReader) GetLanguages(p Package) []string {
	if len(p.Metadata.Languages) == 0 {
		return nil
	}
	languages := make([]string, 0, len(p.Metadata.Languages))
	for _, lang := range p.Metadata.Languages {
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
