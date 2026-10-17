package charset

import "testing"

func TestDetectDeclared(t *testing.T) {
	tests := []struct {
		name string
		head string
		want string
		ok   bool
	}{
		{
			"xml declaration",
			`<?xml version="1.0" encoding="windows-1251"?><FictionBook><body><p>Привет</p></body></FictionBook>`,
			"windows-1251", true,
		},
		{
			"html5 meta",
			`<html><head><meta charset="windows-1252"><title>t</title></head><body><p>café</p></body></html>`,
			"windows-1252", true,
		},
		{
			"http-equiv meta",
			`<html><head><meta http-equiv="Content-Type" content="text/html; charset=windows-1251"></head></html>`,
			"windows-1251", true,
		},
		{
			"utf-8 declaration reports no transcode",
			`<?xml version="1.0" encoding="utf-8"?><html><body>x</body></html>`,
			UTF8, false,
		},
		{
			"no declaration",
			`<html><body>plain</body></html>`,
			UTF8, false,
		},
		{
			"html text mentioning encoding is not a declaration",
			`<html><head><title>Why encoding="windows-1251" is obsolete</title><meta charset="utf-8"></head></html>`,
			UTF8, false,
		},
		{
			"attribute coincidence is not a declaration",
			`<html><body><a data-encoding="windows-1251">x</a></body></html>`,
			UTF8, false,
		},
		{
			"unknown label ignored",
			`<html><head><meta charset="not-a-real-charset"></head></html>`,
			UTF8, false,
		},
		{
			"utf-16le declaration",
			`<?xml version="1.0" encoding="UTF-16LE"?><html/>`,
			"utf-16le", true,
		},
		{
			"utf-16le BOM",
			"\xff\xfe<\x00?\x00x\x00m\x00l\x00",
			"utf-16le", true,
		},
		{
			"utf-16be BOM",
			"\xfe\xff\x00<\x00?\x00x\x00m\x00l",
			"utf-16be", true,
		},
		{
			"utf-16le skeleton without BOM",
			"<\x00!\x00D\x00O\x00C\x00T\x00Y\x00P\x00E",
			"utf-16le", true,
		},
		{
			"utf-16be skeleton without BOM",
			"\x00<\x00!\x00D\x00O\x00C\x00T\x00Y\x00P\x00E",
			"utf-16be", true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := DetectDeclared([]byte(tt.head))
			if got != tt.want || ok != tt.ok {
				t.Errorf("DetectDeclared() = (%q, %v), want (%q, %v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}
