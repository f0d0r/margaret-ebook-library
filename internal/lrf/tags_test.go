package lrf

import (
	"bytes"
	"testing"
)

func TestTagSize(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want int
		ok   bool
	}{
		{"fixed zero payload", []byte{0x01, 0xF5}, 2, true},
		{"fixed six payload", []byte{0x00, 0xF5, 1, 2, 3, 4, 5, 6}, 8, true},
		{"fixed dword", []byte{0x7B, 0xF5, 1, 2, 3, 4}, 6, true},
		{"string", []byte{0x16, 0xF5, 0x04, 0x00, 'a', 0, 'b', 0}, 8, true},
		{"string empty", []byte{0x5D, 0xF5, 0x00, 0x00}, 4, true},
		{"string truncated payload", []byte{0x16, 0xF5, 0x08, 0x00, 'a', 0}, 0, false},
		{"string truncated header", []byte{0x16, 0xF5, 0x04}, 0, false},
		{"type_one", []byte{0x0B, 0xF5, 0x02, 0x00, 1, 0, 0, 0, 2, 0, 0, 0}, 12, true},
		{"type_one truncated", []byte{0x0B, 0xF5, 0x02, 0x00, 1, 0, 0, 0, 2, 0, 0}, 0, false},
		{"tag_78", []byte{0x78, 0xF5, 1, 0, 0, 0, 0x16, 0xF5, 0x02, 0x00, 'a', 0, 7, 0}, 14, true},
		{"tag_78 wrong inner tag", []byte{0x78, 0xF5, 1, 0, 0, 0, 0x15, 0xF5, 0x02, 0x00, 'a', 0, 7, 0}, 0, false},
		{"tag_78 truncated", []byte{0x78, 0xF5, 1, 0, 0, 0, 0x16, 0xF5, 0x08, 0x00, 'a', 0}, 0, false},
		{"unknown low byte", []byte{0xA5, 0xF5}, 0, false},
		{"invalid low byte", []byte{0x0C, 0xF5}, 0, false},
		{"bad magic", []byte{0x00, 0x00}, 0, false},
		{"truncated id", []byte{0x00}, 0, false},
		{"empty", nil, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tagSize(tt.data, 0)
			if got != tt.want || ok != tt.ok {
				t.Errorf("tagSize(% X) = (%d, %v), want (%d, %v)", tt.data, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestTagSizeAtOffset(t *testing.T) {
	data := []byte{0xFF, 0xFF, 0x7B, 0xF5, 1, 0, 0, 0}
	if got, ok := tagSize(data, 2); !ok || got != 6 {
		t.Errorf("tagSize at offset 2 = (%d, %v), want (6, true)", got, ok)
	}
}

func TestStringTagText(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		want    string
		ok      bool
	}{
		{"ascii", []byte{0x04, 0x00, 'a', 0, 'b', 0}, "ab", true},
		{"empty", []byte{0x00, 0x00}, "", true},
		{"odd length", []byte{0x03, 0x00, 'a', 0, 'b'}, "", false},
		{"count beyond payload", []byte{0x10, 0x00, 'a', 0}, "", false},
		{"missing count", []byte{0x04}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := stringTagText(tt.payload)
			if got != tt.want || ok != tt.ok {
				t.Errorf("stringTagText(% X) = (%q, %v), want (%q, %v)", tt.payload, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestParseTypeOnePayload(t *testing.T) {
	got, ok := parseTypeOnePayload([]byte{0x02, 0x00, 1, 0, 0, 0, 2, 0, 0, 0})
	if !ok || len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Errorf("parseTypeOnePayload() = (%v, %v), want ([1 2], true)", got, ok)
	}
	if _, ok := parseTypeOnePayload([]byte{0x02, 0x00, 1, 0, 0, 0}); ok {
		t.Error("parseTypeOnePayload() with truncated values = ok, want false")
	}
	if _, ok := parseTypeOnePayload(nil); ok {
		t.Error("parseTypeOnePayload(nil) = ok, want false")
	}
}

func TestTagScanner(t *testing.T) {
	var data []byte
	data = append(data, 0x7B, 0xF5, 9, 0, 0, 0)             // F57B dword
	data = append(data, 0x54, 0xF5, 0x00, 0x01)             // F554 flags 0x100
	data = append(data, 0x04, 0xF5, 0x03, 0x00, 0x00, 0x00) // F504 size 3
	data = append(data, 0x05, 0xF5, 0xAA, 0xBB, 0xCC)       // F505 payload
	data = append(data, 0x06, 0xF5)                         // F506
	data = append(data, 0x01, 0xF5)                         // F501

	sc := newTagScanner(data)
	var ids []uint32
	var stream []byte
	for {
		id, payload, ok, err := sc.next()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
		ids = append(ids, id)
		if id == tagStreamStart {
			stream = payload
		}
	}
	want := []uint32{tagBookPageTree, tagStreamFlags, tagStreamSize, tagStreamStart, tagStreamEnd, tagObjectEnd}
	if len(ids) != len(want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("ids = %v, want %v", ids, want)
		}
	}
	if !bytes.Equal(stream, []byte{0xAA, 0xBB, 0xCC}) {
		t.Errorf("stream = % X, want AA BB CC", stream)
	}
	if sc.flags != 0x100 {
		t.Errorf("flags = %#x, want 0x100", sc.flags)
	}
}

func TestTagScannerErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"bad magic", []byte{0x00, 0x00}},
		{"unknown tag", []byte{0x0C, 0xF5}},
		{"truncated string", []byte{0x16, 0xF5, 0x08, 0x00, 'a', 0}},
		{"stream without flags", []byte{0x04, 0xF5, 0x01, 0x00, 0x00, 0x00, 0x05, 0xF5, 0xAA}},
		{"stream flags after stream", []byte{0x54, 0xF5, 0x00, 0x00, 0x04, 0xF5, 0x01, 0x00, 0x00, 0x00, 0x05, 0xF5, 0xAA, 0x54, 0xF5, 0x00, 0x00, 0x05, 0xF5, 0xBB}},
		{"stream without size", []byte{0x54, 0xF5, 0x00, 0x00, 0x05, 0xF5, 0xAA}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := newTagScanner(tt.data)
			for {
				_, _, ok, err := sc.next()
				if err != nil {
					return
				}
				if !ok {
					t.Fatal("scanner ended without error, want error")
				}
			}
		})
	}
}

func TestTagScannerStreamSizeFallback(t *testing.T) {
	// F504 absent: the stream size defaults to zero.
	data := []byte{0x54, 0xF5, 0x11, 0x00, 0x05, 0xF5, 0x06, 0xF5}
	sc := newTagScanner(data)
	for {
		id, payload, ok, err := sc.next()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
		if id == tagStreamStart && len(payload) != 0 {
			t.Fatalf("stream payload = % X, want empty", payload)
		}
	}
}

func TestFindDwordTags(t *testing.T) {
	var data []byte
	data = append(data, 0x03, 0xF5, 3, 0, 0, 0)
	data = append(data, 0x4E, 0xF5, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)
	data = append(data, 0x03, 0xF5, 4, 0, 0, 0)

	got, err := findDwordTags(data, tagLink)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != 3 || got[1] != 4 {
		t.Errorf("findDwordTags() = %v, want [3 4]", got)
	}
	first, ok, err := findDwordTag(data, tagLink)
	if err != nil || !ok || first != 3 {
		t.Errorf("findDwordTag() = (%d, %v, %v), want (3, true, nil)", first, ok, err)
	}
}
