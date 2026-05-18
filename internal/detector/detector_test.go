package detector

import (
    "testing"
)

func TestDetect(t *testing.T) {
    // Test that we get EPUB type for a valid epub file
    fileType := Detect("example.epub")
    if fileType != EPUB {
        t.Errorf("Expected EPUB, got %s", fileType)
    }
}
