package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/thesouldev/goboxd/internal/config"
)

const truncationMarker = "\n[output truncated]"

// seccompPolicy is a Kafel deny-list passed to nsjail via --seccomp_string.
// DEFAULT ALLOW keeps all 13 registered language runtimes working without
// enumerating required syscalls.
//
// KILL_PROCESS (not KILL) terminates the entire sandboxed process when a
// forbidden syscall is made. KILL only kills the calling thread — in a
// multi-threaded program other threads would keep running.
//
// Denied syscalls and their attack surface:
//
//	ptrace / process_vm_readv / process_vm_writev
//	  Cross-process memory inspection and writes — sandbox escape primitives.
//	init_module / finit_module / delete_module
//	  Kernel module loading — arbitrary kernel code execution.
//	kexec_load / kexec_file_load
//	  Replace the running kernel image.
//	reboot
//	  Obvious.
//	settimeofday / adjtimex / clock_adjtime
//	  Clock skew — can affect timeout logic and log timestamps on the host.
//	mknodat
//	  Create device nodes; combined with chroot this enables device escapes.
//	  (mknod is absent from the ARM64 Kafel table; mknodat alone suffices.)
//	chroot / pivot_root
//	  Change filesystem root — the sandbox already has its root set by nsjail;
//	  a second chroot inside the jail could escape our bind-mount restrictions.
//	unshare / setns
//	  Manipulate Linux namespaces — could un-isolate the network, PID, or
//	  mount namespace that nsjail established.
//	io_uring_setup / io_uring_enter / io_uring_register
//	  Async I/O interface with a history of privilege-escalation CVEs; none
//	  of the registered language runtimes require it. Omitted from the Kafel
//	  policy on ARM64 (not in the aarch64 syscall table) — a syscall absent
//	  from the kernel cannot be invoked, so no deny rule is needed there.
//	userfaultfd
//	  Pause kernel page-fault handling from userspace — used in many
//	  kernel exploit chains; no legitimate use in a code sandbox.
//	name_to_handle_at / open_by_handle_at
//	  File-handle syscalls that can bypass directory-traversal checks and
//	  cross mount-point boundaries when combined with a leaked handle.
//	acct
//	  Enable/disable process accounting — unneeded and can interfere with
//	  host-side resource bookkeeping.
//	bpf
//	  Load eBPF programs into the kernel — kernel-level arbitrary code.
//	syslog
//	  Read the kernel ring buffer — information leak.
//	add_key / request_key / keyctl
//	  Kernel keyring operations — do not require elevated privileges; can be
//	  used to persist data across sandbox invocations via the session keyring.
//	fanotify_init
//	  Filesystem access notification — leaks path information about files
//	  accessed inside the jail; requires CAP_SYS_ADMIN on newer kernels but
//	  deny regardless for defence in depth.
//	capset
//	  Modify the process capability set — defence in depth against privilege
//	  re-escalation if a bug ever leaves capabilities in the bounding set.
//	mount
//	  Mount filesystems — defence in depth; normally blocked by the lack of
//	  CAP_SYS_ADMIN, but an explicit deny prevents any user-namespace tricks.
//
// perf_event_open is intentionally NOT denied: the JVM uses it for profiling.
// Network access is already blocked by nsjail's network namespace isolation,
// so socket/connect/bind do not need to be denied at the seccomp level.
// Syscalls intentionally omitted from the deny-list because they are absent
// from the ARM64 (aarch64) Kafel syscall table — Kafel would fail to compile
// the policy with "Undefined identifier" if they were included:
//
//	kexec_file_load  — x86_64 only; ARM64 uses kexec_load for all kexec ops.
//	mknod            — absent from ARM64; mknodat (below) covers the attack surface.
//	io_uring_setup / io_uring_enter / io_uring_register — not in this ARM64
//	                   Kafel table; syscalls that don't exist can't be invoked
//	                   so no deny rule is needed.
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

// RunConfig describes one nsjail invocation.
type RunConfig struct {
	NsjailPath   string
	WorkspaceDir string
	Limits       config.LimitsDef
	// Cmd is the program to run inside the sandbox (e.g. "/usr/bin/python3").
	Cmd string
	// Args are the arguments passed to Cmd after template expansion.
	Args []string
	// Stdin is fed to the sandboxed process.
	Stdin io.Reader
	// MaxOutputBytes caps captured stdout.
	MaxOutputBytes int64
	// BindMounts are additional read-only bind mounts: "host:container".
	BindMounts []string
	// Env is a list of extra "KEY=VALUE" environment variables for this invocation.
	Env []string
	// CgroupParent is the cgroup v2 directory name for this job.
	CgroupParent string
}

// RunResult holds the captured output of one nsjail invocation.
type RunResult struct {
	Stdout       []byte
	Stderr       []byte
	Log          []byte // nsjail's own diagnostic output (--log_fd 3)
	ExitCode     int
	DurationMs   int64
	Truncated    bool
	MemoryPeakKB int64
}

// Run executes cmd inside an nsjail sandbox and returns the result.
// It never uses a shell — argv is built as a pure []string.
func Run(ctx context.Context, cfg RunConfig) (RunResult, error) {
	// Generate a unique cgroup parent for this run to track memory peak.
	cfg.CgroupParent = fmt.Sprintf("goboxd-%d", time.Now().UnixNano())

	// Create the cgroup directory so nsjail can use it and we can read it later.
	cgroupPath := filepath.Join("/sys/fs/cgroup", cfg.CgroupParent)
	_ = os.Mkdir(cgroupPath, 0o755)
	defer func() {
		// If nsjail was killed (e.g. context timeout) before it could clean up its
		// own child cgroup, the parent directory will be non-empty and os.Remove
		// (rmdir) would fail with ENOTEMPTY. Remove child cgroup directories first;
		// kernel virtual files (cgroup.procs, memory.peak, …) are not real inodes
		// and do not prevent rmdir of the parent once sub-cgroups are gone.
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

	// Open a pipe for nsjail's own log output (--log_fd 3 in buildArgv).
	// ExtraFiles[0] becomes fd 3 in the child (after stdin/stdout/stderr).
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
	// Close write end in parent so logR gets EOF when nsjail exits.
	_ = logW.Close()

	// Drain nsjail log in a goroutine so a large log can't deadlock stdout.
	var logBuf bytes.Buffer
	logDone := make(chan struct{})
	go func() {
		defer close(logDone)
		_, _ = io.Copy(&logBuf, logR)
		_ = logR.Close()
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

	// Read peak memory from cgroup v2
	var peakKB int64
	if peakBytes, err := os.ReadFile(filepath.Join(cgroupPath, "memory.peak")); err == nil {
		var peak int64
		if _, err := fmt.Sscanf(string(bytes.TrimSpace(peakBytes)), "%d", &peak); err == nil {
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

// buildArgv constructs the nsjail command-line as a pure []string.
// No shell is involved at any point. Compatible with nsjail 3.4+.
func buildArgv(cfg RunConfig) []string {
	argv := []string{cfg.NsjailPath}

	argv = append(argv,
		"--mode", "o",
		"--chroot", cfg.WorkspaceDir,
		"--user", "65534",
		"--group", "65534",
		"--log_fd", "3", // nsjail log → fd 3
		"--max_cpus", "1",
		"--rw",       // remount chroot r/w so compilers can write artifacts
		"--cwd", "/", // working directory inside the jail
		// Consistent UTS hostname inside the jail; prevents host hostname leaking
		// via gethostname(2) in user code.
		"--hostname", "goboxd",
		// Use cgroup v2 when available (host must mount /sys/fs/cgroup as cgroup2;
		// docker-compose sets cgroupns: host for this).
		"--detect_cgroupv2",
		"--cgroupv2_mount", "/sys/fs/cgroup",
		"--cgroup_mem_parent", cfg.CgroupParent,
		// File-descriptor cap; Python and javac open many fds at startup.
		"--rlimit_nofile", "1000",
		// Core dumps disabled: saves disk space and prevents source-code leakage
		// via coredumpctl on the host.
		"--rlimit_core", "0",
		// Hard stack ceiling (8 MiB = Linux default). Without this, the jail
		// inherits the host's rlimit which is sometimes set to unlimited in
		// container environments, allowing stack-based memory exhaustion.
		"--rlimit_stack", "8",
		// Minimal environment: no host secrets leak in; runtimes find their paths.
		"--env", "TMP=/",
		"--env", "TMPDIR=/",
		"--env", "HOME=/",
		"--env", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	)

	// Per-language extra env vars (e.g. GO111MODULE=off for Go).
	for _, e := range cfg.Env {
		argv = append(argv, "--env", e)
	}

	if cfg.Limits.WallTimeS > 0 {
		argv = append(argv, "--time_limit", fmt.Sprintf("%d", cfg.Limits.WallTimeS))
		// CPU rlimit set one second above the wall-time limit so that nsjail's
		// own wall-time check always fires first and logs "run time >= time limit".
		// When both are equal, a 100% CPU-bound process receives SIGXCPU from the
		// kernel before nsjail's poll loop wakes up, causing the status to parse
		// as runtime_error instead of time_exceeded.
		argv = append(argv, "--rlimit_cpu", fmt.Sprintf("%d", cfg.Limits.WallTimeS+1))
	}
	if cfg.Limits.MemoryKB > 0 {
		// cgroup memory.max enforces RSS; this is the primary limit and makes
		// memory_exceeded detection reliable (nsjail logs the OOM event).
		cgroupMemBytes := int64(cfg.Limits.MemoryKB) * 1024
		argv = append(argv, "--cgroup_mem_max", fmt.Sprintf("%d", cgroupMemBytes))
		// Disable swap entirely so the process OOMs at exactly memory.max rather
		// than silently spilling into swap and making the limit unpredictable.
		argv = append(argv, "--cgroup_mem_swap_max", "0")
		// RLIMIT_AS caps virtual address space separately from RSS. Interpreters
		// (Python, JVM) mmap large amounts of virtual space at startup:
		// the JVM on ARM64 pre-allocates ~1 GiB for compressed class space alone.
		// The floor is set to 4096 MiB so JVM-based runtimes (Java, Kotlin) can
		// start reliably. The cgroup memory.max above is the real RSS guard.
		virtMiB := cfg.Limits.MemoryKB * 4 / 1024
		if virtMiB < 4096 {
			virtMiB = 4096
		}
		argv = append(argv, "--rlimit_as", fmt.Sprintf("%d", virtMiB))
	}
	if cfg.Limits.MaxProcesses > 0 {
		// cgroup pids.max is the hard limit; rlimit_nproc is the per-user fallback.
		argv = append(argv, "--cgroup_pids_max", fmt.Sprintf("%d", cfg.Limits.MaxProcesses))
		argv = append(argv, "--rlimit_nproc", fmt.Sprintf("%d", cfg.Limits.MaxProcesses))
	}

	// File size cap: 100 MB per created file (safety net against runaway writes).
	argv = append(argv, "--rlimit_fsize", "100")

	for _, bm := range cfg.BindMounts {
		parts := strings.SplitN(bm, ":", 2)
		if len(parts) == 2 {
			argv = append(argv, "-R", parts[0]+":"+parts[1])
		} else {
			argv = append(argv, "-R", bm)
		}
	}

	argv = append(argv, "--seccomp_string", seccompPolicy)

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
