package lit

import "testing"

// TestHTMLMapConsistent locks the table shape the decoder relies on:
// tag ids index both tables, so they must stay aligned.
func TestHTMLMapConsistent(t *testing.T) {
	if len(htmlTagAttrs) != len(htmlTags) {
		t.Fatalf("len(htmlTagAttrs) = %d, len(htmlTags) = %d, want equal", len(htmlTagAttrs), len(htmlTags))
	}
	for i, name := range htmlTags {
		if name == "" {
			continue
		}
		if htmlTagAttrs[i] == nil {
			t.Logf("note: tag %d %q has no per-tag attributes", i, name)
		}
	}
	if len(htmlAttrs) == 0 {
		t.Fatal("htmlAttrs is empty")
	}
}
