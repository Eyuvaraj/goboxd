package registry

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/thesouldev/goboxd/internal/config"
)

// ProbeResult is the readiness check result for one language or nsjail.
type ProbeResult struct {
	OK      bool   `json:"ok"`
	Version string `json:"version,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ProbeNsjail checks that nsjailPath is executable and returns its version.
func ProbeNsjail(nsjailPath string) ProbeResult {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, nsjailPath, "--version").CombinedOutput()
	if err != nil {
		return ProbeResult{OK: false, Error: fmt.Sprintf("nsjail not found at %s: %v", nsjailPath, err)}
	}
	return ProbeResult{OK: true, Version: strings.TrimSpace(string(out))}
}

// ProbeLanguage runs `cmd --version` (or cmd if no version flag works) and
// returns whether the binary is present and usable.
func ProbeLanguage(lang *config.LanguageDef) ProbeResult {
	cmd := lang.Run.Cmd
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try --version first; fall back to running with no args (some interpreters exit 0).
	out, err := exec.CommandContext(ctx, cmd, "--version").CombinedOutput()
	if err != nil {
		// Some runtimes (e.g. java) write version to stderr and exit non-zero.
		// If we got output, treat that as success.
		combined := strings.TrimSpace(string(out))
		if combined == "" {
			return ProbeResult{
				OK:    false,
				Error: fmt.Sprintf("%s not found at %s: %v", lang.ID, cmd, err),
			}
		}
	}
	version := firstLine(strings.TrimSpace(string(out)))
	return ProbeResult{OK: true, Version: version}
}

func firstLine(s string) string {
	if idx := strings.Index(s, "\n"); idx >= 0 {
		return s[:idx]
	}
	return s
}
