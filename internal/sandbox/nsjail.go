package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/thesouldev/goboxd/internal/config"
)

const truncationMarker = "\n[output truncated]"

// RunConfig describes one nsjail invocation.
type RunConfig struct {
	NsjailPath    string
	WorkspaceDir  string
	Limits        config.LimitsDef
	// Cmd is the program to run inside the sandbox (e.g. "/usr/bin/python3").
	Cmd           string
	// Args are the arguments passed to Cmd after template expansion.
	Args          []string
	// Stdin is fed to the sandboxed process.
	Stdin         io.Reader
	// MaxOutputBytes caps captured stdout.
	MaxOutputBytes int64
	// BindMounts are additional read-only bind mounts: "host:container".
	BindMounts    []string
}

// RunResult holds the captured output of one nsjail invocation.
type RunResult struct {
	Stdout     []byte
	Stderr     []byte
	Log        []byte // nsjail's own diagnostic output (--log_fd 3)
	ExitCode   int
	DurationMs int64
	Truncated  bool
}

// Run executes cmd inside an nsjail sandbox and returns the result.
// It never uses a shell — argv is built as a pure []string.
func Run(ctx context.Context, cfg RunConfig) (RunResult, error) {
	argv := buildArgv(cfg)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)

	if cfg.Stdin != nil {
		cmd.Stdin = cfg.Stdin
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return RunResult{}, fmt.Errorf("stdout pipe: %w", err)
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	// Open a pipe for nsjail's own log output (--log_fd 3 in buildArgv).
	// ExtraFiles[0] becomes fd 3 in the child (after stdin/stdout/stderr).
	logR, logW, err := os.Pipe()
	if err != nil {
		return RunResult{}, fmt.Errorf("log pipe: %w", err)
	}
	cmd.ExtraFiles = []*os.File{logW}

	start := time.Now()
	if err := cmd.Start(); err != nil {
		logW.Close()
		logR.Close()
		return RunResult{}, fmt.Errorf("starting nsjail: %w", err)
	}
	// Close write end in parent so logR gets EOF when nsjail exits.
	logW.Close()

	// Drain nsjail log in a goroutine so a large log can't deadlock stdout.
	var logBuf bytes.Buffer
	logDone := make(chan struct{})
	go func() {
		defer close(logDone)
		_, _ = io.Copy(&logBuf, logR)
		logR.Close()
	}()

	// Read stdout with a hard cap (fixes hole #6: unbounded child output).
	limited := io.LimitReader(stdoutPipe, cfg.MaxOutputBytes+1)
	raw, _ := io.ReadAll(limited)

	truncated := false
	if int64(len(raw)) > cfg.MaxOutputBytes {
		raw = raw[:cfg.MaxOutputBytes]
		truncated = true
		// drain remaining stdout so the process isn't blocked on write
		_, _ = io.Copy(io.Discard, stdoutPipe)
	}

	<-logDone
	err = cmd.Wait()
	durationMs := time.Since(start).Milliseconds()

	exitCode := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		}
	}

	stdout := raw
	if truncated {
		stdout = append(stdout, []byte(truncationMarker)...)
	}

	return RunResult{
		Stdout:     stdout,
		Stderr:     stderrBuf.Bytes(),
		Log:        logBuf.Bytes(),
		ExitCode:   exitCode,
		DurationMs: durationMs,
		Truncated:  truncated,
	}, nil
}

// buildArgv constructs the nsjail command-line as a pure []string.
// No shell is involved at any point.
func buildArgv(cfg RunConfig) []string {
	argv := []string{cfg.NsjailPath}

	argv = append(argv,
		"--mode", "o",
		"--chroot", cfg.WorkspaceDir,
		"--user", "65534",
		"--group", "65534",
		"--log_fd", "3", // nsjail log → fd 3
		"--disable_clone_newnet",
		"--max_cpus", "1",
		"--rw",      // remount chroot r/w so compilers can write artifacts
		"--cwd", "/", // explicit working directory inside the jail
		// File-descriptor cap; Python and javac open many fds at startup.
		"--rlimit_nofile", "1000",
		// Minimal environment: programs must not inherit host secrets, but
		// need a sane TMP and PATH to locate their runtimes and temp space.
		"--env", "TMP=/",
		"--env", "TMPDIR=/",
		"--env", "HOME=/",
		"--env", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	)

	if cfg.Limits.WallTimeS > 0 {
		argv = append(argv, "--time_limit", fmt.Sprintf("%d", cfg.Limits.WallTimeS))
	}
	if cfg.Limits.MemoryKB > 0 {
		// RLIMIT_AS caps virtual address space, not RSS. Interpreters (Python, JVM)
		// mmap 150–300 MiB of virtual space at startup even for trivial programs, so
		// the limit must be significantly larger than memory_kb. Apply a 4× multiplier
		// with a 512 MiB floor so any interpreter can start while still bounding
		// runaway allocators.
		virtMiB := cfg.Limits.MemoryKB * 4 / 1024
		if virtMiB < 512 {
			virtMiB = 512
		}
		argv = append(argv, "--rlimit_as", fmt.Sprintf("%d", virtMiB))
	}
	if cfg.Limits.MaxProcesses > 0 {
		argv = append(argv, "--rlimit_nproc", fmt.Sprintf("%d", cfg.Limits.MaxProcesses))
	}

	// File size cap: 100 MB per created file (safety net).
	argv = append(argv, "--rlimit_fsize", "100")

	for _, bm := range cfg.BindMounts {
		parts := strings.SplitN(bm, ":", 2)
		if len(parts) == 2 {
			argv = append(argv, "-R", parts[0]+":"+parts[1])
		} else {
			argv = append(argv, "-R", bm)
		}
	}

	argv = append(argv, "--")
	argv = append(argv, cfg.Cmd)
	argv = append(argv, cfg.Args...)

	return argv
}

// ExpandArgs replaces {{source}}, {{artifact}}, and {{flags}} in a list of
// argument templates. Expansion is done per-element, never through a shell.
func ExpandArgs(tmplArgs []string, source, artifact string, flags []string) []string {
	expanded := make([]string, 0, len(tmplArgs)+len(flags))
	for _, a := range tmplArgs {
		switch a {
		case "{{flags}}":
			expanded = append(expanded, flags...)
		case "{{source}}":
			expanded = append(expanded, source)
		case "{{artifact}}":
			expanded = append(expanded, artifact)
		default:
			// Replace inline occurrences (e.g. "./{{artifact}}")
			a = strings.ReplaceAll(a, "{{source}}", source)
			a = strings.ReplaceAll(a, "{{artifact}}", artifact)
			expanded = append(expanded, a)
		}
	}
	return expanded
}
