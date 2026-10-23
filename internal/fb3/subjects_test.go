package fb3

import (
	"reflect"
	"testing"
)

func TestParseDescriptionSubjects(t *testing.T) {
	wrap := func(body string) []byte {
		return []byte(`<fb3-description xmlns="http://www.fictionbook.org/FictionBook3/description" id="book-id">` +
			`<title><main>T</main></title>` + body + `<lang>en</lang></fb3-description>`)
	}

	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "single subject",
			body: `<fb3-classification><subject>fantasy</subject></fb3-classification>`,
			want: []string{"fantasy"},
		},
		{
			name: "multiple subjects in source order",
			body: `<fb3-classification><subject>sf</subject><subject>fantasy</subject><subject>adventure</subject></fb3-classification>`,
			want: []string{"sf", "fantasy", "adventure"},
		},
		{
			name: "whitespace collapsed and empty dropped",
			body: "<fb3-classification><subject>  science   fiction \n</subject><subject> </subject><subject>fantasy</subject></fb3-classification>",
			want: []string{"science fiction", "fantasy"},
		},
		{
			name: "deduplicated case-insensitively keeping first spelling",
			body: `<fb3-classification><subject>Fantasy</subject><subject>fantasy</subject><subject>FANTASY</subject><subject>SF</subject></fb3-classification>`,
			want: []string{"Fantasy", "SF"},
		},
		{
			name: "class is not a subject",
			body: `<fb3-classification><class contents="standalone">novel</class><subject>fantasy</subject></fb3-classification>`,
			want: []string{"fantasy"},
		},
		{
			name: "udc and bbk are not subjects",
			body: `<fb3-classification><subject>fantasy</subject><udc>821.111</udc><bbk>84(4)</bbk></fb3-classification>`,
			want: []string{"fantasy"},
		},
		{
			name: "target audience and setting ignored",
			body: `<fb3-classification><subject>fantasy</subject><target-audience age-min="12">teens</target-audience><setting country="UK"/></fb3-classification>`,
			want: []string{"fantasy"},
		},
		{
			name: "missing classification",
			body: `<annotation><p>x</p></annotation>`,
			want: nil,
		},
		{
			name: "empty classification",
			body: `<fb3-classification></fb3-classification>`,
			want: nil,
		},
		{
			name: "empty subjects only",
			body: `<fb3-classification><subject> </subject><subject></subject></fb3-classification>`,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := parseFB3(t, wrap(tt.body))
			if !reflect.DeepEqual(md.Subjects, tt.want) {
				t.Errorf("Subjects = %#v, want %#v", md.Subjects, tt.want)
			}
		})
	}
}

func TestParseDescriptionSubjectsIgnoreRelations(t *testing.T) {
	data := `<fb3-description xmlns="http://www.fictionbook.org/FictionBook3/description">` +
		`<title><main>T</main></title>` +
		`<fb3-relations>` +
		`<subject link="author"><last-name>Author</last-name></subject>` +
		`<subject link="editor"><last-name>Editor</last-name></subject>` +
		`</fb3-relations>` +
		`<fb3-classification><subject>fantasy</subject></fb3-classification>` +
		`<lang>en</lang></fb3-description>`
	md := parseFB3(t, []byte(data))
	want := []string{"fantasy"}
	if !reflect.DeepEqual(md.Subjects, want) {
		t.Errorf("Subjects = %#v, want %#v", md.Subjects, want)
	}
}

func TestParseDescriptionSubjectsPrefixed(t *testing.T) {
	data := `<fb3d:fb3-description xmlns:fb3d="http://www.fictionbook.org/FictionBook3/description">` +
		`<fb3d:title><fb3d:main>T</fb3d:main></fb3d:title>` +
		`<fb3d:fb3-classification><fb3d:subject>fantasy</fb3d:subject></fb3d:fb3-classification>` +
		`<fb3d:lang>en</fb3d:lang></fb3d:fb3-description>`
	md := parseFB3(t, []byte(data))
	want := []string{"fantasy"}
	if !reflect.DeepEqual(md.Subjects, want) {
		t.Errorf("Subjects = %#v, want %#v", md.Subjects, want)
	}
}
