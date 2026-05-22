package mobi

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMOBISupports(t *testing.T) {
	tmpDir := t.TempDir()
	reader := &MobiReader{}

	validMOBIPath := filepath.Join(tmpDir, "valid.mobi")
	validData := make([]byte, 60)
	validData = append(validData, []byte("BOOKMOBI")...)
	if err := os.WriteFile(validMOBIPath, validData, 0644); err != nil {
		t.Fatalf("failed to write valid MOBI file: %v", err)
	}

	invalidMOBIPath := filepath.Join(tmpDir, "invalid.mobi")
	invalidData := []byte("BOOKMOBI" + string(make([]byte, 60)))
	if err := os.WriteFile(invalidMOBIPath, invalidData, 0644); err != nil {
		t.Fatalf("failed to write invalid MOBI file: %v", err)
	}

	shortPath := filepath.Join(tmpDir, "short.mobi")
	if err := os.WriteFile(shortPath, []byte("BOOK"), 0644); err != nil {
		t.Fatalf("failed to write short MOBI file: %v", err)
	}

	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{"Valid MOBI file", validMOBIPath, true},
		{"Identifier in wrong position", invalidMOBIPath, false},
		{"Too short file", shortPath, false},
		{"Non-existing file", filepath.Join(tmpDir, "missing.mobi"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reader.Supports(tt.filePath); got != tt.want {
				t.Errorf("Supports() = %v, want %v", got, tt.want)
			}
		})
	}
}
