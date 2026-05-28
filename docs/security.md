# Security

`goboxd` runs untrusted code by design. The architecture is built around seven vulnerabilities that exist in the Python reference implementation (`pyjail`). All seven are closed.

---

## 1. Path Traversal via Filename

**Risk:** A client-supplied filename like `../../etc/passwd` joined with `filepath.Join` escapes the workspace and writes to host files.

**Fix:** Every filename from the request body passes `validate.Filename()` (`internal/validate/request.go`) before any path join. The function enforces: `filepath.Base(n) == n`, only `[a-zA-Z0-9._-]+`, no leading dot, max 64 characters.

**Enforced in:** `internal/handler/run.go` — before `filepath.Join(ws.Dir, filename)`.

---

## 2. Shell-Style Directory Commands

**Risk:** Using `os.system("rm -rf " + dir)` or `exec.Command("sh", "-c", ...)` with a path lets shell metacharacters execute arbitrary commands.

**Fix:** No shell is invoked anywhere. `NewWorkspace` uses `os.MkdirTemp`, `Cleanup` uses `os.RemoveAll`, and every external program is launched as a pure `[]string` argv via `exec.CommandContext`.

**Enforced in:** `internal/sandbox/workspace.go`, `internal/sandbox/nsjail.go`.

---

## 3. Compiler-Flag Injection

**Risk:** Arbitrary flags passed to `gcc`/`g++`/`javac` give compile-time code execution. Examples: `-fplugin=evil.so` loads a shared library, `-B/tmp` redirects the toolchain, `@file` reads extra flags from an attacker-controlled file.

**Fix:** `validate.Flags()` checks every flag in `build.flags` and `run.flags` against a per-language `flag_allowlist` from `configs/languages.yaml`. Prefix matching is supported (`-std=*`). Any unlisted flag returns 400 `invalid_flag`.

**Enforced in:** `internal/handler/run.go` — before `runner.Submit`.

---

## 4. Unbounded Request Sizes

**Risk:** No size limits allow source, stdin, or expected_stdout payloads to exhaust server memory and disk.

**Fix — four layers:**
1. `handler.BodyLimit` (`internal/handler/middleware.go`) — `http.MaxBytesReader` set to `source_max + tests × 2 × stdin_max + 64 KiB` before JSON decode.
2. `validate.SourceSize` — rejects source over `MAX_SOURCE_BYTES` (default 256 KiB).
3. `validate.StdinSize` / `validate.ExpectedSize` — rejects oversized per-test fields.
4. `io.LimitReader(pipe, max+1)` in `sandbox.Run` — caps captured stdout per phase.

**Enforced in:** `internal/handler/middleware.go`, `internal/handler/run.go`, `internal/sandbox/nsjail.go`.

---

## 5. UID Collisions Under Load

**Risk:** A random UID scheme (e.g. `rand.Intn(30000)`) collides under concurrent load — two jobs share a workspace and can read each other's source.

**Fix:** `os.MkdirTemp(jailDir, "goboxd-*")` atomically creates a unique directory. No counter, no retry loop.

**Enforced in:** `internal/sandbox/workspace.go:NewWorkspace`.

---

## 6. Unbounded Child Output

**Risk:** A process printing gigabytes of data is read with `io.ReadAll`, causing host OOM.

**Fix:** `io.LimitReader(stdoutPipe, maxBytes+1)` in `sandbox.Run`. When the limit is hit, the output is truncated and a `\n[output truncated]` marker is appended. The remaining pipe is drained to `io.Discard` so the sandboxed process is not blocked on write.

**Enforced in:** `internal/sandbox/nsjail.go:Run`.

---

## 7. Stale Jail Directories

**Risk:** A panic between workspace creation and cleanup leaks the directory, filling the disk over time.

**Fix — two mechanisms:**
1. `defer ws.Cleanup()` is placed immediately after `NewWorkspace` in the runner — it runs on every exit path including panics caught by the `Recoverer` middleware.
2. `SweepOrphans` at startup removes any `goboxd-*` directories older than `ORPHAN_MAX_AGE_MIN` (default 60 minutes) left from previous unclean shutdowns.

**Enforced in:** `internal/runner/runner.go`, `internal/sandbox/workspace.go:SweepOrphans`, `cmd/goboxd/main.go`.

---

## Seccomp-BPF Syscall Filtering

Beyond the architectural fixes, `goboxd` passes a Kafel deny-list to nsjail via `--seccomp_string`. `DEFAULT ALLOW` keeps all 13 language runtimes working without enumerating their required syscalls. `KILL_PROCESS` (not `KILL`) terminates the whole process group — not just the offending thread.

The exact policy is in `internal/sandbox/nsjail.go:seccompPolicy`. Syscalls denied:

| Syscall(s) | Risk |
|------------|------|
| `ptrace`, `process_vm_readv`, `process_vm_writev` | Cross-process memory inspection and writes — sandbox escape primitives |
| `init_module`, `finit_module`, `delete_module` | Kernel module loading — arbitrary kernel code execution |
| `kexec_load` | Replace the running kernel |
| `reboot` | Unauthorized system restart |
| `settimeofday`, `adjtimex`, `clock_adjtime` | Host clock skew — affects timeout logic and log timestamps |
| `mknodat` | Create device nodes — enables device escapes inside a chroot |
| `chroot`, `pivot_root` | Change filesystem root — could escape nsjail's bind-mount restrictions |
| `unshare`, `setns` | Manipulate Linux namespaces — could un-isolate network, PID, or mount |
| `userfaultfd` | Pause kernel page-fault handling from userspace — used in many exploit chains |
| `name_to_handle_at`, `open_by_handle_at` | Cross mount-point boundaries via file handles |
| `acct` | Process accounting — unneeded and can interfere with host resource bookkeeping |
| `bpf` | Load eBPF programs — kernel-level arbitrary code execution |
| `syslog` | Read the kernel ring buffer — information leak |
| `add_key`, `request_key`, `keyctl` | Kernel keyring — can persist data across sandbox invocations via the session keyring |
| `fanotify_init` | Filesystem access notification — leaks path information about files accessed inside the jail |
| `capset` | Modify process capabilities — defence in depth against privilege re-escalation |
| `mount` | Mount filesystems — normally blocked by missing `CAP_SYS_ADMIN`, but explicit deny prevents user-namespace tricks |

**Intentionally not denied:**
- `perf_event_open` — the JVM requires it for profiling (Kotlin, Java).
- `socket`, `connect`, `bind` — network access is already blocked at the nsjail network namespace level; seccomp denial is not needed.
- `mknod`, `io_uring_*`, `kexec_file_load` — absent from the ARM64 Kafel syscall table; a deny rule for a syscall that doesn't exist causes Kafel to fail at policy compile time.
