package lit

import (
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
)

func TestLitResourceCharsets(t *testing.T) {
	coverBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0, 'c', 'o', 'v', 'e', 'r'}
	b := book.NewBytesBlob(buildLitSections(t, append(baseEntries(),
		secEntry{name: "/data/c1/content", data: fixtureContent()},
		secEntry{name: "/data/cov", data: coverBytes},
	)))

	ebook, err := NewLitReader(config.DefaultConfig()).Read(b)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	rs := ebook.Resources()

	// Synthesized spine XHTML is the documented exception: it is produced
	// by the reader itself, hence always UTF-8.
	ro := rs.ReadingOrder()
	if len(ro) != 1 {
		t.Fatalf("len(ReadingOrder) = %d, want 1", len(ro))
	}
	if got := ro[0].Resource.Charset; got != book.CharsetUTF8 {
		t.Errorf("spine Charset = %q, want utf-8", got)
	}

	byID := make(map[string]*book.Resource)
	for _, r := range rs.All() {
		byID[r.Id] = r
	}
	if got := byID["cover"].Charset; got != book.CharsetBinary {
		t.Errorf("cover Charset = %q, want binary", got)
	}
	// The raw (non-spine) entries rely on the media-type backstop.
	for _, r := range rs.All() {
		if r.Charset == "" {
			t.Errorf("resource %s left with empty Charset", r.Id)
		}
	}
}
