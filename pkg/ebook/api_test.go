package ebook

import (
	"testing"

	"github.com/f0d0r/margaret-ebook-library/pkg/errs"
)

func TestReadMetadata_EmptyPath(t *testing.T) {
	_, err := ReadMetadata("")
	if err != errs.ErrUnsupportedFormat {
		t.Fatalf("expected ErrUnsupportedFormat for empty path, got %v", err)
	}
}

func TestReadMetadata_WhitespacePath(t *testing.T) {
	_, err := ReadMetadata("   \t\n  ")
	if err != errs.ErrUnsupportedFormat {
		t.Fatalf("expected ErrUnsupportedFormat for whitespace-only path, got %v", err)
	}
}
