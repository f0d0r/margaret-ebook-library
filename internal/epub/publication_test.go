package epub

import (
	"path/filepath"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
)

func TestReadPublisherAndPublicationDate(t *testing.T) {
	tmpDir := t.TempDir()
	reader := NewEpubReader(config.DefaultConfig())

	tests := []struct {
		name            string
		metadata        string
		wantPublisher   string
		wantPublication string
	}{
		{
			name: "direct metadata",
			metadata: `<dc:publisher>Acme Press</dc:publisher>` +
				`<dc:date>2000-01-01T00:00:00Z</dc:date>`,
			wantPublisher:   "Acme Press",
			wantPublication: "2000-01-01T00:00:00Z",
		},
		{
			name:            "publication event preferred",
			metadata:        `<dc:date opf:event="modification">2020-01-01</dc:date><dc:date opf:event="publication">1999-05-04</dc:date>`,
			wantPublication: "1999-05-04",
		},
		{
			name:            "raw date preserved",
			metadata:        `<dc:date>circa 1999</dc:date>`,
			wantPublication: "circa 1999",
		},
		{
			name:            "first non-empty values win",
			metadata:        `<dc:publisher> </dc:publisher><dc:publisher>Real Press</dc:publisher><dc:date></dc:date><dc:date>2001</dc:date>`,
			wantPublisher:   "Real Press",
			wantPublication: "2001",
		},
		{
			name:     "missing",
			metadata: `<dc:title>Test Book</dc:title>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opfContent := `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" xmlns:opf="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book</dc:title>` + tt.metadata + `
  </metadata>
</package>`

			epubPath := filepath.Join(tmpDir, tt.name+".epub")
			createValidTestEPUBWithContent(t, epubPath, opfContent)

			ebook, err := reader.Read(book.NewPathBlob(epubPath))
			if err != nil {
				t.Fatalf("Read() error: %v", err)
			}
			metadata := ebook.Metadata()
			if metadata.Publisher != tt.wantPublisher {
				t.Errorf("Publisher = %q, want %q", metadata.Publisher, tt.wantPublisher)
			}
			if metadata.PublicationDate != tt.wantPublication {
				t.Errorf("PublicationDate = %q, want %q", metadata.PublicationDate, tt.wantPublication)
			}
		})
	}
}
