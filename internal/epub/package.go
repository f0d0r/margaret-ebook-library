package epub

import (
	"archive/zip"
	"fmt"

	compressutil "github.com/f0d0r/margaret-ebook-library/internal/compress"
	"github.com/f0d0r/margaret-ebook-library/internal/opf"
)

// readPackage reads and parses the OPF package file selected by the OCF
// container from the EPUB.
func readPackage(zr *zip.Reader, c Container) (opf.Package, error) {
	if len(c.Rootfiles.RootfileList) == 0 {
		return opf.Package{}, fmt.Errorf("no rootfile found in container.xml")
	}

	for _, rootfile := range c.Rootfiles.RootfileList {
		if rootfile.MediaType == "application/oebps-package+xml" {
			return findPackageFile(zr, rootfile)
		}
	}

	return opf.Package{}, fmt.Errorf("no opf file found")
}

// findPackageFile locates and parses the OPF file specified by the rootfile.
func findPackageFile(zr *zip.Reader, rootfile Rootfile) (opf.Package, error) {
	pkgFile := compressutil.Find(zr, rootfile.FullPath)
	if pkgFile == nil {
		return opf.Package{}, fmt.Errorf("opf file not found")
	}

	opfr, err := pkgFile.Open()
	if err != nil {
		return opf.Package{}, fmt.Errorf("failed to open opf file: %w", err)
	}
	defer func() { _ = opfr.Close() }()

	p, err := opf.Parse(opfr)
	if err != nil {
		return opf.Package{}, fmt.Errorf("failed to decode opf file: %w", err)
	}
	p.OpfPath = rootfile.FullPath
	return p, nil
}
