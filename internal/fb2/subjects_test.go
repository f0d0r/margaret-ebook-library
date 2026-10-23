package fb2

import (
	"reflect"
	"testing"
)

func TestParseMetadataSubjects(t *testing.T) {
	wrap := func(desc string) []byte {
		return []byte(`<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0"><description>` + desc + `</description><body><p>x</p></body></FictionBook>`)
	}

	tests := []struct {
		name string
		desc string
		want []string
	}{
		{
			name: "single genre",
			desc: `<title-info><genre>sf</genre></title-info>`,
			want: []string{"sf"},
		},
		{
			name: "multiple genres in source order",
			desc: `<title-info><genre>sf</genre><genre>fantasy</genre><genre>adventure</genre></title-info>`,
			want: []string{"sf", "fantasy", "adventure"},
		},
		{
			name: "genres trimmed and empty dropped",
			desc: `<title-info><genre>  sf  </genre><genre> </genre><genre></genre><genre>fantasy</genre></title-info>`,
			want: []string{"sf", "fantasy"},
		},
		{
			name: "genre match attribute ignored",
			desc: `<title-info><genre match="50">sf</genre></title-info>`,
			want: []string{"sf"},
		},
		{
			name: "keywords split on commas",
			desc: `<title-info><genre>sf</genre><keywords>fantasy, adventure, history</keywords></title-info>`,
			want: []string{"sf", "fantasy", "adventure", "history"},
		},
		{
			name: "keywords split on semicolons",
			desc: `<title-info><genre>sf</genre><keywords>fantasy; adventure</keywords></title-info>`,
			want: []string{"sf", "fantasy", "adventure"},
		},
		{
			name: "keyword parts trimmed and empty dropped",
			desc: `<title-info><genre>sf</genre><keywords> fantasy , , adventure , </keywords></title-info>`,
			want: []string{"sf", "fantasy", "adventure"},
		},
		{
			name: "title-info wins over src-title-info",
			desc: `<title-info><genre>sf</genre><keywords>fantasy</keywords></title-info>` +
				`<src-title-info><genre>history</genre><keywords>war</keywords></src-title-info>`,
			want: []string{"sf", "fantasy"},
		},
		{
			name: "title-info keywords suppress src-title-info genres",
			desc: `<title-info><keywords>fantasy</keywords></title-info>` +
				`<src-title-info><genre>history</genre></src-title-info>`,
			want: []string{"fantasy"},
		},
		{
			name: "src-title-info fallback",
			desc: `<title-info></title-info>` +
				`<src-title-info><genre>history</genre><keywords>war, politics</keywords></src-title-info>`,
			want: []string{"history", "war", "politics"},
		},
		{
			name: "genres and keywords deduplicated case-insensitively",
			desc: `<title-info><genre>Fantasy</genre><keywords>fantasy, Adventure, FANTASY</keywords></title-info>`,
			want: []string{"Fantasy", "Adventure"},
		},
		{
			name: "duplicate genres keep first spelling",
			desc: `<title-info><genre>SF</genre><genre>sf</genre><genre>Sf</genre></title-info>`,
			want: []string{"SF"},
		},
		{
			name: "duplicate keywords deduplicated",
			desc: `<title-info><genre>sf</genre><keywords>fantasy, fantasy, FANTASY</keywords></title-info>`,
			want: []string{"sf", "fantasy"},
		},
		{
			name: "missing",
			desc: `<title-info><book-title>T</book-title></title-info>`,
			want: nil,
		},
		{
			name: "empty values only",
			desc: `<title-info><genre> </genre><keywords> , ; </keywords></title-info>`,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md, _ := parseBytes(t, wrap(tt.desc))
			if !reflect.DeepEqual(md.Subjects, tt.want) {
				t.Errorf("Subjects = %#v, want %#v", md.Subjects, tt.want)
			}
		})
	}
}

func TestParseMetadataSubjectsPrefixed(t *testing.T) {
	data := `<fb:FictionBook xmlns:fb="http://www.gribuser.ru/xml/fictionbook/2.0">` +
		`<fb:description><fb:title-info><fb:genre>sf</fb:genre>` +
		`<fb:keywords>fantasy, adventure</fb:keywords></fb:title-info></fb:description>` +
		`<fb:body><fb:p>x</fb:p></fb:body></fb:FictionBook>`
	md, _ := parseBytes(t, []byte(data))
	want := []string{"sf", "fantasy", "adventure"}
	if !reflect.DeepEqual(md.Subjects, want) {
		t.Errorf("Subjects = %#v, want %#v", md.Subjects, want)
	}
}
