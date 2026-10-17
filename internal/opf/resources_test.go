package opf

import (
	"bytes"
	"io"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

func TestBuildResourcesPropagatesEntryCharset(t *testing.T) {
	p := Package{
		Manifest: Manifest{Items: []Item{
			{ID: "ch", Href: "ch.html", MediaType: mediatype.HTML},
			{ID: "img", Href: "i.png", MediaType: mediatype.PNG},
			{ID: "undecided", Href: "n.html", MediaType: mediatype.HTML},
		}},
		Spine: Spine{ItemRefs: []ItemRef{{IdRef: "ch"}}},
	}
	open := func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(nil)), nil }
	lookup := func(it Item) (EntrySource, bool) {
		switch it.ID {
		case "ch":
			return EntrySource{Size: 1, Charset: "windows-1251", Open: open}, true
		case "img":
			return EntrySource{Size: 1, Charset: book.CharsetBinary, Open: open}, true
		case "undecided":
			return EntrySource{Size: 1, Open: open}, true
		}
		return EntrySource{}, false
	}

	all, order, _ := BuildResources(p, lookup)
	if len(all) != 3 {
		t.Fatalf("len(all) = %d, want 3", len(all))
	}
	byID := make(map[string]*book.Resource)
	for _, r := range all {
		byID[r.Id] = r
	}
	if got := byID["ch"].Charset; got != "windows-1251" {
		t.Errorf("ch Charset = %q, want windows-1251", got)
	}
	if got := byID["img"].Charset; got != book.CharsetBinary {
		t.Errorf("img Charset = %q, want binary", got)
	}
	// "" is intentional here: the NewResourceSet backstop decides.
	if got := byID["undecided"].Charset; got != "" {
		t.Errorf("undecided Charset = %q, want empty before backfill", got)
	}

	rs := book.NewResourceSet(all, order, nil)
	for _, r := range rs.All() {
		if r.Charset == "" {
			t.Errorf("resource %s left with empty Charset after NewResourceSet", r.Id)
		}
	}
	if got := byID["undecided"].Charset; got != book.CharsetUTF8 {
		t.Errorf("undecided Charset = %q, want utf-8 after backfill", got)
	}
}
