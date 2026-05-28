# Security Overview

`goboxd` operates in a highly adversarial environment where executing untrusted code is the primary feature. The architecture is explicitly designed to address vulnerabilities common in naive sandboxing implementations. 

The original Python reference implementation (`pyjail`) deliberately included seven critical security loopholes for educational purposes. `goboxd` closes all of them.

---

## 1. Path Traversal via Filename

**The Risk:** 
If client-supplied filenames are not validated, a user could provide a path like `../../etc/passwd` to escape the sandboxed workspace and interact with host system files.

**The Fix:** 
The `validate.Filename()` function (in `internal/validate/request.go`) strictly enforces path safety. It explicitly rejects:
- Empty strings
- Absolute paths
- Strings containing directory separators (`/` or `\`)
- Strings where `filepath.Base(s) != s`
- Leading dots (`.`)
- Non-printable or non-ASCII characters
- Strings exceeding 64 characters

**Enforcement Location:** `internal/handler/run.go` processes all filenames through this validator before appending them to the workspace root via `filepath.Join(ws.Dir, filename)`.

---

## 2. Shell-style Directory Commands

**The Risk:** 
Using shell commands (e.g., `os.system("rm -rf " + dir)`) to manipulate directories introduces shell injection if the directory path contains special characters.

**The Fix:** 
`goboxd` does not invoke the shell for any file operations. `NewWorkspace()` uses `os.MkdirTemp()`, `Workspace.TestDir()` uses `os.MkdirAll()`, and `Workspace.Cleanup()` relies on `os.RemoveAll()`. The absence of shell evaluation entirely neutralizes injection risks.

---

## 3. Compiler-Flag Injection

**The Risk:** 
Permitting unvalidated compiler flags grants attackers compile-time code execution capabilities via malicious flags like `-fplugin=evil.so`, `-B/tmp`, `--specs=/tmp/bad`, or `@responsefile`.

**The Fix:** 
The `validate.Flags()` function validates every user-provided flag against a strict, per-language `flag_allowlist` defined in `configs/languages.yaml`. 
- Prefix matching is supported (e.g., `-std=*` permits `-std=c++17`).
- Any non-allowlisted flag results in an HTTP 400 Bad Request error.

---

## 4. Unbounded Request Sizes

**The Risk:** 
Unbounded request payloads allow attackers to exhaust server memory and disk space.

**The Fix (Four Layers):**
1. **HTTP Body Limit:** `internal/handler/middleware.go` applies `http.MaxBytesReader` to terminate oversized requests before parsing begins.
2. **Source Code Limit:** `internal/handler/run.go` enforces a maximum source file size (default 256 KiB).
3. **Stdin Limit:** `validate.StdinSize()` limits individual test case inputs.
4. **Output Capture Limit:** `internal/sandbox/nsjail.go` uses `io.LimitReader` to safely cap child process output. 

---

## 5. UID Collisions Under Load

**The Risk:** 
Relying on random UID generation for temporary directories creates collisions under concurrent load, leading to race conditions where one request overwrites another's workspace.

**The Fix:** 
`NewWorkspace()` uses `os.MkdirTemp(jailDir, "goboxd-*")`, delegating collision-free atomic directory creation directly to the OS kernel.

---

## 6. Unbounded Child Output Memory Exhaustion

**The Risk:** 
Buffering an unbounded process output directly into memory can trigger a host-side Out-Of-Memory (OOM) error if a script prints gigabytes of data.

**The Fix:** 
`nsjail.Run()` wraps the `stdout` pipe with an `io.LimitReader`. When the output limit (default 256 KiB) is reached, reading halts, the captured output is marked with `\n[output truncated]`, and the remainder of the pipe is drained to `io.Discard` so the process doesn't block.

---

## 7. Stale Jail Directories

**The Risk:** 
Failing to guarantee workspace cleanup on early returns or panics results in disk space exhaustion over time due to stale directories.

**The Fix (Two Mechanisms):**
1. **Deferred Cleanup:** `internal/runner/runner.go` immediately defers `ws.Cleanup()` after workspace creation, ensuring it runs on every exit path (including caught panics).
2. **Startup Sweep:** On startup, `cmd/goboxd/main.go` invokes `SweepOrphans()` to clean out any legacy directories left over from prior unexpected terminations.

---

## Advanced Hardening: Seccomp-BPF Syscall Filtering

Beyond architectural fixes, `goboxd` enhances `nsjail` with a custom Kafel seccomp policy via the `--seccomp_string` argument. 

Instead of a fragile allowlist, `goboxd` utilizes a **targeted deny-list (`KILL_PROCESS`)**. This is vital for a multi-language sandbox, as enumerating every legitimate syscall across Node, JVM, GCC, Python, and Rust is impractical. 

> **Note:** We use `KILL_PROCESS` (not `KILL`) to tear down the entire process group, preventing multi-threaded programs from surviving a restricted syscall.

### Restricted Syscalls

| Syscall(s) | Risk Mitigated |
|------------|----------------|
| `ptrace`, `process_vm_readv/writev` | Cross-process memory inspection and modification. |
| `init_module`, `finit_module`, `delete_module` | Kernel module loading. |
| `kexec_load`, `kexec_file_load` | Replacing the running kernel. |
| `reboot` | Unauthorized system restarts. |
| `settimeofday`, `adjtimex`, `clock_adjtime` | Host clock manipulation. |
| `mknod`, `mknodat` | Device node creation (chroot bypass). |
| `chroot`, `pivot_root` | Escaping `nsjail` mount restrictions. |
| `unshare`, `setns` | Un-isolating network, PID, or mount namespaces. |
| `io_uring_setup/enter/register` | Async I/O (frequent source of privilege escalation CVEs). |
| `userfaultfd` | Kernel page-fault manipulation (common in exploit chains). |
| `name_to_handle_at`, `open_by_handle_at` | Crossing mount boundaries via file handles. |
| `acct` | Process accounting interference. |
| `bpf` | Loading eBPF programs into the kernel. |
| `syslog` | Reading the kernel ring-buffer (information leak). |

**Permitted exceptions:**
- `perf_event_open`: Not denied because the JVM (e.g., Kotlin) requires it for profiling.
- Network syscalls (`socket`, `connect`, `bind`): Blocked at the `nsjail` network namespace level rather than via seccomp.
