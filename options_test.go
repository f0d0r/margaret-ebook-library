package ebook

import (
	"testing"

	"github.com/f0d0r/margaret-ebook-library/internal/config"
)

func TestResolveOptionsDefaults(t *testing.T) {
	cfg := resolveOptions(nil)
	want := config.DefaultConfig()

	if cfg != want {
		t.Errorf("resolveOptions(nil) = %+v, want %+v", cfg, want)
	}
}

func TestResolveOptionsOverrides(t *testing.T) {
	cfg := resolveOptions([]Option{
		WithMaxResourceSize(1),
		WithMaxRecordSize(2),
		WithMaxExthRecords(3),
	})

	if cfg.MaxResourceSize != 1 {
		t.Errorf("MaxResourceSize = %d, want 1", cfg.MaxResourceSize)
	}
	if cfg.MaxRecordSize != 2 {
		t.Errorf("MaxRecordSize = %d, want 2", cfg.MaxRecordSize)
	}
	if cfg.MaxExthRecords != 3 {
		t.Errorf("MaxExthRecords = %d, want 3", cfg.MaxExthRecords)
	}
}

func TestResolveOptionsPartialOverride(t *testing.T) {
	cfg := resolveOptions([]Option{WithMaxExthRecords(5)})
	d := config.DefaultConfig()

	if cfg.MaxResourceSize != d.MaxResourceSize {
		t.Errorf("MaxResourceSize = %d, want default %d", cfg.MaxResourceSize, d.MaxResourceSize)
	}
	if cfg.MaxRecordSize != d.MaxRecordSize {
		t.Errorf("MaxRecordSize = %d, want default %d", cfg.MaxRecordSize, d.MaxRecordSize)
	}
	if cfg.MaxExthRecords != 5 {
		t.Errorf("MaxExthRecords = %d, want 5", cfg.MaxExthRecords)
	}
}
