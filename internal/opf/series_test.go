package opf

import (
	"strings"
	"testing"
)

func TestSeries(t *testing.T) {
	tests := []struct {
		name      string
		doc       string
		wantName  string
		wantIndex string
	}{
		{
			name: "EPUB 3 canonical",
			doc: `<package><metadata>` +
				`<meta property="belongs-to-collection" id="c1">Saga</meta>` +
				`<meta refines="#c1" property="collection-type">series</meta>` +
				`<meta refines="#c1" property="group-position">2.5</meta>` +
				`</metadata></package>`,
			wantName:  "Saga",
			wantIndex: "2.5",
		},
		{
			name: "canonical values trimmed and index verbatim",
			doc: `<package><metadata>` +
				`<meta property="belongs-to-collection" id="c1">  The  Saga  </meta>` +
				`<meta refines="#c1" property="collection-type"> series </meta>` +
				`<meta refines="#c1" property="group-position"> 02.50 </meta>` +
				`</metadata></package>`,
			wantName:  "The  Saga",
			wantIndex: "02.50",
		},
		{
			name: "canonical without group-position has empty index",
			doc: `<package><metadata>` +
				`<meta property="belongs-to-collection" id="c1">Saga</meta>` +
				`<meta refines="#c1" property="collection-type">series</meta>` +
				`<meta name="calibre:series_index" content="9"/>` +
				`</metadata></package>`,
			wantName:  "Saga",
			wantIndex: "",
		},
		{
			name: "canonical wins over calibre",
			doc: `<package><metadata>` +
				`<meta name="calibre:series" content="Calibre Series"/>` +
				`<meta name="calibre:series_index" content="9"/>` +
				`<meta property="belongs-to-collection" id="c1">Saga</meta>` +
				`<meta refines="#c1" property="collection-type">series</meta>` +
				`<meta refines="#c1" property="group-position">2</meta>` +
				`</metadata></package>`,
			wantName:  "Saga",
			wantIndex: "2",
		},
		{
			name: "non-series collection falls back to calibre",
			doc: `<package><metadata>` +
				`<meta property="belongs-to-collection" id="c1">Other</meta>` +
				`<meta refines="#c1" property="collection-type">collection</meta>` +
				`<meta name="calibre:series" content="Calibre Series"/>` +
				`<meta name="calibre:series_index" content="3"/>` +
				`</metadata></package>`,
			wantName:  "Calibre Series",
			wantIndex: "3",
		},
		{
			name: "first non-empty canonical wins",
			doc: `<package><metadata>` +
				`<meta property="belongs-to-collection" id="c1">  </meta>` +
				`<meta refines="#c1" property="collection-type">series</meta>` +
				`<meta property="belongs-to-collection" id="c2">Second</meta>` +
				`<meta refines="#c2" property="collection-type">series</meta>` +
				`<meta refines="#c2" property="group-position">4</meta>` +
				`</metadata></package>`,
			wantName:  "Second",
			wantIndex: "4",
		},
		{
			name: "calibre fallback",
			doc: `<package><metadata>` +
				`<meta name="calibre:series_index" content="2.5"/>` +
				`<meta name="calibre:series" content="Discworld"/>` +
				`</metadata></package>`,
			wantName:  "Discworld",
			wantIndex: "2.5",
		},
		{
			name: "calibre without index",
			doc: `<package><metadata>` +
				`<meta name="calibre:series" content="Discworld"/>` +
				`</metadata></package>`,
			wantName:  "Discworld",
			wantIndex: "",
		},
		{
			name: "calibre index without series",
			doc: `<package><metadata>` +
				`<meta name="calibre:series_index" content="2.5"/>` +
				`</metadata></package>`,
			wantName:  "",
			wantIndex: "",
		},
		{
			name: "first non-empty calibre series wins",
			doc: `<package><metadata>` +
				`<meta name="calibre:series" content=" "/>` +
				`<meta name="calibre:series" content="First"/>` +
				`<meta name="calibre:series" content="Second"/>` +
				`</metadata></package>`,
			wantName:  "First",
			wantIndex: "",
		},
		{
			name: "refinement for another id is ignored",
			doc: `<package><metadata>` +
				`<meta property="belongs-to-collection" id="c1">Saga</meta>` +
				`<meta refines="#c2" property="collection-type">series</meta>` +
				`</metadata></package>`,
			wantName:  "",
			wantIndex: "",
		},
		{
			name: "malformed group-position kept verbatim",
			doc: `<package><metadata>` +
				`<meta property="belongs-to-collection" id="c1">Saga</meta>` +
				`<meta refines="#c1" property="collection-type">series</meta>` +
				`<meta refines="#c1" property="group-position">x</meta>` +
				`</metadata></package>`,
			wantName:  "Saga",
			wantIndex: "x",
		},
		{
			name: "OEB wrapped calibre metas",
			doc: `<package><metadata><x-metadata>` +
				`<meta name="calibre:series" content="Wrapped"/>` +
				`<meta name="calibre:series_index" content="7"/>` +
				`</x-metadata></metadata></package>`,
			wantName:  "Wrapped",
			wantIndex: "7",
		},
		{
			name:      "missing",
			doc:       `<package><metadata></metadata></package>`,
			wantName:  "",
			wantIndex: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := Parse(strings.NewReader(tt.doc))
			if err != nil {
				t.Fatalf("Parse() error: %v", err)
			}
			name, index := p.Series()
			if name != tt.wantName || index != tt.wantIndex {
				t.Errorf("Series() = (%q, %q), want (%q, %q)", name, index, tt.wantName, tt.wantIndex)
			}
		})
	}
}
