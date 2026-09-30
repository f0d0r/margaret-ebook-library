package ebook

import (
	"testing"

	"github.com/f0d0r/margaret-ebook-library/internal/config"
)

func TestResolveOptionsDefaults(t *testing.T) {
	cfg := resolveOptions(nil)
	want := config.DefaultConfig()

	if cfg.MaxResourceSize != want.MaxResourceSize ||
		cfg.MaxRecordSize != want.MaxRecordSize ||
		cfg.MaxExthRecords != want.MaxExthRecords {
		t.Errorf("resolveOptions(nil) = %+v, want the default limits %+v", cfg, want)
	}
	if !cfg.KF8Fallback() {
		t.Error("KF8Fallback() = false, want the default true")
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

func TestResolveOptionsKF8Fallback(t *testing.T) {
	// Default (nil) should use library default (true)
	cfg := resolveOptions(nil)
	if !cfg.KF8Fallback() {
		t.Error("KF8Fallback() default = false, want true")
	}

	// Explicit true
	cfg = resolveOptions([]Option{WithKF8FallbackToMOBI6(true)})
	if !cfg.KF8Fallback() {
		t.Error("WithKF8FallbackToMOBI6(true) = false, want true")
	}

	// Explicit false (Calibre-like strict)
	cfg = resolveOptions([]Option{WithKF8FallbackToMOBI6(false)})
	if cfg.KF8Fallback() {
		t.Error("WithKF8FallbackToMOBI6(false) = true, want false")
	}
}

// TestZeroConfigKeepsDocumentedDefaults pins the behaviour of a hand-built
// config: a plain Config{} must behave like DefaultConfig instead of silently
// switching the KF8 fallback off.
func TestZeroConfigKeepsDocumentedDefaults(t *testing.T) {
	var zero config.Config
	if !zero.KF8Fallback() {
		t.Error("Config{}.KF8Fallback() = false, want the documented default true")
	}

	norm := zero.Normalize()
	d := config.DefaultConfig()
	if norm.MaxResourceSize != d.MaxResourceSize ||
		norm.MaxRecordSize != d.MaxRecordSize ||
		norm.MaxExthRecords != d.MaxExthRecords {
		t.Errorf("Config{}.Normalize() = %+v, want %+v", norm, d)
	}
}
