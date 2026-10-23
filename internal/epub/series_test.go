package epub

import (
	"path/filepath"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
)

func TestReadSeries(t *testing.T) {
	tests := []struct {
		name      string
		metadata  string
		wantName  string
		wantIndex string
	}{
		{
			name: "EPUB 3 canonical",
			metadata: `<meta property="belongs-to-collection" id="c1">Saga</meta>` +
				`<meta refines="#c1" property="collection-type">series</meta>` +
				`<meta refines="#c1" property="group-position">2.5</meta>`,
			wantName:  "Saga",
			wantIndex: "2.5",
		},
		{
			name: "canonical without group-position has empty index",
			metadata: `<meta property="belongs-to-collection" id="c1">Saga</meta>` +
				`<meta refines="#c1" property="collection-type">series</meta>` +
				`<meta name="calibre:series_index" content="9"/>`,
			wantName:  "Saga",
			wantIndex: "",
		},
		{
			name: "OPF 2 calibre fallback",
			metadata: `<meta name="calibre:series" content="Discworld"/>` +
				`<meta name="calibre:series_index" content="2.5"/>`,
			wantName:  "Discworld",
			wantIndex: "2.5",
		},
		{
			name:      "missing",
			metadata:  `<dc:title>Test Book</dc:title>`,
			wantName:  "",
			wantIndex: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opfContent := `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Series Book</dc:title>` + tt.metadata + `
  </metadata>
</package>`

			epubPath := filepath.Join(t.TempDir(), tt.name+".epub")
			createValidTestEPUBWithContent(t, epubPath, opfContent)

			ebook, err := NewEpubReader(config.DefaultConfig()).Read(book.NewPathBlob(epubPath))
			if err != nil {
				t.Fatalf("Read() error: %v", err)
			}
			meta := ebook.Metadata()
			if meta.Series != tt.wantName || meta.SeriesIndex != tt.wantIndex {
				t.Errorf("Series = (%q, %q), want (%q, %q)",
					meta.Series, meta.SeriesIndex, tt.wantName, tt.wantIndex)
			}
		})
	}
}
