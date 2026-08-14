package model

import "testing"

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxCoverSize != 50*1024*1024 {
		t.Errorf("MaxCoverSize = %d, want %d", cfg.MaxCoverSize, 50*1024*1024)
	}
	if cfg.MaxRecordSize != 100*1024*1024 {
		t.Errorf("MaxRecordSize = %d, want %d", cfg.MaxRecordSize, 100*1024*1024)
	}
	if cfg.MaxExthRecords != 256 {
		t.Errorf("MaxExthRecords = %d, want %d", cfg.MaxExthRecords, 256)
	}
}
