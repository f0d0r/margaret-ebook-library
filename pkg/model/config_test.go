package model

import "testing"

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxCoverSize != 50*1024*1024 {
		t.Errorf("MaxCoverSize = %d, want %d", cfg.MaxCoverSize, 50*1024*1024)
	}
}
