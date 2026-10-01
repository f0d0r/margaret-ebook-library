package lit

import "testing"

// TestHTMLMapConsistent locks the table shape the decoder relies on:
// tag ids index both tables, so they must stay aligned, and every
// referenced per-tag map must be usable (nil entries fall back to the
// global attribute map).
func TestHTMLMapConsistent(t *testing.T) {
	if len(htmlTagAttrs) != len(htmlTags) {
		t.Fatalf("len(htmlTagAttrs) = %d, len(htmlTags) = %d, want equal", len(htmlTagAttrs), len(htmlTags))
	}
	for i, attrs := range htmlTagAttrs {
		if attrs != nil && len(attrs) == 0 {
			t.Errorf("tag %d %q has an empty per-tag attribute map", i, htmlTags[i])
		}
	}
	if len(htmlAttrs) == 0 {
		t.Fatal("htmlAttrs is empty")
	}
	// Spot-check transcriptions against the reference tables.
	for tag, name := range map[int]string{3: "a", 53: "img", 74: "p", 107: "wbr"} {
		if htmlTags[tag] != name {
			t.Errorf("htmlTags[%d] = %q, want %q", tag, htmlTags[tag], name)
		}
	}
	if htmlTagAttrs[3][0x0001] != "href" {
		t.Errorf("tag 3 attr 0x0001 = %q, want href", htmlTagAttrs[3][0x0001])
	}
}
