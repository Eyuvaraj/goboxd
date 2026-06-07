# Sandbox Containment Tests

This document describes the adversarial test suite in `tests/sandbox/`. The tests verify that goboxd's nsjail sandbox actually contains dangerous programs — not just that the API returns the right status codes under normal conditions.

These tests are **separate from the integration suite** (`make integration`). They run once against a live service and produce a Markdown report you can save and review.

---

## How to Run

You need the service running in one terminal and the probe script in another.

```bash
# Terminal 1 — build and start the service
make run

# Terminal 2 — run all 15 probes, print results to stdout
go run ./tests/sandbox/

# Save the report to a file
go run ./tests/sandbox/ --out docs/sandbox-report.md

# Target a different address
go run ./tests/sandbox/ --url http://localhost:9090
```

The full suite takes under 45 seconds. Each probe waits for its configured time limit before moving on.

---

## Reading the Results

The script prints a Markdown table with one row per probe:

| Column | Meaning |
|--------|---------|
| **Expected** | The status(es) that confirm the sandbox contained the program |
| **Actual** | The status goboxd returned |
| **Result** | `PASS`, `FAIL`, or `BREACH` |

**PASS** — the sandbox stopped the program as expected.

**FAIL** — the program returned a status that was neither expected nor a breach. Usually means a misconfigured limit or the wrong language was used.

**BREACH** — the program ran to completion when it should have been blocked. This is a security gap, not a test error. The script exits with code `2` and the affected probe is highlighted in the report.

---

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | All probes passed |
| `1` | One or more unexpected statuses (FAIL) |
| `2` | One or more security breaches detected |

---

## The Probes

Programs live in `tests/sandbox/programs/` as real source files. The script embeds them at compile time — no runtime file reads.

---

### CPU Limits / Timeout Enforcement

**Probe:** CPU Stress  
**File:** `cpu_stress.py`

```python
# Tight infinite loop — wall-time limit must fire.
while True:
    pass
```

**How it works:** The loop consumes one CPU core indefinitely. nsjail's `--time_limit` fires after the configured `wall_time_s` and kills the process.

**Expected:** `time_exceeded`

**If it fails:** The wall-time limit is not enforced. Check that `wall_time_s` is set in the language YAML and that `--time_limit` appears in the nsjail argv.

---

### Memory Limits

**Probe:** Memory Allocation  
**File:** `mem_alloc.py`

```python
# Allocate 512 MiB — cgroup memory cap must kill the process.
x = bytearray(512 * 1024 * 1024)
```

**How it works:** Python asks the OS for 512 MiB. The cgroup `memory.max` is set to 32 MiB for this probe (`--memory_kb 32768` override). When physical pages are touched, the cgroup OOM killer fires and sends SIGKILL.

**Expected:** `memory_exceeded` or `runtime_error`  
Both confirm containment. `runtime_error` occurs when the SIGKILL arrives before nsjail reads the cgroup `memory.events` file.

**If it fails (returns `accepted`):** The memory cap is not enforced. The container may not have cgroup v2 support, or `--detect_cgroupv2` failed. Check that `--cgroup_mem_max` appears in the nsjail argv and that the container was started with `--security-opt seccomp=unconfined`.

---

### Disk Quotas

**Probe:** Large File Write  
**File:** `file_write_large.py`

```python
# Write a single file past rlimit_fsize (100 MB) — SIGXFSZ must terminate.
with open('/bigfile', 'wb') as f:
    while True:
        f.write(b'A' * (1024 * 1024))
```

**How it works:** nsjail sets `--rlimit_fsize 100` (100 MB per-file limit). Once the file reaches 100 MB, the kernel sends SIGXFSZ to the process. If the jail workspace root is not writable by the jail user, the `open()` itself raises a `PermissionError` — either way the result is `runtime_error`.

**Expected:** `runtime_error` or `time_exceeded`

---

**Probe:** Temp File Flood  
**File:** `tempfile_flood.py`

```python
# Create temp files without limit — wall time must stop this.
import tempfile
while True:
    tempfile.NamedTemporaryFile(delete=False, dir='/')
```

**How it works:** The program creates files as fast as possible. There is no explicit inode or file-count limit in the sandbox, so this probe primarily verifies that the wall-time limit stops unbounded disk activity before the host runs out of space.

**Expected:** `time_exceeded` or `runtime_error`

---

### Process Limits

**Probe:** Fork Bomb  
**File:** `fork_bomb.py`

```python
# Process flood — cgroup_pids_max + rlimit_nproc must stop it.
import os
while True:
    os.fork()
```

**How it works:** `os.fork()` doubles the process count on every iteration. nsjail enforces two limits simultaneously: `--cgroup_pids_max` (a kernel cgroup limit on total PIDs) and `--rlimit_nproc` (a per-user process count limit). When either limit is hit, `os.fork()` raises `BlockingIOError` (EAGAIN), the exception goes unhandled, and Python exits with code 1.

**Expected:** `runtime_error` or `time_exceeded`

**If it fails (returns `accepted`):** Process limits are not enforced. Check that both `--cgroup_pids_max` and `--rlimit_nproc` appear in the nsjail argv, and that the container has `--security-opt seccomp=unconfined` (needed for `clone3` which is the backing syscall for process creation).

---

**Probe:** Process Spawn  
**File:** `proc_spawn.py`

```python
# Spawn long-lived children to fill the process table.
import os, time
while True:
    if os.fork() == 0:
        time.sleep(30)
        os._exit(0)
```

**How it works:** Unlike the pure fork bomb, each child sleeps for 30 seconds instead of forking again. This fills the process table with sleeping processes rather than with actively forking ones, testing whether accumulated long-lived children are counted against the limit.

**Expected:** `runtime_error` or `time_exceeded`

---

**Probe:** Thread Explosion  
**File:** `thread_flood.py`

```python
# Linux threads count against rlimit_nproc — spawn until limit hit.
import threading, time
while True:
    threading.Thread(target=time.sleep, args=(60,), daemon=True).start()
```

**How it works:** On Linux, threads are implemented as processes sharing an address space (`clone(CLONE_THREAD)`). They count against `rlimit_nproc` the same way `fork()` does. When the limit is hit, `threading.Thread.start()` raises `RuntimeError: can't start new thread`.

**Expected:** `runtime_error` or `time_exceeded`

---

**Probe:** C Fork Bomb  
**File:** `fork_bomb.c`

```c
/* Native fork bomb — kernel cgroup_pids_max must contain it. */
#include <unistd.h>
int main(void) {
    while (1) { (void)fork(); }
    return 0;
}
```

**How it works:** Same as the Python fork bomb but as a compiled native binary. This verifies that process limits apply at the kernel level, not through any interpreter-level mechanism.

**Expected:** `runtime_error` or `time_exceeded`

---

### Deep Recursion

**Probe:** Deep Recursion  
**File:** `deep_recursion.py`

```python
# Unbounded recursion — Python raises RecursionError and exits non-zero.
def f():
    return f()
f()
```

**How it works:** Python's default recursion limit is 1000 frames. The function calls itself until `RecursionError` is raised. Since the exception is unhandled, Python prints a traceback and exits with code 1.

**Expected:** `runtime_error`

Note: This tests Python's built-in depth limit, not the nsjail stack size (`--rlimit_stack`). A C version would instead test the OS stack limit.

---

### Output Cap

**Probe:** Output Bomb  
**File:** `output_bomb.py`

```python
# Unbounded stdout — io.LimitReader drains to Discard; wall time fires.
while True:
    print('A' * 1000)
```

**How it works:** The program writes data as fast as the OS allows. goboxd reads stdout through `io.LimitReader(pipe, maxBytes+1)`. Once `MAX_OUTPUT_BYTES` is read, the remaining pipe output is drained to `io.Discard` — the process keeps running (it never blocks on write) but no more data is buffered. The process eventually hits the wall-time limit.

**Expected:** `time_exceeded`

**Why this matters:** Without `io.LimitReader`, a process generating gigabytes of output would exhaust the service's memory. The test verifies the service remains responsive even under output flooding.

---

### Network Isolation

**Probe:** Network Connect  
**File:** `network_connect.py`

```python
# Outbound connect — new network namespace has no interfaces; must fail ENETUNREACH.
import socket
socket.create_connection(('8.8.8.8', 53), timeout=1)
```

**How it works:** nsjail creates a new network namespace (`CLONE_NEWNET`) with `--iface_no_lo`, which means no loopback and no other interfaces. Any `connect()` call immediately returns `ENETUNREACH` (errno 101) — there is no route to any address. The unhandled `OSError` causes Python to exit with code 1.

**Expected:** `runtime_error`

**If it fails (returns `accepted`):** The program connected to `8.8.8.8:53`. The jail has outbound internet access. Verify that `--iface_no_lo` is present in the nsjail argv and that `CLONE_NEWNET` was applied (check nsjail logs for namespace setup errors).

---

### Filesystem Isolation

**Probe:** Read /etc/passwd  
**File:** `fs_read_passwd.py`

```python
# Read /etc/passwd — path does not exist inside the chroot; must raise FileNotFoundError.
open('/etc/passwd').read()
```

**How it works:** nsjail chroots the process to the per-job workspace directory. The workspace contains only the source file, the compiled binary (if any), and a `proc` mount point. `/etc` is never created or bind-mounted. `open('/etc/passwd')` raises `FileNotFoundError`.

**Expected:** `runtime_error`

**If it fails (returns `accepted` or `wrong_output`):** `/etc/passwd` is readable inside the jail. This means `/etc` is either bind-mounted or the chroot is not active. Check `--chroot` in the nsjail argv and review the bind-mounts in `configs/languages.yaml`.

---

**Probe:** Write /etc/hostname  
**File:** `fs_write_etc.py`

```python
# Write to /etc — directory absent in chroot; must raise FileNotFoundError.
open('/etc/hostname', 'w').write('hacked')
```

**How it works:** Same chroot isolation as the read probe, but testing a write path. Since `/etc` doesn't exist in the workspace, `open()` raises `FileNotFoundError` before any write occurs.

**Expected:** `runtime_error`

**If it fails (returns `accepted`):** The program wrote to `/etc/hostname` on the host. Critical filesystem isolation failure.

---

### Container Escape Resistance

**Probe:** Chroot Escape  
**File:** `escape_chroot.py`

```python
# os.chroot() requires CAP_SYS_CHROOT — must raise PermissionError.
import os
os.chroot('/')
```

**How it works:** `chroot(2)` requires the `CAP_SYS_CHROOT` capability. nsjail drops all capabilities before exec-ing the user program, so the call raises `PermissionError` (EPERM). Additionally, `chroot` is in the seccomp deny-list as a secondary layer.

**Expected:** `runtime_error`

**If it fails (returns `accepted`):** The process called `chroot('/')`, which is a no-op when you're already in a chroot but signals that capabilities were not dropped. Check nsjail's `--cap` configuration.

---

**Probe:** Ptrace Syscall  
**File:** `escape_ptrace.py`

```python
# ptrace is in the seccomp KILL_PROCESS deny-list — must deliver SIGKILL.
import ctypes
ctypes.CDLL(None).ptrace(0, 0, 0, 0)
```

**How it works:** `ctypes.CDLL(None)` loads libc. Calling `.ptrace(0, 0, 0, 0)` makes the `ptrace(2)` syscall (PTRACE_TRACEME). The seccomp BPF filter catches this syscall and applies the `KILL_PROCESS` action, which sends SIGKILL to the entire process group immediately. nsjail detects the signal-killed exit and reports `runtime_error`.

`ptrace` is one of the primary sandbox-escape primitives — it allows one process to read and modify the memory and registers of another. Blocking it at the seccomp level ensures it cannot be used even if capabilities were somehow misconfigured.

**Expected:** `runtime_error`

**If it fails (returns `accepted`):** The `ptrace` syscall succeeded. The seccomp policy is not active. Check that `--seccomp_string` appears in the nsjail argv and that the seccomp policy compiled without errors (look for `[W]` lines in nsjail logs at startup).

---

## File Layout

```
tests/sandbox/
├── main.go              # probe runner — go run ./tests/sandbox/
└── programs/
    ├── cpu_stress.py    # CPU / timeout
    ├── mem_alloc.py     # memory limits
    ├── file_write_large.py  # disk quota (rlimit_fsize)
    ├── tempfile_flood.py    # disk quota (file count)
    ├── fork_bomb.py     # process limits (Python)
    ├── fork_bomb.c      # process limits (native C)
    ├── proc_spawn.py    # process limits (long-lived children)
    ├── thread_flood.py  # process limits (threads)
    ├── deep_recursion.py  # stack / recursion
    ├── output_bomb.py   # output cap
    ├── network_connect.py  # network isolation
    ├── fs_read_passwd.py   # filesystem isolation (read)
    ├── fs_write_etc.py     # filesystem permissions (write)
    ├── escape_chroot.py    # container escape (capabilities)
    └── escape_ptrace.py    # container escape (seccomp)
```

---

## Probe Summary Table

| # | Probe | Lang | Wall Time | Memory Cap | Expected Status |
|---|-------|------|-----------|------------|-----------------|
| 1 | CPU Stress | py3 | 1 s | default | `time_exceeded` |
| 2 | Memory Allocation | py3 | 5 s | 32 MiB | `memory_exceeded` or `runtime_error` |
| 3 | Large File Write | py3 | 10 s | default | `runtime_error` or `time_exceeded` |
| 4 | Temp File Flood | py3 | 2 s | default | `time_exceeded` or `runtime_error` |
| 5 | Fork Bomb | py3 | 3 s | default | `runtime_error` or `time_exceeded` |
| 6 | Process Spawn | py3 | 3 s | default | `runtime_error` or `time_exceeded` |
| 7 | Thread Explosion | py3 | 3 s | default | `runtime_error` or `time_exceeded` |
| 8 | C Fork Bomb | c | 3 s | default | `runtime_error` or `time_exceeded` |
| 9 | Deep Recursion | py3 | 5 s | default | `runtime_error` |
| 10 | Output Bomb | py3 | 1 s | default | `time_exceeded` |
| 11 | Network Connect | py3 | 3 s | default | `runtime_error` |
| 12 | Read /etc/passwd | py3 | 5 s | default | `runtime_error` |
| 13 | Write /etc/hostname | py3 | 5 s | default | `runtime_error` |
| 14 | Chroot Escape | py3 | 5 s | default | `runtime_error` |
| 15 | Ptrace Syscall | py3 | 5 s | default | `runtime_error` |

---

## Troubleshooting

**The script says "service unreachable"**  
Run `make run` in another terminal and wait for it to print `listening on :8080`.

**All probes return `internal_error`**  
nsjail is not found or can't run. The service must run inside Docker (`make run`), not natively on macOS.

**Fork bomb probes return `accepted`**  
The container is missing `--security-opt seccomp=unconfined`. Add it to the `docker run` command in the `Makefile`. Without it, `clone3` (the modern `fork()` backing syscall) is blocked by Docker's default seccomp profile before nsjail even starts.

**Memory probe never triggers `memory_exceeded`**  
cgroup v2 memory accounting is not active. Check that `/sys/fs/cgroup/memory.max` exists inside the container. Some Docker environments require `--cgroupns=host` or explicit memory limit settings on the container itself.

**Network probe returns `runtime_error` for the wrong reason**  
If the timeout fires before `ENETUNREACH` (which would be unusual), you would still see `time_exceeded` rather than `runtime_error`. Either result confirms the connection was not made; the probe only fails on `accepted`.
