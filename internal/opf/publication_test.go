package opf

import (
	"strings"
	"testing"
)

func TestPublisher(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want string
	}{
		{
			name: "direct",
			doc: `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` +
				`<dc:publisher>Acme Press</dc:publisher></metadata></package>`,
			want: "Acme Press",
		},
		{
			name: "whitespace trimmed",
			doc: `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` +
				`<dc:publisher>  Acme  Press  </dc:publisher></metadata></package>`,
			want: "Acme  Press",
		},
		{
			name: "first non-empty wins",
			doc: `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` +
				`<dc:publisher> </dc:publisher><dc:publisher>First</dc:publisher>` +
				`<dc:publisher>Second</dc:publisher></metadata></package>`,
			want: "First",
		},
		{
			name: "empty",
			doc: `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` +
				`<dc:publisher> </dc:publisher></metadata></package>`,
			want: "",
		},
		{
			name: "missing",
			doc:  `<package><metadata></metadata></package>`,
			want: "",
		},
		{
			name: "custom prefix",
			doc: `<package><metadata xmlns:custom="http://purl.org/dc/elements/1.1/">` +
				`<custom:publisher>Custom Press</custom:publisher></metadata></package>`,
			want: "Custom Press",
		},
		{
			name: "OEB wrapped fallback",
			doc: `<package><metadata><dc-metadata>` +
				`<dc:Publisher>Wrapped Press</dc:Publisher></dc-metadata></metadata></package>`,
			want: "Wrapped Press",
		},
		{
			name: "direct beats wrapped",
			doc: `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` +
				`<dc:publisher>Direct Press</dc:publisher>` +
				`<dc-metadata><dc:Publisher>Wrapped Press</dc:Publisher></dc-metadata>` +
				`</metadata></package>`,
			want: "Direct Press",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := Parse(strings.NewReader(tt.doc))
			if err != nil {
				t.Fatalf("Parse() error: %v", err)
			}
			if got := p.Publisher(); got != tt.want {
				t.Errorf("Publisher() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPublicationDate(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want string
	}{
		{
			name: "direct",
			doc: `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` +
				`<dc:date>2000-01-01T00:00:00Z</dc:date></metadata></package>`,
			want: "2000-01-01T00:00:00Z",
		},
		{
			name: "raw value returned unparsed",
			doc: `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` +
				`<dc:date>not a date</dc:date></metadata></package>`,
			want: "not a date",
		},
		{
			name: "whitespace trimmed",
			doc: `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` +
				`<dc:date>  1999  </dc:date></metadata></package>`,
			want: "1999",
		},
		{
			name: "first non-empty wins",
			doc: `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` +
				`<dc:date> </dc:date><dc:date>2001</dc:date><dc:date>2002</dc:date>` +
				`</metadata></package>`,
			want: "2001",
		},
		{
			name: "publication event preferred",
			doc: `<package xmlns:opf="http://www.idpf.org/2007/opf">` +
				`<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` +
				`<dc:date opf:event="modification">2020-01-01</dc:date>` +
				`<dc:date opf:event="publication">1999-05-04</dc:date>` +
				`</metadata></package>`,
			want: "1999-05-04",
		},
		{
			name: "unprefixed publication event preferred",
			doc: `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` +
				`<dc:date event="modification">2020-01-01</dc:date>` +
				`<dc:date event="publication">1999-05-04</dc:date>` +
				`</metadata></package>`,
			want: "1999-05-04",
		},
		{
			name: "empty publication event ignored",
			doc: `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` +
				`<dc:date event="publication"> </dc:date>` +
				`<dc:date event="modification">2020-01-01</dc:date>` +
				`</metadata></package>`,
			want: "",
		},
		{
			name: "non-publication events ignored",
			doc: `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` +
				`<dc:date event="creation">2010-01-01</dc:date>` +
				`<dc:date event="modification">2020-01-01</dc:date>` +
				`</metadata></package>`,
			want: "",
		},
		{
			name: "publication event beats unqualified date",
			doc: `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` +
				`<dc:date>2010-01-01</dc:date>` +
				`<dc:date event="publication">1999-05-04</dc:date>` +
				`</metadata></package>`,
			want: "1999-05-04",
		},
		{
			name: "non-publication direct falls back to wrapped",
			doc: `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` +
				`<dc:date event="modification">2020-01-01</dc:date>` +
				`<dc-metadata><dc:Date>1998</dc:Date></dc-metadata>` +
				`</metadata></package>`,
			want: "1998",
		},
		{
			name: "wrapped publication beats direct unqualified",
			doc: `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` +
				`<dc:date>2001</dc:date>` +
				`<dc-metadata><dc:Date event="publication">1998</dc:Date></dc-metadata>` +
				`</metadata></package>`,
			want: "1998",
		},
		{
			name: "missing",
			doc:  `<package><metadata></metadata></package>`,
			want: "",
		},
		{
			name: "custom prefix",
			doc: `<package><metadata xmlns:custom="http://purl.org/dc/elements/1.1/">` +
				`<custom:date>2005</custom:date></metadata></package>`,
			want: "2005",
		},
		{
			name: "OEB wrapped fallback",
			doc: `<package><metadata><dc-metadata>` +
				`<dc:Date>1998</dc:Date></dc-metadata></metadata></package>`,
			want: "1998",
		},
		{
			name: "direct beats wrapped",
			doc: `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` +
				`<dc:date>2001</dc:date>` +
				`<dc-metadata><dc:Date>1998</dc:Date></dc-metadata>` +
				`</metadata></package>`,
			want: "2001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := Parse(strings.NewReader(tt.doc))
			if err != nil {
				t.Fatalf("Parse() error: %v", err)
			}
			if got := p.PublicationDate(); got != tt.want {
				t.Errorf("PublicationDate() = %q, want %q", got, tt.want)
			}
		})
	}
}
