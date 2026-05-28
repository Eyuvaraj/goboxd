package config_test

import (
	"testing"

	"github.com/thesouldev/goboxd/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear any env vars that might be set in the test environment.
	for _, k := range []string{
		"PORT", "NSJAIL_PATH", "JAIL_DIR", "LANGUAGE_FILE",
		"MAX_SOURCE_BYTES", "MAX_TESTS", "MAX_CONCURRENT_JOBS",
		"MAX_OUTPUT_BYTES", "MAX_STDIN_BYTES", "ORPHAN_MAX_AGE_MINUTES",
	} {
		t.Setenv(k, "")
	}

	cfg := config.Load()

	if cfg.Port != "8080" {
		t.Errorf("Port default: want %q, got %q", "8080", cfg.Port)
	}
	if cfg.NsjailPath != "/usr/local/bin/nsjail" {
		t.Errorf("NsjailPath default: want %q, got %q", "/usr/local/bin/nsjail", cfg.NsjailPath)
	}
	if cfg.MaxSourceBytes != 256*1024 {
		t.Errorf("MaxSourceBytes default: want %d, got %d", 256*1024, cfg.MaxSourceBytes)
	}
	if cfg.MaxTests != 50 {
		t.Errorf("MaxTests default: want %d, got %d", 50, cfg.MaxTests)
	}
	if cfg.MaxStdinBytes != 64*1024 {
		t.Errorf("MaxStdinBytes default: want %d, got %d", 64*1024, cfg.MaxStdinBytes)
	}
	if cfg.MaxOutputBytes != 256*1024 {
		t.Errorf("MaxOutputBytes default: want %d, got %d", 256*1024, cfg.MaxOutputBytes)
	}
	if cfg.MaxConcurrentJobs <= 0 {
		t.Errorf("MaxConcurrentJobs default: want > 0, got %d", cfg.MaxConcurrentJobs)
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("NSJAIL_PATH", "/opt/nsjail")
	t.Setenv("MAX_SOURCE_BYTES", "131072")
	t.Setenv("MAX_TESTS", "10")
	t.Setenv("MAX_CONCURRENT_JOBS", "4")
	t.Setenv("MAX_STDIN_BYTES", "1024")
	t.Setenv("MAX_OUTPUT_BYTES", "8192")
	t.Setenv("ORPHAN_MAX_AGE_MINUTES", "15")

	cfg := config.Load()

	if cfg.Port != "9090" {
		t.Errorf("Port: want %q, got %q", "9090", cfg.Port)
	}
	if cfg.NsjailPath != "/opt/nsjail" {
		t.Errorf("NsjailPath: want %q, got %q", "/opt/nsjail", cfg.NsjailPath)
	}
	if cfg.MaxSourceBytes != 131072 {
		t.Errorf("MaxSourceBytes: want %d, got %d", 131072, cfg.MaxSourceBytes)
	}
	if cfg.MaxTests != 10 {
		t.Errorf("MaxTests: want %d, got %d", 10, cfg.MaxTests)
	}
	if cfg.MaxConcurrentJobs != 4 {
		t.Errorf("MaxConcurrentJobs: want %d, got %d", 4, cfg.MaxConcurrentJobs)
	}
	if cfg.MaxStdinBytes != 1024 {
		t.Errorf("MaxStdinBytes: want %d, got %d", 1024, cfg.MaxStdinBytes)
	}
	if cfg.MaxOutputBytes != 8192 {
		t.Errorf("MaxOutputBytes: want %d, got %d", 8192, cfg.MaxOutputBytes)
	}
	if cfg.OrphanMaxAge != 15 {
		t.Errorf("OrphanMaxAge: want %d, got %d", 15, cfg.OrphanMaxAge)
	}
}

func TestLoad_InvalidIntFallsToDefault(t *testing.T) {
	t.Setenv("MAX_SOURCE_BYTES", "not-a-number")
	t.Setenv("MAX_TESTS", "-5")          // non-positive: rejected
	t.Setenv("MAX_CONCURRENT_JOBS", "0") // zero: rejected

	cfg := config.Load()

	if cfg.MaxSourceBytes != 256*1024 {
		t.Errorf("MaxSourceBytes: invalid env should use default, got %d", cfg.MaxSourceBytes)
	}
	if cfg.MaxTests != 50 {
		t.Errorf("MaxTests: non-positive env should use default, got %d", cfg.MaxTests)
	}
	if cfg.MaxConcurrentJobs <= 0 {
		t.Errorf("MaxConcurrentJobs: zero env should use default (NumCPU), got %d", cfg.MaxConcurrentJobs)
	}
}
