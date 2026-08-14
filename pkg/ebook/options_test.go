package ebook

import (
	"testing"

	"github.com/f0d0r/margaret-ebook-library/pkg/model"
)

func TestResolveOptionsDefaults(t *testing.T) {
	cfg := resolveOptions(nil)
	want := model.DefaultConfig()

	if cfg != want {
		t.Errorf("resolveOptions(nil) = %+v, want %+v", cfg, want)
	}
}

func TestResolveOptionsOverrides(t *testing.T) {
	cfg := resolveOptions([]Option{
		WithMaxCoverSize(1),
		WithMaxRecordSize(2),
		WithMaxExthRecords(3),
	})

	if cfg.MaxCoverSize != 1 {
		t.Errorf("MaxCoverSize = %d, want 1", cfg.MaxCoverSize)
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
	d := model.DefaultConfig()

	if cfg.MaxCoverSize != d.MaxCoverSize {
		t.Errorf("MaxCoverSize = %d, want default %d", cfg.MaxCoverSize, d.MaxCoverSize)
	}
	if cfg.MaxRecordSize != d.MaxRecordSize {
		t.Errorf("MaxRecordSize = %d, want default %d", cfg.MaxRecordSize, d.MaxRecordSize)
	}
	if cfg.MaxExthRecords != 5 {
		t.Errorf("MaxExthRecords = %d, want 5", cfg.MaxExthRecords)
	}
}
