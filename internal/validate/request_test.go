package validate_test

import (
	"testing"

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
		t.Error("expected error for 0 tests")
	}
	if err := validate.TestCount(1, 50); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := validate.TestCount(51, 50); err == nil {
		t.Error("expected error for exceeding max")
	}
}
