# Sandbox Containment Tests

15 adversarial programs submitted to a live goboxd service to verify the sandbox actually contains dangerous code, not just that the API returns correct status codes under normal conditions.

Separate from the integration suite (`make integration`). Runs against a live service and produces a Markdown report.

---

## Run

```bash
make run                                               # terminal 1
go run ./tests/sandbox/                                # terminal 2 (all 15 probes, stdout)
go run ./tests/sandbox/ --out docs/sandbox-report.md  # save report
go run ./tests/sandbox/ --url http://localhost:9090    # different address
```

Full suite takes under 45 seconds. Each probe waits for its configured time limit before moving on.

---

## Results

| # | Probe | Lang | Wall Time | Memory | Expected Status |
|---|-------|------|-----------|--------|-----------------|
| 1 | CPU Stress | py3 | 1 s | default | `time_exceeded` |
| 2 | Memory Allocation | py3 | 5 s | 32 MiB | `memory_exceeded` or `runtime_error` |
| 3 | Large File Write | py3 | 10 s | default | `runtime_error` or `time_exceeded` |
| 4 | Temp File Flood | py3 | 2 s | default | `time_exceeded` or `runtime_error` |
| 5 | Fork Bomb (Python) | py3 | 3 s | default | `runtime_error` or `time_exceeded` |
| 6 | Process Spawn | py3 | 3 s | default | `runtime_error` or `time_exceeded` |
| 7 | Thread Explosion | py3 | 3 s | default | `runtime_error` or `time_exceeded` |
| 8 | Fork Bomb (C) | c | 3 s | default | `runtime_error` or `time_exceeded` |
| 9 | Deep Recursion | py3 | 5 s | default | `runtime_error` |
| 10 | Output Bomb | py3 | 1 s | default | `time_exceeded` |
| 11 | Network Connect | py3 | 3 s | default | `runtime_error` |
| 12 | Read /etc/passwd | py3 | 5 s | default | `runtime_error` |
| 13 | Write /etc/hostname | py3 | 5 s | default | `runtime_error` |
| 14 | Chroot Escape | py3 | 5 s | default | `runtime_error` |
| 15 | Ptrace Syscall | py3 | 5 s | default | `runtime_error` |

**Result column meanings:**
- `PASS`: sandbox stopped the program as expected
- `FAIL`: status was neither expected nor a breach; usually a misconfigured limit
- `BREACH`: program ran to completion when it should have been blocked; security gap, not a test error; exits with code `2`

---

## Probe Details

<details>
<summary><strong>1. CPU Stress</strong> (py3, expected: <code>time_exceeded</code>)</summary>

```python
while True:
    pass
```

Tight infinite loop consumes one CPU core indefinitely. nsjail's `--time_limit` fires after `wall_time_s` and kills the process.

**If it fails:** wall-time limit is not enforced. Check that `wall_time_s` is set in the language YAML and `--time_limit` appears in the nsjail argv.

</details>

<details>
<summary><strong>2. Memory Allocation</strong> (py3, expected: <code>memory_exceeded</code> or <code>runtime_error</code>)</summary>

```python
x = bytearray(512 * 1024 * 1024)
```

Asks the OS for 512 MiB. The cgroup `memory.max` is set to 32 MiB for this probe. When physical pages are touched, the cgroup OOM killer fires and sends SIGKILL. Both statuses confirm containment; `runtime_error` occurs when SIGKILL arrives before nsjail reads `memory.events`.

**If it fails (returns `accepted`):** memory cap is not enforced. Check that `--cgroup_mem_max` appears in the nsjail argv and that the container was started with `seccomp:unconfined`.

</details>

<details>
<summary><strong>3. Large File Write</strong> (py3, expected: <code>runtime_error</code> or <code>time_exceeded</code>)</summary>

```python
with open('/bigfile', 'wb') as f:
    while True:
        f.write(b'A' * (1024 * 1024))
```

nsjail sets `--rlimit_fsize 100` (100 MB per-file limit). Once the file reaches 100 MB, the kernel sends SIGXFSZ. If the jail workspace root is not writable by the jail user, `open()` itself raises `PermissionError`; either way the result is `runtime_error`.

</details>

<details>
<summary><strong>4. Temp File Flood</strong> (py3, expected: <code>time_exceeded</code> or <code>runtime_error</code>)</summary>

```python
import tempfile
while True:
    tempfile.NamedTemporaryFile(delete=False, dir='/')
```

Creates files as fast as possible. No explicit inode/file-count limit in the sandbox, so this primarily verifies that the wall-time limit stops unbounded disk activity before the host runs out of space.

</details>

<details>
<summary><strong>5. Fork Bomb (Python)</strong> (py3, expected: <code>runtime_error</code> or <code>time_exceeded</code>)</summary>

```python
import os
while True:
    os.fork()
```

`os.fork()` doubles the process count on every iteration. nsjail enforces two limits: `--cgroup_pids_max` (kernel cgroup limit on total PIDs) and `--rlimit_nproc` (per-user process count). When either limit is hit, `os.fork()` raises `BlockingIOError` (EAGAIN).

**If it fails (returns `accepted`):** process limits are not enforced. Check that both `--cgroup_pids_max` and `--rlimit_nproc` appear in the nsjail argv, and that the container has `seccomp:unconfined`.

</details>

<details>
<summary><strong>6. Process Spawn</strong> (py3, expected: <code>runtime_error</code> or <code>time_exceeded</code>)</summary>

```python
import os, time
while True:
    if os.fork() == 0:
        time.sleep(30)
        os._exit(0)
```

Each child sleeps for 30 seconds instead of forking again. Tests whether accumulated long-lived children are counted against the process limit.

</details>

<details>
<summary><strong>7. Thread Explosion</strong> (py3, expected: <code>runtime_error</code> or <code>time_exceeded</code>)</summary>

```python
import threading, time
while True:
    threading.Thread(target=time.sleep, args=(60,), daemon=True).start()
```

On Linux, threads are implemented as processes sharing an address space (`clone(CLONE_THREAD)`) and count against `rlimit_nproc`. When the limit is hit, `Thread.start()` raises `RuntimeError: can't start new thread`.

</details>

<details>
<summary><strong>8. Fork Bomb (C)</strong> (c, expected: <code>runtime_error</code> or <code>time_exceeded</code>)</summary>

```c
#include <unistd.h>
int main(void) {
    while (1) { (void)fork(); }
    return 0;
}
```

Same as the Python fork bomb but as a compiled native binary. Verifies that process limits apply at the kernel level, not through any interpreter mechanism.

</details>

<details>
<summary><strong>9. Deep Recursion</strong> (py3, expected: <code>runtime_error</code>)</summary>

```python
def f():
    return f()
f()
```

Python's default recursion limit is 1000 frames. The function calls itself until `RecursionError` is raised; unhandled, Python exits with code 1. Tests Python's built-in depth limit, not the nsjail stack size (`--rlimit_stack`).

</details>

<details>
<summary><strong>10. Output Bomb</strong> (py3, expected: <code>time_exceeded</code>)</summary>

```python
while True:
    print('A' * 1000)
```

goboxd reads stdout through `io.LimitReader(pipe, maxBytes+1)`. Once `MAX_OUTPUT_BYTES` is read, the remaining pipe output is drained to `io.Discard`; the process keeps running (never blocks on write) but no more data is buffered. The process eventually hits the wall-time limit.

**Why it matters:** without `io.LimitReader`, a process generating gigabytes of output would exhaust the service's memory.

</details>

<details>
<summary><strong>11. Network Connect</strong> (py3, expected: <code>runtime_error</code>)</summary>

```python
import socket
socket.create_connection(('8.8.8.8', 53), timeout=1)
```

nsjail creates a new network namespace (`CLONE_NEWNET`) with `--iface_no_lo`: no loopback, no interfaces. Any `connect()` call immediately returns `ENETUNREACH` (errno 101); the unhandled `OSError` exits Python with code 1.

**If it fails (returns `accepted`):** the program connected to `8.8.8.8:53`. The jail has outbound internet access. Verify `--iface_no_lo` is present in the nsjail argv.

</details>

<details>
<summary><strong>12. Read /etc/passwd</strong> (py3, expected: <code>runtime_error</code>)</summary>

```python
open('/etc/passwd').read()
```

nsjail chroots the process to the per-job workspace directory. The workspace contains only the source file and compiled binary; `/etc` is never created or bind-mounted. `open('/etc/passwd')` raises `FileNotFoundError`.

**If it fails (returns `accepted` or `wrong_output`):** `/etc/passwd` is readable inside the jail. Check `--chroot` in the nsjail argv.

</details>

<details>
<summary><strong>13. Write /etc/hostname</strong> (py3, expected: <code>runtime_error</code>)</summary>

```python
open('/etc/hostname', 'w').write('hacked')
```

Same chroot isolation as probe 12, testing a write path. Since `/etc` does not exist in the workspace, `open()` raises `FileNotFoundError` before any write occurs.

**If it fails (returns `accepted`):** the program wrote to `/etc/hostname` on the host. Critical filesystem isolation failure.

</details>

<details>
<summary><strong>14. Chroot Escape</strong> (py3, expected: <code>runtime_error</code>)</summary>

```python
import os
os.chroot('/')
```

`chroot(2)` requires `CAP_SYS_CHROOT`. nsjail drops all capabilities before exec-ing the user program, so the call raises `PermissionError` (EPERM). Additionally, `chroot` is in the seccomp deny-list as a secondary layer.

**If it fails (returns `accepted`):** the process called `chroot('/')` without error, signalling that capabilities were not dropped.

</details>

<details>
<summary><strong>15. Ptrace Syscall</strong> (py3, expected: <code>runtime_error</code>)</summary>

```python
import ctypes
ctypes.CDLL(None).ptrace(0, 0, 0, 0)
```

`ctypes.CDLL(None)` loads libc. Calling `.ptrace(0, 0, 0, 0)` makes the `ptrace(2)` syscall (PTRACE_TRACEME). The seccomp BPF filter catches this syscall and applies `KILL_PROCESS`, which sends SIGKILL to the entire process group immediately.

`ptrace` is a primary sandbox-escape primitive; it allows one process to read and modify the memory and registers of another. Blocking it at the seccomp level ensures it cannot be used even if capabilities were somehow misconfigured.

**If it fails (returns `accepted`):** the `ptrace` syscall succeeded; the seccomp policy is not active. Check that `--seccomp_string` appears in the nsjail argv.

</details>

---

## File Layout

```
tests/sandbox/
├── main.go
└── programs/
    ├── cpu_stress.py
    ├── mem_alloc.py
    ├── file_write_large.py
    ├── tempfile_flood.py
    ├── fork_bomb.py
    ├── fork_bomb.c
    ├── proc_spawn.py
    ├── thread_flood.py
    ├── deep_recursion.py
    ├── output_bomb.py
    ├── network_connect.py
    ├── fs_read_passwd.py
    ├── fs_write_etc.py
    ├── escape_chroot.py
    └── escape_ptrace.py
```

Programs are embedded at compile time via `//go:embed`; no runtime file reads.

---

## Troubleshooting

<details>
<summary>The script says "service unreachable"</summary>

Run `make run` in another terminal and wait for it to print `listening on :8080`.

</details>

<details>
<summary>All probes return <code>internal_error</code></summary>

nsjail is not found or cannot run. The service must run inside Docker (`make run`), not natively on macOS.

</details>

<details>
<summary>Fork bomb probes return <code>accepted</code></summary>

The container is missing `seccomp:unconfined`. Without it, `clone3` (the modern `fork()` backing syscall) is blocked by Docker's default seccomp profile before nsjail even starts. Add it to `docker-compose.yml`.

</details>

<details>
<summary>Memory probe never triggers <code>memory_exceeded</code></summary>

cgroup v2 memory accounting is not active. Check that `/sys/fs/cgroup/memory.max` exists inside the container. Some Docker environments require `--cgroupns=host` or explicit memory limit settings on the container.

</details>

<details>
<summary>Network probe returns <code>runtime_error</code> for the wrong reason</summary>

If the timeout fires before `ENETUNREACH` (unusual), you would see `time_exceeded` rather than `runtime_error`. Either result confirms the connection was not made; the probe only fails on `accepted`.

</details>

---

<!-- nav-footer -->
<sub>[← Documentation index](README.md) · [API](api.md) · [Architecture](architecture.md) · [Concurrency](concurrency.md) · [Security](security.md) · [Languages](languages.md) · [Configuration](configuration.md)</sub>
