package util

import (
	"testing"
)

func TestScanOpenTag(t *testing.T) {
	tests := []struct {
		name        string
		buf         string
		tag         string
		foldCase    bool
		stripPrefix bool
		want        bool
	}{
		{"exact match", "<FictionBook>", "FictionBook", false, false, true},
		{"exact mismatch case", "<fictionbook>", "FictionBook", false, false, false},
		{"fold matches lower", "<fictionbook>", "FictionBook", true, false, true},
		{"fold matches upper", "<HTML>", "html", true, false, true},
		{"fold mismatch", "<body>", "html", true, false, false},
		{"prefix stripped", "<fb:FictionBook>", "FictionBook", false, true, true},
		{"prefix kept misses", "<fb:FictionBook>", "FictionBook", false, false, false},
		{"prefix without prefix still matches", "<FictionBook>", "FictionBook", false, true, true},
		{"delimiter fail", "<htmlx>", "html", true, false, false},
		{"delimiter space", "<html lang=\"en\">", "html", true, false, true},
		{"delimiter slash", "<br/>", "br", true, false, true},
		{"closing skipped", "</html>", "html", true, false, false},
		{"pi skipped", "<?xml version=\"1.0\"?>", "xml", true, false, false},
		{"decl skipped", "<!DOCTYPE html>", "html", true, false, false},
		{"prose mention", "I like html soup", "html", true, false, false},
		{"truncated tag counts", "<htm", "htm", true, false, true},
		{"lone bracket", "a < b", "b", true, false, false},
		{"empty", "", "html", true, false, false},
		{"tag after junk", "\x0e\x00<HTML>", "html", true, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ScanOpenTag([]byte(tt.buf), tt.tag, tt.foldCase, tt.stripPrefix); got != tt.want {
				t.Errorf("ScanOpenTag(%q, %q, fold=%v, strip=%v) = %v, want %v",
					tt.buf, tt.tag, tt.foldCase, tt.stripPrefix, got, tt.want)
			}
		})
	}
}
