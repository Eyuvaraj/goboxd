# Architecture

goboxd is a small HTTP service that accepts untrusted source code, compiles or interprets it inside an nsjail sandbox, and returns per-test execution results.

## Request lifecycle

```
POST /run
  │
  ├─ middleware stack
  │    ├─ RequestID         generate / propagate X-Request-Id
  │    ├─ Recoverer         catch panics, log, return 500
  │    ├─ BodyLimit         http.MaxBytesReader — 413 before any parsing
  │    └─ StructuredLogger  one JSON log line per request when done
  │
  ├─ handler/run.go
  │    ├─ decode JSON body (400 on bad JSON)
  │    ├─ validate language (400 unknown)
  │    ├─ validate source size (400 too large)
  │    ├─ validate filenames (400 traversal / bad chars)
  │    ├─ validate flags against per-language allowlist (400 disallowed)
  │    └─ validate test count and stdin sizes (400)
  │
  ├─ runner.Submit(ctx, req)
  │    ├─ acquire semaphore slot (blocks if at max_concurrent_jobs)
  │    ├─ sandbox.NewWorkspace() — os.MkdirTemp, atomic
  │    ├─ defer ws.Cleanup()
  │    │
  │    ├─ job.compile(ctx)
  │    │    ├─ (interpreted: return {status:"ok"} immediately)
  │    │    ├─ os.WriteFile(source)
  │    │    ├─ sandbox.Run() — nsjail argv as []string, no shell
  │    │    └─ parse exit code + stderr → build.status
  │    │
  │    └─ job.runTests(ctx, buildStatus)
  │         ├─ (build failed: all tests → not_executed)
  │         ├─ for each test:
  │         │    ├─ os.WriteFile(stdin)
  │         │    ├─ sandbox.Run() — io.LimitReader on stdout
  │         │    └─ compare stdout → accepted / whitespace_mismatch / wrong_output / ...
  │         └─ release semaphore slot
  │
  └─ aggregate top-level status → JSON 200
```

## Package layout

```
cmd/goboxd/         entry point — wire config, registry, runner, router
internal/
  config/           ServerConfig (env vars), LanguageDef YAML schema
  registry/         load + validate YAML, language lookup, readiness probes
  validate/         filename rules, flag allowlist, size limits, status constants
  sandbox/          nsjail argv builder, workspace (tempdir), output capping
  runner/           bounded semaphore, job lifecycle, status aggregation
  handler/          HTTP handlers (/run, /healthz, /readyz, /info), middleware
  stats/            atomic counters for /info
configs/
  languages.yaml    all language definitions — no Go change to add a language
tests/integration/  end-to-end tests (build tag: integration)
scripts/
  lang_install/     per-language toolchain install scripts
  load_test.sh      hey / k6 benchmark
docs/               api, languages, security, benchmarks, architecture
```

## Concurrency model

A single buffered channel of `struct{}` (capacity = `MAX_CONCURRENT_JOBS`, default `runtime.NumCPU()`) acts as the semaphore. `runner.Submit()` blocks on a channel receive until a slot is available, then returns the slot when the job finishes. This means:

- Requests queue rather than fail when the service is at capacity.
- The in-flight count stays bounded regardless of request rate.
- Each slot corresponds to exactly one live nsjail process (for compiled languages) or one set of nsjail test processes.

Per-test execution is sequential within a job. Parallel test execution was considered but rejected: sequential is safer (deterministic file layout, no race on workspace files), and the limiting factor is nsjail process startup, not Go goroutines.

## Adding a language

1. Add one entry to `configs/languages.yaml` (see `docs/languages.md` for the schema).
2. Add `scripts/lang_install/<language>.sh` to install the toolchain in the Docker image.
3. Add the toolchain install call to `Dockerfile` (or the install script to the `RUN` block).
4. Rebuild the image. `/readyz` and `/info` will reflect the new language automatically.

No Go code change is required unless the language needs custom argument expansion logic beyond `{{source}}`, `{{artifact}}`, and `{{flags}}`.

## Security model

See `docs/security.md` for the full breakdown of the seven holes from the reference and where each is fixed. The short summary:

| Layer | What is protected |
|-------|-------------------|
| HTTP middleware | request body size (hole #4 partial) |
| handler/run.go | filename validation (#1), flag allowlist (#3), source/stdin sizes (#4) |
| sandbox/workspace.go | atomic tempdir (#5), pure-Go cleanup (#2), startup orphan sweep (#7) |
| sandbox/nsjail.go | output cap (#6), pure argv slice — no shell (#2) |
| runner/runner.go | deferred cleanup on every exit path (#7) |

## nsjail invocation

goboxd always calls nsjail as a pure `[]string` argv through `os/exec`. A representative build invocation for C++:

```
/usr/local/bin/nsjail
  --mode o
  --chroot /tmp/goboxd/goboxd-1234567890
  --user 65534 --group 65534
  --log_fd 3
  --disable_clone_newnet
  --max_cpus 1
  --time_limit 10
  --rlimit_as 512         (memory_kb / 1024)
  --rlimit_nproc 100
  --rlimit_fsize 100
  -R /usr/bin -R /lib -R /lib64 -R /usr/lib
  --
  /usr/bin/g++ -O2 -o solution solution.cpp
```

No shell, no string interpolation into a shell command, no `exec.Command("sh", "-c", ...)`.
