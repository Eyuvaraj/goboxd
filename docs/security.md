# Security

Seven deliberate security holes exist in the Python reference (pyjail). All seven are closed in goboxd.

## Hole 1 — Path traversal via filename

**Reference:** `code_manager.py:117–119` assigns the client-supplied `filename` directly to the language config with no validation. Values like `../../etc/passwd` or absolute paths escape the jail directory on the host.

**Fix:** `internal/validate/request.go` — `validate.Filename()` rejects: empty, absolute paths, strings containing `/` or `\`, strings where `filepath.Base(s) != s`, leading dot, non-printable or non-ASCII characters, and strings longer than 64 characters.

**Enforced at:** `internal/handler/run.go` — filenames from client are passed through `validate.Filename()` before use. Paths are always joined to the workspace directory with `filepath.Join(ws.Dir, filename)`.

---

## Hole 2 — Shell-style directory commands

**Reference:** `code_manager.py:103, 135, 193` creates and deletes per-request directories by formatting shell commands (`os.system(f"mkdir -p {self.root_directory}/proc")`, `os.system(f"rm -rf {self.root_directory}")`). Any special character in a path component can become a shell injection.

**Fix:** `internal/sandbox/workspace.go` — `NewWorkspace()` uses `os.MkdirTemp()`, `Workspace.TestDir()` uses `os.MkdirAll()`, and `Workspace.Cleanup()` uses `os.RemoveAll()`. Zero shell invocations anywhere in the codebase for filesystem operations.

---

## Hole 3 — Compiler-flag injection

**Reference:** `code_runner.py:168–172` joins client-supplied `extra_args` into the compiler argv with no filtering. Flags like `-fplugin=evil.so`, `-x c`, `-B/tmp`, `--specs=/tmp/bad`, `-Wl,-rpath,/tmp`, `@responsefile` give compile-time code execution.

**Fix:** `internal/validate/request.go` — `validate.Flags()` checks every flag against a per-language `flag_allowlist` from `configs/languages.yaml`. Entries ending in `*` are treated as prefix matches (e.g. `-std=*` allows `-std=c++17`). Any flag not in the list is rejected with HTTP 400 `{"error":{"code":"invalid_flag",...}}`.

**Enforced at:** `internal/handler/run.go` — build and run flags are validated before the job is submitted.

---

## Hole 4 — No request size limits

**Reference:** `server.py:45–52` — no `max_length` on the `message` argument. Source, testcase stdin, expected output, and evaluation script are all unbounded.

**Fix (four layers):**
1. **HTTP body:** `internal/handler/middleware.go` — `BodyLimit()` wraps `r.Body` with `http.MaxBytesReader(w, r.Body, maxBodyBytes)`. Oversized requests are rejected before any handler runs.
2. **Source:** `internal/handler/run.go` — `validate.SourceSize()` checks `len(source) <= cfg.MaxSourceBytes` (default 256 KiB).
3. **Stdin per test:** `validate.StdinSize()` checks each test's stdin.
4. **Captured output:** `internal/sandbox/nsjail.go` — `Run()` wraps the child stdout pipe with `io.LimitReader(stdoutPipe, maxOutputBytes+1)`. If the limit is exceeded, output is truncated and `\n[output truncated]` is appended.

---

## Hole 5 — UID collisions under load

**Reference:** `code_manager.py:74–76` picks a UID from a 30 000-wide range and retries only three times on collision. Under concurrent load, two requests can share the same directory, causing one to overwrite the other's source or stdin.

**Fix:** `internal/sandbox/workspace.go` — `NewWorkspace()` calls `os.MkdirTemp(jailDir, "goboxd-*")`. The OS kernel guarantees uniqueness atomically — no UID scheme, no collision possible regardless of concurrency.

---

## Hole 6 — Unbounded child output

**Reference:** `code_runner.py:301` reads the full child stdout into memory (`out, err = self.run_command(args)`). A program printing gigabytes of output can OOM the host process.

**Fix:** `internal/sandbox/nsjail.go` — `Run()` reads stdout through `io.LimitReader(stdoutPipe, cfg.MaxOutputBytes+1)`. If more than `MaxOutputBytes` (default 256 KiB) are produced, reading stops and the captured output is truncated with a `\n[output truncated]` marker. The remaining pipe is drained to `io.Discard` so the child is not blocked.

---

## Additional hardening — Seccomp-bpf syscall filtering

Beyond closing the seven pyjail holes, goboxd adds a Kafel deny-list seccomp policy passed to nsjail via `--seccomp_string`.

**File:** `internal/sandbox/nsjail.go` — `seccompPolicy` constant, applied in `buildArgv()`.

**Approach:** `DEFAULT ALLOW` with a targeted `KILL_PROCESS` list. This is appropriate for a multi-language sandbox because enumerating every syscall needed by Python, Node, JVM, GCC, Rust, OCaml, etc. is fragile. The deny-list instead blocks only calls with no legitimate use in a sandboxed executor.

`KILL_PROCESS` (not `KILL`) is used because `KILL` only terminates the calling thread — in a multi-threaded program, other threads would keep running. `KILL_PROCESS` tears down the whole process group.

| Syscall(s) | Risk blocked |
|---|---|
| `ptrace`, `process_vm_readv/writev` | Cross-process memory inspection and writes |
| `init_module`, `finit_module`, `delete_module` | Kernel module loading |
| `kexec_load`, `kexec_file_load` | Replacing the running kernel |
| `reboot` | System restart |
| `settimeofday`, `adjtimex`, `clock_adjtime` | Host clock manipulation |
| `mknod`, `mknodat` | Device node creation (chroot bypass) |
| `chroot`, `pivot_root` | Filesystem-root change — escape nsjail's mount restrictions |
| `unshare`, `setns` | Namespace manipulation — un-isolate network/PID/mount namespaces |
| `io_uring_setup/enter/register` | Async I/O with multiple privilege-escalation CVEs; unused by all runtimes |
| `userfaultfd` | Pause kernel page-fault handling — common in kernel exploit chains |
| `name_to_handle_at`, `open_by_handle_at` | File-handle syscalls that can cross mount boundaries |
| `acct` | Process accounting interference |
| `bpf` | Load eBPF programs into the kernel |
| `syslog` | Kernel ring-buffer read (information leak) |

`perf_event_open` is **not** denied — the JVM uses it for profiling, and denying it would break Kotlin. Network syscalls (`socket`, `connect`, `bind`) are **not** denied at the seccomp level because nsjail's network namespace isolation already provides that guarantee.

---

## Hole 7 — Stale jail directories

**Reference:** There is no `defer` or equivalent guaranteed-cleanup around the per-request directory lifecycle. A panic or early return between directory creation and cleanup leaks the directory permanently.

**Fix (two mechanisms):**
1. **Deferred cleanup:** `internal/runner/runner.go` — `defer ws.Cleanup()` is set immediately after `sandbox.NewWorkspace()` returns. In Go, `defer` runs on every exit path including panics caught by `middleware.Recoverer`.
2. **Startup sweep:** `internal/sandbox/workspace.go` — `SweepOrphans(jailDir, maxAge)` is called once in `cmd/goboxd/main.go` before the server starts. It removes any `goboxd-*` directories in `JailDir` whose modification time is older than `OrphanMaxAge` (default 30 minutes). This handles directories left over from a prior unclean shutdown.
