package epub

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
)

// OcfReader handles Open Container Format (OCF) parsing and operations
type OcfReader struct{}

// NewOcfReader creates a new OCFReader instance
func NewOcfReader() OcfReader {
	return OcfReader{}
}

// Read reads and parses the container.xml file from the EPUB
func (ocf *OcfReader) Read(zr *zip.Reader) (Container, error) {
	containerFile, err := ocf.findContainerFile(zr)
	if err != nil {
		return Container{}, fmt.Errorf("failed to get container file: %w", err)
	}

	cfr, err := containerFile.Open()
	if err != nil {
		return Container{}, fmt.Errorf("failed to open container.xml: %w", err)
	}
	defer func() { _ = cfr.Close() }()

	var c Container
	if err := xml.NewDecoder(cfr).Decode(&c); err != nil {
		return Container{}, fmt.Errorf("failed to decode container.xml: %w", err)
	}
	return c, nil
}

// findContainerFile locates the container.xml file within the ZIP archive
func (ocf *OcfReader) findContainerFile(zr *zip.Reader) (*zip.File, error) {
	containerPath := "META-INF/container.xml"
	f := findFileInZip(zr, containerPath)
	if f == nil {
		return nil, fmt.Errorf("container.xml not found")
	}
	return f, nil
}
