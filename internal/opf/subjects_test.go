package opf

import (
	"reflect"
	"strings"
	"testing"
)

func TestSubjects(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want []string
	}{
		{
			name: "single",
			doc: `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` +
				`<dc:subject>Fantasy</dc:subject></metadata></package>`,
			want: []string{"Fantasy"},
		},
		{
			name: "multiple in source order",
			doc: `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` +
				`<dc:subject>Fantasy</dc:subject><dc:subject>Adventure</dc:subject>` +
				`<dc:subject>History</dc:subject></metadata></package>`,
			want: []string{"Fantasy", "Adventure", "History"},
		},
		{
			name: "trimmed and empty dropped",
			doc: `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` +
				`<dc:subject>  Science  </dc:subject><dc:subject> </dc:subject>` +
				`<dc:subject></dc:subject><dc:subject>` + "\n\t" + `</dc:subject>` +
				`<dc:subject>History</dc:subject></metadata></package>`,
			want: []string{"Science", "History"},
		},
		{
			name: "missing",
			doc:  `<package><metadata></metadata></package>`,
			want: nil,
		},
		{
			name: "custom prefix",
			doc: `<package><metadata xmlns:custom="http://purl.org/dc/elements/1.1/">` +
				`<custom:subject>Custom</custom:subject></metadata></package>`,
			want: []string{"Custom"},
		},
		{
			name: "unprefixed",
			doc:  `<package><metadata><subject>Plain</subject></metadata></package>`,
			want: []string{"Plain"},
		},
		{
			name: "OEB wrapped",
			doc: `<package><metadata><dc-metadata>` +
				`<dc:Subject>Wrapped</dc:Subject><dc:Subject>Second</dc:Subject>` +
				`</dc-metadata></metadata></package>`,
			want: []string{"Wrapped", "Second"},
		},
		{
			name: "direct before wrapped",
			doc: `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` +
				`<dc:subject>Direct</dc:subject>` +
				`<dc-metadata><dc:Subject>Wrapped</dc:Subject></dc-metadata>` +
				`</metadata></package>`,
			want: []string{"Direct", "Wrapped"},
		},
		{
			name: "attributes ignored",
			doc: `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` +
				`<dc:subject id="s1" lang="en">Science</dc:subject></metadata></package>`,
			want: []string{"Science"},
		},
		{
			name: "entities decoded",
			doc: `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` +
				`<dc:subject>Science &amp; Technology</dc:subject></metadata></package>`,
			want: []string{"Science & Technology"},
		},
		{
			name: "semicolons split",
			doc: `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` +
				`<dc:subject>Fantasy; Adventure</dc:subject></metadata></package>`,
			want: []string{"Fantasy", "Adventure"},
		},
		{
			name: "semicolon parts trimmed and empty dropped",
			doc: `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` +
				`<dc:subject> Science ; ; History ; </dc:subject></metadata></package>`,
			want: []string{"Science", "History"},
		},
		{
			name: "direct and wrapped semicolons in source order",
			doc: `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` +
				`<dc:subject>Direct; Second</dc:subject>` +
				`<dc-metadata><dc:Subject>Wrapped; Last</dc:Subject></dc-metadata>` +
				`</metadata></package>`,
			want: []string{"Direct", "Second", "Wrapped", "Last"},
		},
		{
			name: "commas kept intact",
			doc: `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` +
				`<dc:subject>Science, Fiction</dc:subject></metadata></package>`,
			want: []string{"Science, Fiction"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := Parse(strings.NewReader(tt.doc))
			if err != nil {
				t.Fatalf("Parse() error: %v", err)
			}
			if got := p.Subjects(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Subjects() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
