package mobi

import (
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
)

func TestPalmDocSeries(t *testing.T) {
	payload := []byte(`<HTML><HEAD><metadata>` +
		`<dc-metadata><dc:Title>Palm Series</dc:Title></dc-metadata>` +
		`<x-metadata>` +
		`<meta name="calibre:series" content="Palm Saga"/>` +
		`<meta name="calibre:series_index" content="2.5"/>` +
		`</x-metadata>` +
		`</metadata></HEAD><BODY><P>Hello PalmDOC</P></BODY></HTML>`)
	raw := buildPalmDocBookFile("PalmSeries", payload, palmDocBookOpt{compression: 2})

	ebook, err := NewMobiReader(config.DefaultConfig()).Read(book.NewBytesBlob(raw))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	meta := ebook.Metadata()
	if meta.Series != "Palm Saga" || meta.SeriesIndex != "2.5" {
		t.Errorf("Series = (%q, %q), want (%q, %q)",
			meta.Series, meta.SeriesIndex, "Palm Saga", "2.5")
	}
}
