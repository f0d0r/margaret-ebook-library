package opf

import (
	"reflect"
	"strings"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
)

func TestIdentifiers(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want []book.Identifier
	}{
		{
			name: "OPF2 scheme attribute",
			doc: `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" xmlns:opf="http://www.idpf.org/2007/opf" version="2.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="isbn" opf:scheme="ISBN">0-306-40615-2</dc:identifier>
  </metadata>
</package>`,
			want: []book.Identifier{{Scheme: "ISBN", Value: "0-306-40615-2"}},
		},
		{
			name: "OPF3 identifier-type ONIX codes",
			doc: `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="pub-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="pub-id">urn:uuid:A1B0D67E-2E81-4DF5-9E67-A64CBE366809</dc:identifier>
    <dc:identifier id="isbn10">0306406152</dc:identifier>
    <meta refines="#isbn10" property="identifier-type" scheme="onix:codelist5">02</meta>
    <dc:identifier id="isbn13">9780306406157</dc:identifier>
    <meta refines="#isbn13" property="identifier-type" scheme="onix:codelist5">15</meta>
    <dc:identifier id="doi">10.1016/j.iheduc.2008.03.001</dc:identifier>
    <meta refines="#doi" property="identifier-type" scheme="onix:codelist5">06</meta>
  </metadata>
</package>`,
			want: []book.Identifier{
				{Scheme: "UUID", Value: "A1B0D67E-2E81-4DF5-9E67-A64CBE366809"},
				{Scheme: "ISBN", Value: "0306406152"},
				{Scheme: "ISBN", Value: "9780306406157"},
				{Scheme: "DOI", Value: "10.1016/j.iheduc.2008.03.001"},
			},
		},
		{
			name: "value prefix is split",
			doc: `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier>urn:isbn:978-0-306-40615-7</dc:identifier>
  </metadata>
</package>`,
			want: []book.Identifier{{Scheme: "ISBN", Value: "978-0-306-40615-7"}},
		},
		{
			name: "uuid and calibre value prefixes",
			doc: `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uuid_id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="uuid_id">uuid:06182534-7d14-4ef9-a738-843ff09973d8</dc:identifier>
    <dc:identifier>calibre:06182534-7d14-4ef9-a738-843ff09973d8</dc:identifier>
  </metadata>
</package>`,
			want: []book.Identifier{
				{Scheme: "UUID", Value: "06182534-7d14-4ef9-a738-843ff09973d8"},
				{Scheme: "calibre", Value: "06182534-7d14-4ef9-a738-843ff09973d8"},
			},
		},
		{
			name: "explicit scheme strips matching prefix",
			doc: `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" xmlns:opf="http://www.idpf.org/2007/opf" version="2.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier opf:scheme="ISBN">urn:isbn:978-0-306-40615-7</dc:identifier>
  </metadata>
</package>`,
			want: []book.Identifier{{Scheme: "ISBN", Value: "978-0-306-40615-7"}},
		},
		{
			name: "explicit scheme keeps non-matching prefix",
			doc: `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" xmlns:opf="http://www.idpf.org/2007/opf" version="2.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier opf:scheme="ISBN">calibre:978-0-306-40615-7</dc:identifier>
  </metadata>
</package>`,
			want: []book.Identifier{{Scheme: "ISBN", Value: "calibre:978-0-306-40615-7"}},
		},
		{
			name: "URL value keeps full value",
			doc: `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier>http://example.com/id</dc:identifier>
  </metadata>
</package>`,
			want: []book.Identifier{{Scheme: "URL", Value: "http://example.com/id"}},
		},
		{
			name: "unknown scheme is preserved",
			doc: `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" xmlns:opf="http://www.idpf.org/2007/opf" version="2.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier opf:scheme="Proprietary">ABC-123</dc:identifier>
  </metadata>
</package>`,
			want: []book.Identifier{{Scheme: "Proprietary", Value: "ABC-123"}},
		},
		{
			name: "schemeless identifier",
			doc: `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier>12345</dc:identifier>
  </metadata>
</package>`,
			want: []book.Identifier{{Value: "12345"}},
		},
		{
			name: "OEB 1.0 wrapped identifier",
			doc: `<?xml version="1.0" encoding="UTF-8" ?>
<package unique-identifier="uuid_id">
 <metadata>
  <dc-metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
   <dc:Identifier scheme="ISBN">1234567890</dc:Identifier>
  </dc-metadata>
 </metadata>
</package>`,
			want: []book.Identifier{{Scheme: "ISBN", Value: "1234567890"}},
		},
		{
			name: "empty values are skipped",
			doc: `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" xmlns:opf="http://www.idpf.org/2007/opf" version="2.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier opf:scheme="ISBN">   </dc:identifier>
    <dc:identifier opf:scheme="ISBN">9780306406157</dc:identifier>
  </metadata>
</package>`,
			want: []book.Identifier{{Scheme: "ISBN", Value: "9780306406157"}},
		},
		{
			name: "OPF2 source prefix",
			doc: `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" xmlns:opf="http://www.idpf.org/2007/opf" version="2.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:source>urn:isbn:9780375704024</dc:source>
  </metadata>
</package>`,
			want: []book.Identifier{{Scheme: "ISBN", Value: "9780375704024"}},
		},
		{
			name: "OPF2 source scheme attribute",
			doc: `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" xmlns:opf="http://www.idpf.org/2007/opf" version="2.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:source opf:scheme="ISBN">0-306-40615-2</dc:source>
  </metadata>
</package>`,
			want: []book.Identifier{{Scheme: "ISBN", Value: "0-306-40615-2"}},
		},
		{
			name: "EPUB3 source identifier-type refinement",
			doc: `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="isbn-id">urn:isbn:9780101010101</dc:identifier>
    <meta refines="#isbn-id" property="identifier-type" scheme="onix:codelist5">15</meta>
    <dc:source id="src-id">urn:isbn:9780375704024</dc:source>
    <meta refines="#src-id" property="identifier-type" scheme="onix:codelist5">15</meta>
  </metadata>
</package>`,
			want: []book.Identifier{
				{Scheme: "ISBN", Value: "9780101010101"},
				{Scheme: "ISBN", Value: "9780375704024"},
			},
		},
		{
			name: "sources follow identifiers",
			doc: `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:source>urn:isbn:9780375704024</dc:source>
    <dc:identifier>urn:isbn:9780101010101</dc:identifier>
  </metadata>
</package>`,
			want: []book.Identifier{
				{Scheme: "ISBN", Value: "9780101010101"},
				{Scheme: "ISBN", Value: "9780375704024"},
			},
		},
		{
			name: "source URL keeps full value",
			doc: `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:source>http://example.com/source</dc:source>
  </metadata>
</package>`,
			want: []book.Identifier{{Scheme: "URL", Value: "http://example.com/source"}},
		},
		{
			name: "schemeless source",
			doc: `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:source>9780375704024</dc:source>
  </metadata>
</package>`,
			want: []book.Identifier{{Value: "9780375704024"}},
		},
		{
			name: "OEB 1.0 wrapped source",
			doc: `<?xml version="1.0" encoding="UTF-8" ?>
<package unique-identifier="uuid_id">
 <metadata>
  <dc-metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
   <dc:Source scheme="ISBN">1234567890</dc:Source>
  </dc-metadata>
 </metadata>
</package>`,
			want: []book.Identifier{{Scheme: "ISBN", Value: "1234567890"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := Parse(strings.NewReader(tt.doc))
			if err != nil {
				t.Fatalf("Parse() error: %v", err)
			}
			if got := p.Identifiers(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Identifiers() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
