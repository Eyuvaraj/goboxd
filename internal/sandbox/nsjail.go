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
	"sync/atomic"
	"time"

	"github.com/thesouldev/goboxd/internal/config"
)

const truncationMarker = "\n[output truncated]"

// runCounter provides collision-free cgroup names and sandbox UIDs across concurrent runs.
var runCounter atomic.Uint64

// Sandbox UIDs are drawn from [uidBase, uidBase+uidPoolSize).
const (
	uidBase     = 60000
	uidPoolSize = 1024
)

// seccompPolicy is a Kafel deny-list passed to nsjail via --seccomp_string.
// DEFAULT ALLOW keeps all runtimes working. KILL_PROCESS terminates the whole
// process group on a violation. See docs/security.md for the full rationale.
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
	SandboxUID     string   // per-run uid/gid inside the jail; set by Run(), not callers
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

// Run executes cmd inside an nsjail sandbox.
func Run(ctx context.Context, cfg RunConfig) (RunResult, error) {
	// Monotonic counter gives each run a unique cgroup name and sandbox UID.
	runID := runCounter.Add(1)
	cfg.CgroupParent = "goboxd-" + strconv.FormatUint(runID, 10)
	cfg.SandboxUID = strconv.FormatUint(uidBase+runID%uidPoolSize, 10)

	cgroupPath := filepath.Join("/sys/fs/cgroup", cfg.CgroupParent)
	_ = os.Mkdir(cgroupPath, 0o755)
	defer func() {
		// nsjail may leave a child cgroup behind; clean sub-dirs first.
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

	// ExtraFiles[0] becomes fd 3 inside nsjail (--log_fd 3).
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

	// Drain nsjail's log fd in a goroutine so it never blocks stdout reads.
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
		_, _ = io.Copy(io.Discard, stdoutPipe) // drain so the process isn't blocked
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
		"--user", cfg.SandboxUID,
		"--group", cfg.SandboxUID,
		"--log_fd", "3",
		"--max_cpus", "1",
		"--rw",
		// CLONE_NEWPID is on by default in nsjail 3.4; do not pass --disable_clone_newpid.
		"--cwd", "/",
		"--hostname", "goboxd",
		"--detect_cgroupv2",
		"--cgroupv2_mount", "/sys/fs/cgroup",
		"--cgroup_mem_parent", cfg.CgroupParent,
		"--rlimit_nofile", "1000", // javac and Python open many fds at startup
		"--rlimit_core", "0",
		"--rlimit_stack", "8", // container envs can inherit unlimited stack
		"--env", "TMP=/",
		"--env", "TMPDIR=/",
		"--env", "HOME=/",
		"--env", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"--iface_no_lo",
	)

	for _, e := range cfg.Env {
		argv = append(argv, "--env", e)
	}

	if cfg.Limits.WallTimeS > 0 {
		argv = append(argv, "--time_limit", strconv.Itoa(cfg.Limits.WallTimeS))
		// rlimit_cpu is 1s above --time_limit so nsjail's wall-time check fires first.
		// Equal values let SIGXCPU arrive before nsjail polls, misclassifying it as runtime_error.
		argv = append(argv, "--rlimit_cpu", strconv.Itoa(cfg.Limits.WallTimeS+1))
	}
	if cfg.Limits.MemoryKB > 0 {
		cgroupMemBytes := int64(cfg.Limits.MemoryKB) * 1024
		argv = append(argv, "--cgroup_mem_max", strconv.FormatInt(cgroupMemBytes, 10))
		argv = append(argv, "--cgroup_mem_swap_max", "0")
		// RLIMIT_AS must be large enough for the JVM (~1 GiB virtual on ARM64).
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

	argv = append(argv, "--rlimit_fsize", "100") // 100 MB per-file write cap

	for _, bm := range cfg.BindMounts {
		argv = append(argv, "-R", bm)
	}

	argv = append(argv, "--seccomp_string", seccompPolicy)
	argv = append(argv, "--")
	argv = append(argv, cfg.Cmd)
	argv = append(argv, cfg.Args...)

	return argv
}

// ExpandArgs replaces {{source}}, {{artifact}}, and {{flags}} tokens in a YAML args list.
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
			// Handle inline occurrences like "./{{artifact}}".
			a = strings.ReplaceAll(a, "{{source}}", source)
			a = strings.ReplaceAll(a, "{{artifact}}", artifact)
			expanded = append(expanded, a)
		}
	}
	return expanded
}
