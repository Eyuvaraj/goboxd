package validate_test

import (
	"testing"

	"github.com/thesouldev/goboxd/internal/config"
	"github.com/thesouldev/goboxd/internal/validate"
)

func TestFilename(t *testing.T) {
	valid := []string{
		"solution.cpp", "Main.java", "test.py", "a", "file_name-1.go",
	}
	for _, name := range valid {
		if err := validate.Filename(name); err != nil {
			t.Errorf("Filename(%q) unexpected error: %v", name, err)
		}
	}

	type badCase struct {
		name string
		s    string
	}
	bad := []badCase{
		{"empty", ""},
		{"absolute", "/etc/passwd"},
		{"traversal", "../../etc/passwd"},
		{"with slash", "foo/bar"},
		{"leading dot", ".bashrc"},
		{"leading dot2", ".hidden"},
		{"too long", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, // 66 chars
		{"null byte", "foo\x00bar"},
		{"space", "solution .py"},
		{"exclamation", "file!.py"},
		{"at sign", "file@.py"},
	}
	for _, tc := range bad {
		if err := validate.Filename(tc.s); err == nil {
			t.Errorf("Filename(%q) [%s]: expected error, got nil", tc.s, tc.name)
		}
	}
}

func TestFlags(t *testing.T) {
	allowlist := []string{"-O0", "-O1", "-O2", "-O3", "-Wall", "-Wextra", "-std=*"}

	if err := validate.Flags([]string{"-O2", "-Wall", "-std=c++17"}, allowlist); err != nil {
		t.Errorf("unexpected error for valid flags: %v", err)
	}
	if err := validate.Flags([]string{}, allowlist); err != nil {
		t.Errorf("unexpected error for empty flags: %v", err)
	}

	bad := []string{"-fplugin=evil.so", "-x", "c", "--specs=/tmp/bad", "@response_file", "-Wl,-rpath,/tmp"}
	for _, f := range bad {
		if err := validate.Flags([]string{f}, allowlist); err == nil {
			t.Errorf("Flags([%q]): expected error, got nil", f)
		}
	}
}

func TestSourceSize(t *testing.T) {
	if err := validate.SourceSize("hello", 10); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := validate.SourceSize("hello world!", 5); err == nil {
		t.Error("expected error for oversized source, got nil")
	}
}

func TestTestCount(t *testing.T) {
	if err := validate.TestCount(0, 50); err == nil {
		t.Error("expected error for 0 tests (/run requires at least one)")
	}
	if err := validate.TestCount(1, 50); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := validate.TestCount(51, 50); err == nil {
		t.Error("expected error for exceeding max")
	}
}

func TestStdinSize(t *testing.T) {
	if err := validate.StdinSize("hello", 10); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := validate.StdinSize("", 10); err != nil {
		t.Errorf("empty stdin should be valid: %v", err)
	}
	if err := validate.StdinSize("hello world!", 5); err == nil {
		t.Error("expected error for oversized stdin, got nil")
	}
}

func TestExpectedSize(t *testing.T) {
	if err := validate.ExpectedSize("hello\n", 10); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := validate.ExpectedSize("", 10); err != nil {
		t.Errorf("empty expected_stdout should be valid: %v", err)
	}
	if err := validate.ExpectedSize("hello world!", 5); err == nil {
		t.Error("expected error for oversized expected_stdout, got nil")
	}
}

func TestLimits(t *testing.T) {
	base := config.LimitsDef{WallTimeS: 10, MemoryKB: 102400, MaxProcesses: 100}

	// Zero override leaves base unchanged — always valid.
	if err := validate.Limits(config.LimitsDef{}, base); err != nil {
		t.Errorf("zero override should be valid: %v", err)
	}

	// Override matching base exactly is valid.
	if err := validate.Limits(base, base); err != nil {
		t.Errorf("equal limits should be valid: %v", err)
	}

	// Each field independently triggering an error.
	if err := validate.Limits(config.LimitsDef{WallTimeS: 11}, base); err == nil {
		t.Error("expected error: wall_time_s exceeds max")
	}
	if err := validate.Limits(config.LimitsDef{MemoryKB: 102401}, base); err == nil {
		t.Error("expected error: memory_kb exceeds max")
	}
	if err := validate.Limits(config.LimitsDef{MaxProcesses: 101}, base); err == nil {
		t.Error("expected error: max_processes exceeds max")
	}

	// All fields under the limit is fine.
	if err := validate.Limits(config.LimitsDef{WallTimeS: 5, MemoryKB: 1024, MaxProcesses: 10}, base); err != nil {
		t.Errorf("under-limit override should be valid: %v", err)
	}
}
