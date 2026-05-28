package registry

import (
	"context"
	"fmt"
	"os"
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

// ProbeNsjail checks that nsjailPath is executable.
// nsjail has no --version flag (it exits 255 for unknown flags), so we
// verify the binary exists and is executable via os.Stat, then confirm
// it can at least be launched (--help exits 255 but doesn't fail to exec).
func ProbeNsjail(nsjailPath string) ProbeResult {
	info, err := os.Stat(nsjailPath)
	if err != nil {
		return ProbeResult{OK: false, Error: fmt.Sprintf("nsjail not found at %s: %v", nsjailPath, err)}
	}
	if info.Mode()&0o111 == 0 {
		return ProbeResult{OK: false, Error: fmt.Sprintf("nsjail at %s is not executable", nsjailPath)}
	}

	// Run with --help: nsjail exits 255 but produces output and doesn't fail to exec.
	// We only care that the binary can be launched; exec.ExitError means it ran fine.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, execErr := exec.CommandContext(ctx, nsjailPath, "--help").CombinedOutput()
	if execErr != nil {
		if _, ok := execErr.(*exec.ExitError); !ok {
			// Real exec failure (e.g. permission denied, ELF not found).
			return ProbeResult{OK: false, Error: fmt.Sprintf("nsjail not found at %s: %v", nsjailPath, execErr)}
		}
	}
	return ProbeResult{OK: true, Version: "ok"}
}

// ProbeLanguage runs `cmd --version` (or cmd if no version flag works) and
// returns whether the binary is present and usable.
// For compiled languages the run.cmd is the per-job artifact path (e.g. /solution)
// which doesn't exist at probe time, so we probe the build.cmd (the compiler) instead.
func ProbeLanguage(lang *config.LanguageDef) ProbeResult {
	// For compiled languages, run.cmd is the output artifact path — not a real binary.
	// Probe the compiler (build.cmd) instead.
	cmd := lang.Run.Cmd
	if lang.IsCompiled() && lang.Build != nil && lang.Build.Cmd != "" {
		cmd = lang.Build.Cmd
	}

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
