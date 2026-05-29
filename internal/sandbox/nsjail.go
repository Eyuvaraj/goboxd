package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/thesouldev/goboxd/internal/config"
)

const truncationMarker = "\n[output truncated]"

// seccompPolicy is a Kafel deny-list applied via --seccomp_string.
// DEFAULT ALLOW keeps all language runtimes working without enumerating needed syscalls.
// KILL_PROCESS (not KILL) kills the whole sandboxed process on a violation — KILL only
// kills the calling thread, leaving other threads running.
//
// Denied categories and rationale:
//   - sandbox escape: ptrace, process_vm_readv/writev, chroot, pivot_root, unshare, setns
//   - kernel/module loading: init_module, finit_module, delete_module, kexec_load, bpf
//   - device creation: mknodat  (mknod is absent from ARM64 Kafel; mknodat suffices)
//   - clock manipulation: settimeofday, adjtimex, clock_adjtime
//   - privilege escalation: capset, userfaultfd, acct, mount
//   - file-handle bypass: name_to_handle_at, open_by_handle_at
//   - information leaks: syslog, fanotify_init, add_key, request_key, keyctl
//   - obvious: reboot
//
// Intentionally NOT denied: perf_event_open (JVM profiling), socket/connect/bind
// (already isolated by nsjail's network namespace).
// io_uring_* and kexec_file_load are absent from the ARM64 Kafel syscall table and
// are omitted to avoid "Undefined identifier" compile errors.
const seccompPolicy = `POLICY goboxd_safe {
    KILL_PROCESS {
        ptrace,
        process_vm_readv,
        process_vm_writev,
        init_module,
        finit_module,
        delete_module,
        kexec_load,
        reboot,
        settimeofday,
        adjtimex,
        clock_adjtime,
        mknodat,
        chroot,
        pivot_root,
        unshare,
        setns,
        userfaultfd,
        name_to_handle_at,
        open_by_handle_at,
        acct,
        bpf,
        syslog,
        add_key,
        request_key,
        keyctl,
        fanotify_init,
        capset,
        mount
    }
}
USE goboxd_safe DEFAULT ALLOW`

type RunConfig struct {
	NsjailPath     string
	WorkspaceDir   string
	Limits         config.LimitsDef
	Cmd            string
	Args           []string
	Stdin          io.Reader
	MaxOutputBytes int64
	BindMounts     []string // read-only host paths to bind-mount into the jail
	Env            []string // extra KEY=VALUE vars for this invocation
	CgroupParent   string   // cgroup v2 directory name for this run
}

type RunResult struct {
	Stdout       []byte
	Stderr       []byte
	Log          []byte // nsjail diagnostic output from --log_fd 3
	ExitCode     int
	DurationMs   int64
	Truncated    bool
	MemoryPeakKB int64
}

// Run executes cmd inside an nsjail sandbox. argv is a pure []string — no shell.
func Run(ctx context.Context, cfg RunConfig) (RunResult, error) {
	cfg.CgroupParent = "goboxd-" + strconv.FormatInt(time.Now().UnixNano(), 10)

	cgroupPath := filepath.Join("/sys/fs/cgroup", cfg.CgroupParent)
	_ = os.Mkdir(cgroupPath, 0o755)
	defer func() {
		// If nsjail was killed before cleaning up its own child cgroup, the parent
		// dir is non-empty and os.Remove would fail. Remove sub-dirs first.
		if entries, err := os.ReadDir(cgroupPath); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					_ = os.Remove(filepath.Join(cgroupPath, e.Name()))
				}
			}
		}
		_ = os.Remove(cgroupPath)
	}()

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

	// ExtraFiles[0] becomes fd 3 in the child (nsjail --log_fd 3).
	logR, logW, err := os.Pipe()
	if err != nil {
		return RunResult{}, fmt.Errorf("log pipe: %w", err)
	}
	cmd.ExtraFiles = []*os.File{logW}

	start := time.Now()
	if err := cmd.Start(); err != nil {
		_ = logW.Close()
		_ = logR.Close()
		return RunResult{}, fmt.Errorf("starting nsjail: %w", err)
	}
	_ = logW.Close() // parent closes write end so logR gets EOF when nsjail exits

	// Drain nsjail log in a goroutine so it can't deadlock stdout.
	var logBuf bytes.Buffer
	logDone := make(chan struct{})
	go func() {
		defer close(logDone)
		_, _ = io.Copy(&logBuf, logR)
		_ = logR.Close()
	}()

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
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exitCode = ee.ExitCode()
		}
	}

	stdout := raw
	if truncated {
		stdout = append(stdout, []byte(truncationMarker)...)
	}

	var peakKB int64
	if peakBytes, err := os.ReadFile(filepath.Join(cgroupPath, "memory.peak")); err == nil {
		if peak, err := strconv.ParseInt(string(bytes.TrimSpace(peakBytes)), 10, 64); err == nil {
			peakKB = peak / 1024
		}
	}

	return RunResult{
		Stdout:       stdout,
		Stderr:       stderrBuf.Bytes(),
		Log:          logBuf.Bytes(),
		ExitCode:     exitCode,
		DurationMs:   durationMs,
		Truncated:    truncated,
		MemoryPeakKB: peakKB,
	}, nil
}

// buildArgv constructs the nsjail command-line as a pure []string (no shell).
func buildArgv(cfg RunConfig) []string {
	argv := []string{cfg.NsjailPath}

	argv = append(argv,
		"--mode", "o",
		"--chroot", cfg.WorkspaceDir,
		"--user", "65534",
		"--group", "65534",
		"--log_fd", "3", // nsjail diagnostic log → fd 3
		"--max_cpus", "1",
		"--rw", // remount chroot r/w so compilers can write artifacts
		"--cwd", "/",
		"--hostname", "goboxd", // prevents host hostname leaking via gethostname(2)
		"--detect_cgroupv2",
		"--cgroupv2_mount", "/sys/fs/cgroup",
		"--cgroup_mem_parent", cfg.CgroupParent,
		"--rlimit_nofile", "1000", // Python and javac open many fds at startup
		"--rlimit_core", "0", // no core dumps — saves disk and prevents source leakage
		// Hard stack ceiling: container environments sometimes inherit unlimited rlimit_stack.
		"--rlimit_stack", "8",
		"--env", "TMP=/",
		"--env", "TMPDIR=/",
		"--env", "HOME=/",
		"--env", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	)

	for _, e := range cfg.Env {
		argv = append(argv, "--env", e)
	}

	if cfg.Limits.WallTimeS > 0 {
		argv = append(argv, "--time_limit", strconv.Itoa(cfg.Limits.WallTimeS))
		// rlimit_cpu is set one second above --time_limit so nsjail's wall-time check
		// always fires first. Equal values cause SIGXCPU before nsjail's poll loop wakes,
		// making the status parse as runtime_error instead of time_exceeded.
		argv = append(argv, "--rlimit_cpu", strconv.Itoa(cfg.Limits.WallTimeS+1))
	}
	if cfg.Limits.MemoryKB > 0 {
		cgroupMemBytes := int64(cfg.Limits.MemoryKB) * 1024
		argv = append(argv, "--cgroup_mem_max", strconv.FormatInt(cgroupMemBytes, 10))
		argv = append(argv, "--cgroup_mem_swap_max", "0") // disable swap so OOM fires at exactly memory.max
		// RLIMIT_AS caps virtual space separately from RSS. The JVM pre-allocates
		// ~1 GiB of virtual space on ARM64, so floor at 4096 MiB.
		virtMiB := cfg.Limits.MemoryKB * 4 / 1024
		if virtMiB < 4096 {
			virtMiB = 4096
		}
		argv = append(argv, "--rlimit_as", strconv.Itoa(virtMiB))
	}
	if cfg.Limits.MaxProcesses > 0 {
		argv = append(argv, "--cgroup_pids_max", strconv.Itoa(cfg.Limits.MaxProcesses))
		argv = append(argv, "--rlimit_nproc", strconv.Itoa(cfg.Limits.MaxProcesses))
	}

	argv = append(argv, "--rlimit_fsize", "100") // 100 MB per-file cap

	for _, bm := range cfg.BindMounts {
		argv = append(argv, "-R", bm)
	}

	argv = append(argv, "--seccomp_string", seccompPolicy)
	argv = append(argv, "--")
	argv = append(argv, cfg.Cmd)
	argv = append(argv, cfg.Args...)

	return argv
}

// ExpandArgs replaces {{source}}, {{artifact}}, and {{flags}} template tokens per-element.
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
