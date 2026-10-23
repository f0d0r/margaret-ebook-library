package epub

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
)

func TestReadSubjects(t *testing.T) {
	tests := []struct {
		name     string
		metadata string
		want     []string
	}{
		{
			name: "multiple in source order",
			metadata: `<dc:subject>Fantasy</dc:subject>` +
				`<dc:subject>Adventure</dc:subject>` +
				`<dc:subject>History</dc:subject>`,
			want: []string{"Fantasy", "Adventure", "History"},
		},
		{
			name: "trimmed and empty dropped",
			metadata: `<dc:subject>  Science  </dc:subject>` +
				`<dc:subject> </dc:subject><dc:subject>History</dc:subject>`,
			want: []string{"Science", "History"},
		},
		{
			name:     "semicolons split",
			metadata: `<dc:subject>Fantasy; Adventure</dc:subject>`,
			want:     []string{"Fantasy", "Adventure"},
		},
		{
			name:     "missing",
			metadata: `<dc:title>Test Book</dc:title>`,
			want:     nil,
		},
		{
			name: "OEB wrapped",
			metadata: `<dc-metadata><dc:Subject>Wrapped</dc:Subject>` +
				`</dc-metadata>`,
			want: []string{"Wrapped"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opfContent := `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Subject Book</dc:title>` + tt.metadata + `
  </metadata>
</package>`

			epubPath := filepath.Join(t.TempDir(), tt.name+".epub")
			createValidTestEPUBWithContent(t, epubPath, opfContent)

			ebook, err := NewEpubReader(config.DefaultConfig()).Read(book.NewPathBlob(epubPath))
			if err != nil {
				t.Fatalf("Read() error: %v", err)
			}
			if got := ebook.Metadata().Subjects; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Subjects = %#v, want %#v", got, tt.want)
			}
		})
	}
}
