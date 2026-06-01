package registry

import (
	"context"
	"errors"
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

// ProbeNsjail checks that nsjailPath exists and can be launched.
// nsjail has no --version flag; --help exits 255 but doesn't fail to exec,
// so an exec.ExitError from --help means the binary is functional. The version
// string is not discoverable from the binary, so it is reported from the
// NSJAIL_VERSION env var the image sets at build time (falling back to "ok").
func ProbeNsjail(nsjailPath string) ProbeResult {
	info, err := os.Stat(nsjailPath)
	if err != nil {
		return ProbeResult{OK: false, Error: fmt.Sprintf("nsjail not found at %s: %v", nsjailPath, err)}
	}
	if info.Mode()&0o111 == 0 {
		return ProbeResult{OK: false, Error: fmt.Sprintf("nsjail at %s is not executable", nsjailPath)}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, execErr := exec.CommandContext(ctx, nsjailPath, "--help").CombinedOutput()
	var exitErr *exec.ExitError
	if execErr != nil && !errors.As(execErr, &exitErr) {
		return ProbeResult{OK: false, Error: fmt.Sprintf("nsjail not found at %s: %v", nsjailPath, execErr)}
	}
	version := os.Getenv("NSJAIL_VERSION")
	if version == "" {
		version = "ok"
	}
	return ProbeResult{OK: true, Version: version}
}

// ProbeLanguage runs a version check to confirm the binary is present and executable.
// For compiled languages, run.cmd is the per-job artifact path (e.g. /solution),
// so we probe build.cmd (the compiler) instead.
// probe_args in the YAML overrides the default ["--version"] for runtimes that use
// a different flag (e.g. lua5.4 uses -v, iverilog uses -V).
// Some runtimes (e.g. javac) write version to stderr and exit non-zero — if we
// got any output at all, we treat the probe as successful.
func ProbeLanguage(lang *config.LanguageDef) ProbeResult {
	cmd := lang.Run.Cmd
	if lang.IsCompiled() && lang.Build != nil && lang.Build.Cmd != "" {
		cmd = lang.Build.Cmd
	}

	probeArgs := []string{"--version"}
	if len(lang.ProbeArgs) > 0 {
		probeArgs = lang.ProbeArgs
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, cmd, probeArgs...).CombinedOutput()
	if err != nil {
		if strings.TrimSpace(string(out)) == "" {
			return ProbeResult{OK: false, Error: fmt.Sprintf("%s not found at %s: %v", lang.ID, cmd, err)}
		}
	}
	return ProbeResult{OK: true, Version: firstLine(strings.TrimSpace(string(out)))}
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
