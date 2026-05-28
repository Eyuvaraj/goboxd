# Architecture

`goboxd` is a lightweight HTTP service that securely accepts untrusted source code, compiles or interprets it inside an `nsjail` sandbox, and returns per-test execution results.

---

## Request Lifecycle

```mermaid
flowchart TD
    Req[POST /run] --> MW[Middleware Stack]

    subgraph Middleware Stack
        MW_RID[RequestID: Generate/Propagate]
        MW_Rec[Recoverer: Catch panics]
        MW_Body[BodyLimit: MaxBytesReader]
        MW_Log[StructuredLogger: JSON log line]

        MW_RID --> MW_Rec --> MW_Body --> MW_Log
    end

    MW --> Handler[handler/run.go]

    subgraph Handler Validation
        H_Parse[Decode JSON body]
        H_Lang[Validate language]
        H_Size[Validate sizes]
        H_Files[Validate filenames]
        H_Flags[Validate flags]
        H_Limits[Validate limit overrides]

        H_Parse --> H_Lang --> H_Size --> H_Files --> H_Flags --> H_Limits
    end

    Handler --> Runner[runner.Submit]

    subgraph Runner Execution
        R_Sem[Acquire semaphore slot]
        R_WS[sandbox.NewWorkspace: MkdirTemp]
        R_Def[Defer ws.Cleanup]

        R_Sem --> R_WS --> R_Def

        R_Def --> R_Comp[job.compile]
        R_Comp -.-> |Interpreted| R_Ok[Return OK]
        R_Comp --> R_CompRun[sandbox.Run: Build]

        R_CompRun --> R_Test[job.runTests]
        R_Test --> R_TestRun[sandbox.Run: per-test]

        R_TestRun --> R_Rel[Release semaphore slot]
    end

    Runner --> Resp[Aggregate status → JSON 200]
```

---

## Package Layout

- **`cmd/goboxd/`** — entry point; wires config → registry → probe cache → runner → chi router.
- **`internal/config/`** — environment variable parsing, `LanguageDef` and `LimitsDef` types.
- **`internal/registry/`** — YAML loading, language lookup (`Get`/`All`), startup validation, 30s TTL probe cache.
- **`internal/validate/`** — filename, flag, source-size, stdin-size, limit, and test-count validation. Single source of truth for all status constant strings.
- **`internal/sandbox/`** — nsjail argv builder, workspace (`MkdirTemp`), limits merge, output capping, status parsing.
- **`internal/runner/`** — bounded semaphore (`chan struct{}`), job lifecycle (compile + runTests), status aggregation.
- **`internal/handler/`** — `/run`, `/healthz`, `/readyz`, `/info` handlers; `BodyLimit` and `StructuredLogger` middleware.
- **`internal/stats/`** — atomic counters for in-flight jobs, queue size, totals, and internal error tracking.
- **`internal/logctx/`** — typed context key for per-request log fields written by the handler and read by the middleware after `ServeHTTP`.
- **`internal/playground/`** — embeds the playground SPA (`index.html`) served at `/playground/`.
- **`configs/languages.yaml`** — all 13 language definitions (7 required + 6 bonus).
- **`tests/integration/`** — end-to-end tests (build tag: `integration`).

---

## Concurrency Model

`goboxd` uses a buffered channel as a counting semaphore.

- **Capacity** — `MAX_CONCURRENT_JOBS` (default: `runtime.NumCPU()`).
- **Behaviour** — `runner.Submit()` blocks on send until a slot is free; the slot is released on function return. Requests queue, they never fail due to backpressure.
- **Why not a worker pool** — a pool requires a persistent goroutine per slot and a job channel. A semaphore is simpler: each request goroutine drives its own job and blocks on the semaphore. The effect on throughput is identical.
- **Per-test execution is sequential** within a job. Parallel execution was considered and rejected: nsjail process startup (not goroutines) is the bottleneck, sequential execution gives deterministic file layout, and it avoids workspace races.

---

## Adding a New Language

No Go code change is required when the language fits the existing templates (`{{source}}`, `{{artifact}}`, `{{flags}}`).

1. Add an entry to `configs/languages.yaml`.
2. Add `apt-get install` for the toolchain in the Dockerfile's runtime stage.
3. `make build` — the registry loads at startup and `/readyz` reports the new language automatically.

---

## Security Model

Seven known vulnerabilities from the reference implementation are closed. Full details: [security.md](security.md).

| Layer | Protection |
|-------|-----------|
| HTTP middleware | `MaxBytesReader` body cap before JSON parse |
| Handler | Filename validation, flag allowlist, source/stdin/expected-size caps, limit cap |
| Workspace | `os.MkdirTemp` (atomic, no collision), pure-Go cleanup, startup orphan sweep |
| Sandbox | `io.LimitReader` output cap, pure `[]string` argv (no shell), seccomp deny-list |
| Runner | `defer ws.Cleanup()` on every exit path |

---

## `nsjail` Invocation

`goboxd` invokes `nsjail` as a pure `[]string` slice — no shell at any point. Below is a representative invocation for running a compiled C++ binary:

```
/usr/local/bin/nsjail
  --mode o
  --chroot /tmp/goboxd/goboxd-<id>
  --user 65534 --group 65534
  --log_fd 3
  --max_cpus 1
  --rw
  --cwd /
  --hostname goboxd
  --detect_cgroupv2
  --cgroupv2_mount /sys/fs/cgroup
  --cgroup_mem_parent goboxd-<timestamp>
  --rlimit_nofile 1000
  --rlimit_core 0
  --rlimit_stack 8
  --env TMP=/ --env TMPDIR=/ --env HOME=/
  --env PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
  --time_limit 5   --rlimit_cpu 5
  --cgroup_mem_max 268435456   --cgroup_mem_swap_max 0
  --rlimit_as 4096
  --cgroup_pids_max 64   --rlimit_nproc 64
  --rlimit_fsize 100
  -R /bin -R /usr -R /lib -R /etc -R /dev -R /var
  --seccomp_string 'POLICY goboxd_safe { KILL_PROCESS { ptrace, bpf, mount, ... } } USE goboxd_safe DEFAULT ALLOW'
  --
  /solution
```

Key design points:
- `--rlimit_as` is set to `max(memory_kb × 4 / 1024, 4096)` MiB. The JVM pre-allocates ~1 GiB of virtual address space at startup, so the floor prevents false OOM kills on Java and Kotlin.
- `--cgroup_mem_max` is the real RSS guard. After `cmd.Wait()`, `memory.peak` is read from the cgroup path to populate `memory_peak_kb` in the response.
- `--cgroup_mem_swap_max 0` disables swap so memory limits are exact.
- The nsjail log is captured on fd 3. `ParseBuildStatus` and `ParseRunStatus` read it to distinguish `internal_error` (`[E][` prefix lines) from normal exit codes.
