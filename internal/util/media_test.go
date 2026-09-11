package util

import (
	"testing"
)

func TestDetectImageMedia(t *testing.T) {
	pngHeader := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}

	tests := []struct {
		name    string
		data    []byte
		wantNil bool
		wantTyp string
		wantExt string
	}{
		{name: "nil", data: nil, wantNil: true},
		{name: "empty", data: []byte{}, wantNil: true},
		{name: "single byte", data: []byte{0xFF}, wantNil: true},
		{name: "truncated jpeg", data: []byte{0xFF, 0xD8}, wantNil: true},
		{name: "jpeg", data: []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}, wantTyp: "image/jpeg", wantExt: "jpg"},
		{name: "jpeg minimal", data: []byte{0xFF, 0xD8, 0xFF}, wantTyp: "image/jpeg", wantExt: "jpg"},
		{name: "png", data: append(append([]byte{}, pngHeader...), 0x00, 0x01), wantTyp: "image/png", wantExt: "png"},
		{name: "png truncated", data: []byte{0x89, 'P', 'N', 'G'}, wantNil: true},
		{name: "gif87a", data: []byte("GIF87a\x01\x00"), wantTyp: "image/gif", wantExt: "gif"},
		{name: "gif89a", data: []byte("GIF89a\x01\x00"), wantTyp: "image/gif", wantExt: "gif"},
		{name: "svg", data: []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`), wantTyp: "image/svg+xml", wantExt: "svg"},
		{name: "svg leading whitespace", data: []byte("  \n <svg></svg>"), wantTyp: "image/svg+xml", wantExt: "svg"},
		{name: "svg via xml prolog", data: []byte(`<?xml version="1.0"?><svg></svg>`), wantTyp: "image/svg+xml", wantExt: "svg"},
		{name: "unknown text", data: []byte("hello world, this is not an image"), wantNil: true},
		{name: "unknown binary", data: []byte{0x00, 0x01, 0x02, 0x03, 0x04}, wantNil: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectImageMedia(tt.data)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("DetectImageMedia() = %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("DetectImageMedia() = nil, want Type %q", tt.wantTyp)
			}
			if got.Type != tt.wantTyp {
				t.Errorf("Type = %q, want %q", got.Type, tt.wantTyp)
			}
			if got.Extension != tt.wantExt {
				t.Errorf("Extension = %q, want %q", got.Extension, tt.wantExt)
			}
		})
	}
}
