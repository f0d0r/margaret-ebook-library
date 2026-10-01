package opf

import (
	"reflect"
	"strings"
	"testing"
)

// oeb10Sample mirrors the OEB 1.0.1 package found in MS Reader .LIT books:
// no version attribute, metadata in dc-metadata/x-metadata wrappers with
// capitalized element names.
const oeb10Sample = `<?xml version="1.0" encoding="UTF-8" ?>
<!DOCTYPE package
  PUBLIC "+//ISBN 0-9673008-1-9//DTD OEB 1.0.1 Package//EN"
  "http://openebook.org/dtds/oeb-1.0.1/oebpkg101.dtd">
<package unique-identifier="uuid_id">
 <metadata>
  <dc-metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:oebpackage="http://openebook.org/namespaces/oeb-package/1.0/">
   <dc:Language>hun</dc:Language>
   <dc:Title>Evelyn Hardcastle 7 hal&#225;la</dc:Title>
   <dc:Creator file-as="Turton, Stuart" role="aut">Stuart Turton</dc:Creator>
   <dc:Contributor role="bkp">calibre (9.15.0) [https://calibre-ebook.com]</dc:Contributor>
   <dc:Description>&lt;div&gt;A Blackheath-h&#225;z szab&#225;lyai.&lt;/div&gt;</dc:Description>
  </dc-metadata>
  <x-metadata>
   <meta name="cover" content="cover" />
  </x-metadata>
 </metadata>
 <manifest>
  <item id="cover" href="cover.jpeg" media-type="image/jpeg" />
 </manifest>
 <spine>
  <itemref idref="cover" />
 </spine>
</package>`

func TestParseOEB10Metadata(t *testing.T) {
	p, err := Parse(strings.NewReader(oeb10Sample))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if got, want := p.Title(), "Evelyn Hardcastle 7 halála"; got != want {
		t.Errorf("Title() = %q, want %q", got, want)
	}
	if got, want := p.Authors(), []string{"Stuart Turton"}; len(got) != 1 || got[0] != want[0] {
		t.Errorf("Authors() = %q, want %q", got, want)
	}
	if got := p.Description(); !strings.HasPrefix(got, "<div>A Blackheath-ház szabályai.") {
		t.Errorf("Description() = %q, want div prefix", got)
	}
	if got := p.Languages(); len(got) != 1 || got[0] != "hun" {
		t.Errorf("Languages() = %q, want [hun]", got)
	}
	if meta := p.MetaByName("cover"); meta == nil || meta.Content != "cover" {
		t.Errorf("MetaByName(cover) = %+v, want content cover", meta)
	}
	if item := p.ItemById("cover"); item == nil || item.Href != "cover.jpeg" {
		t.Errorf("ItemById(cover) = %+v, want href cover.jpeg", item)
	}
}

func TestModernPackageUnaffected(t *testing.T) {
	const modern = `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Modern Book</dc:title>
    <dc:creator>Jane Doe</dc:creator>
    <dc:language>en</dc:language>
  </metadata>
</package>`
	p, err := Parse(strings.NewReader(modern))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if got := p.Title(); got != "Modern Book" {
		t.Errorf("Title() = %q, want Modern Book", got)
	}
	if got := p.Authors(); len(got) != 1 || got[0] != "Jane Doe" {
		t.Errorf("Authors() = %q, want [Jane Doe]", got)
	}
	if got := p.Languages(); len(got) != 1 || got[0] != "en" {
		t.Errorf("Languages() = %q, want [en]", got)
	}
	if p.Version != "3.0" {
		t.Errorf("Version = %q, want 3.0", p.Version)
	}
}

func TestDirectBeatsWrapped(t *testing.T) {
	// When both shapes are present, the modern direct elements win.
	const both = `<package version="2.0"><metadata>
<title>Direct</title>
<dc-metadata><dc:Title>Wrapped</dc:Title></dc-metadata>
</metadata></package>`
	p, err := Parse(strings.NewReader(both))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if got := p.Title(); got != "Direct" {
		t.Errorf("Title() = %q, want Direct", got)
	}
}

func TestOEB10AuthorRoles(t *testing.T) {
	// Only aut (or empty) roles count as authors, like in modern OPF.
	const doc = `<package><metadata><dc-metadata>
<dc:Creator role="aut">Writer One</dc:Creator>
<dc:Creator role="ill">Illustrator</dc:Creator>
<dc:Creator>Writer Two</dc:Creator>
</dc-metadata></metadata></package>`
	p, err := Parse(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	want := []string{"Writer One", "Writer Two"}
	if got := p.Authors(); !reflect.DeepEqual(got, want) {
		t.Errorf("Authors() = %q, want %q", got, want)
	}
}
