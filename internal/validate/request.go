package validate

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/thesouldev/goboxd/internal/config"
)

const (
	MaxFilenameLen   = 64
	MaxSourceBytes   = 256 * 1024 // 256 KiB
	MaxStdinBytes    = 64 * 1024  // 64 KiB
	MaxExpectedBytes = 64 * 1024  // 64 KiB
	MaxTests         = 50
	MaxOutputBytes   = 256 * 1024 // 256 KiB captured stdout/stderr per run
)

var ErrFilenameEmpty = errors.New("filename must not be empty")
var ErrFilenameAbsolute = errors.New("filename must not be an absolute path")
var ErrFilenameSeparator = errors.New("filename must be a single path component with no separators")
var ErrFilenameLeadingDot = errors.New("filename must not start with a dot")
var ErrFilenameTooLong = errors.New("filename exceeds maximum length")
var ErrFilenameInvalidChar = errors.New("filename contains invalid characters")

// Filename validates that s is safe to use as a filename inside a sandbox
// directory. Rules: non-empty, not absolute, single path component (no /),
// no leading dot, printable ASCII only, length ≤ MaxFilenameLen.
func Filename(s string) error {
	if s == "" {
		return ErrFilenameEmpty
	}
	if filepath.IsAbs(s) {
		return ErrFilenameAbsolute
	}
	if strings.ContainsAny(s, "/\\") {
		return ErrFilenameSeparator
	}
	if filepath.Base(s) != s {
		return ErrFilenameSeparator
	}
	if strings.HasPrefix(s, ".") {
		return ErrFilenameLeadingDot
	}
	if len(s) > MaxFilenameLen {
		return ErrFilenameTooLong
	}
	for _, r := range s {
		if !unicode.IsPrint(r) || r > 127 {
			return ErrFilenameInvalidChar
		}
	}
	return nil
}

// Flags validates that every flag in flags appears in allowlist.
// An allowlist entry ending in "*" is treated as a prefix match
// (e.g. "-std=*" matches "-std=c++17").
func Flags(flags []string, allowlist []string) error {
	for _, f := range flags {
		if !flagAllowed(f, allowlist) {
			return fmt.Errorf("flag %q is not in the allowlist for this language", f)
		}
	}
	return nil
}

func flagAllowed(flag string, allowlist []string) bool {
	for _, a := range allowlist {
		if strings.HasSuffix(a, "*") {
			if strings.HasPrefix(flag, a[:len(a)-1]) {
				return true
			}
		} else if flag == a {
			return true
		}
	}
	return false
}

// SourceSize returns an error if the source string exceeds max bytes.
func SourceSize(src string, max int) error {
	if len(src) > max {
		return fmt.Errorf("source size %d bytes exceeds maximum of %d bytes", len(src), max)
	}
	return nil
}

// TestCount returns an error if n exceeds max.
func TestCount(n, max int) error {
	if n == 0 {
		return errors.New("at least one test case is required")
	}
	if n > max {
		return fmt.Errorf("test count %d exceeds maximum of %d", n, max)
	}
	return nil
}

// StdinSize returns an error if the stdin string exceeds max bytes.
func StdinSize(s string, max int) error {
	if len(s) > max {
		return fmt.Errorf("stdin size %d bytes exceeds maximum of %d bytes", len(s), max)
	}
	return nil
}

// ExpectedSize returns an error if the expected_stdout string exceeds max bytes.
func ExpectedSize(s string, max int) error {
	if len(s) > max {
		return fmt.Errorf("expected_stdout size %d bytes exceeds maximum of %d bytes", len(s), max)
	}
	return nil
}

// Limits validates that client-supplied limit overrides do not exceed the language defaults.
// Clients may not grant themselves more resources than the server has configured for the language.
// Zero values in override are ignored (they mean "use the default").
func Limits(override, langDefault config.LimitsDef) error {
	if override.WallTimeS > 0 && override.WallTimeS > langDefault.WallTimeS {
		return fmt.Errorf("wall_time_s %d exceeds language maximum of %d", override.WallTimeS, langDefault.WallTimeS)
	}
	if override.MemoryKB > 0 && override.MemoryKB > langDefault.MemoryKB {
		return fmt.Errorf("memory_kb %d exceeds language maximum of %d", override.MemoryKB, langDefault.MemoryKB)
	}
	if override.MaxProcesses > 0 && override.MaxProcesses > langDefault.MaxProcesses {
		return fmt.Errorf("max_processes %d exceeds language maximum of %d", override.MaxProcesses, langDefault.MaxProcesses)
	}
	return nil
}
