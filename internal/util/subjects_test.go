package util

import (
	"reflect"
	"testing"
)

func TestSplitSubjects(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   []string
	}{
		{name: "nil", values: nil, want: nil},
		{name: "empty", values: []string{}, want: nil},
		{name: "single", values: []string{"Fantasy"}, want: []string{"Fantasy"}},
		{
			name:   "semicolons split",
			values: []string{"Fantasy; Adventure"},
			want:   []string{"Fantasy", "Adventure"},
		},
		{
			name:   "parts trimmed and empty dropped",
			values: []string{" Science ; ; History ; "},
			want:   []string{"Science", "History"},
		},
		{
			name:   "source order preserved",
			values: []string{"A; B", "C", "D;E"},
			want:   []string{"A", "B", "C", "D", "E"},
		},
		{
			name:   "commas preserved",
			values: []string{"Science, Fiction"},
			want:   []string{"Science, Fiction"},
		},
		{
			name:   "duplicates preserved",
			values: []string{"A; A", "A"},
			want:   []string{"A", "A", "A"},
		},
		{
			name:   "whitespace only",
			values: []string{" ", "\t", "\n"},
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SplitSubjects(tt.values); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitSubjects(%#v) = %#v, want %#v", tt.values, got, tt.want)
			}
		})
	}
}

func TestDedupeSubjects(t *testing.T) {
	tests := []struct {
		name     string
		subjects []string
		want     []string
	}{
		{name: "nil", subjects: nil, want: nil},
		{name: "empty", subjects: []string{}, want: nil},
		{
			name:     "no duplicates",
			subjects: []string{"Fantasy", "Adventure"},
			want:     []string{"Fantasy", "Adventure"},
		},
		{
			name:     "exact duplicates",
			subjects: []string{"Fantasy", "Fantasy"},
			want:     []string{"Fantasy"},
		},
		{
			name:     "case-insensitive keeps first spelling",
			subjects: []string{"Fantasy", "fantasy", "FANTASY"},
			want:     []string{"Fantasy"},
		},
		{
			name:     "source order preserved",
			subjects: []string{"sf", "Fantasy", "sf", "Adventure", "fantasy"},
			want:     []string{"sf", "Fantasy", "Adventure"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DedupeSubjects(tt.subjects); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DedupeSubjects(%#v) = %#v, want %#v", tt.subjects, got, tt.want)
			}
		})
	}
}
