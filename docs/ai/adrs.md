# Architecture Decision Records

Non-obvious design decisions made while building goboxd, with the context that
forced them and the consequences we accepted.

Format: **Context → Decision → Consequences**. Newest decisions are appended at
the bottom.

---

## ADR-1 — chi for routing, not gin/echo or bare net/http

**Context.** We needed routing, a couple of middlewares (body limit, structured
logging), and nothing else. The reference pyjail was a single handler. The real
question was how much framework to take on for a service with four endpoints.

**Decision.** Use `chi`. It is a thin layer over the standard library
`http.Handler` — no custom context, no reflection-based binding, no ORM-style
magic. Middleware is just `func(http.Handler) http.Handler`, which is exactly
what our `BodyLimit` and `StructuredLogger` already are.

**Consequences.** We keep stdlib-compatible handlers (easy to test with
`httptest`), get clean middleware composition, and avoid gin/echo's heavier
surface area that we would never use. The cost is one third-party dependency
where `net/http`'s 1.22 `ServeMux` could *almost* have done the job — we judged
the middleware ergonomics worth it. A team could swap chi out in an afternoon
because nothing depends on chi-specific types.

---

## ADR-2 — Counting semaphore with a bounded queue, NOT a worker pool or a priority scheduler

**Context.** The concurrency criterion is heavily weighted, so the temptation is
to build something that *looks* sophisticated: a worker-pool, a persistent job
queue, or a shortest-job-first priority scheduler that reorders pending requests
by predicted cost. The actual requirement is narrower: run at most *N* sandboxes
at once so we never oversubscribe CPU/memory on the host, and behave sanely when
more requests arrive than we can serve.

**Decision.** A buffered channel used as a counting semaphore
(`sem chan struct{}`, capacity = `MAX_CONCURRENT_JOBS`), plus a bounded wait
queue with optional load-shedding (`503 + Retry-After`) once the queue is full.
We explicitly rejected:
- a **worker pool** — it adds goroutine lifecycle management and a dispatch
  channel to achieve the identical invariant (≤ N concurrent jobs) that one
  semaphore already guarantees;
- a **persistent / priority queue** — FIFO admission is the *fair* default. A
  shortest-job-first scheduler optimizes mean latency for short jobs by
  penalizing long ones, which is a worse property for a code-judge where a slow
  compile is not lower-priority than a fast one. It also needs a cost model we
  cannot measure accurately before running the job, plus starvation-prevention
  aging to undo the unfairness it introduces. That is a lot of moving parts and
  failure modes bolted on to beat a semaphore on a metric the spec never asks
  for.

**Consequences.** The whole concurrency control is a few lines, trivially
correct, and easy to reason about under the race detector. Admission is FIFO and
fair. Latency at saturation grows predictably (our benchmarks show p95/p99
rising linearly while throughput plateaus — textbook bounded-queue behaviour).
We give up the ability to favour cheap jobs under load, which we consider a
feature, not a gap. If profiling ever showed head-of-line blocking hurting
throughput, the semaphore is the right place to evolve from — but we will not
pay that complexity speculatively.

---

## ADR-3 — Sequential test execution within a single job

**Context.** Each `POST /run` carries up to `MAX_TESTS` test cases. Running them
in parallel would cut wall-time for multi-test submissions.

**Decision.** Run a job's tests **sequentially**, reusing one workspace.

**Consequences.** No file races on the shared source/artifact, no fan-out of
sandboxes from a single request (which would let one request consume *N*
semaphore-equivalent resources and break the global concurrency bound), and a
deterministic, ordered result array. The cost is latency for submissions with
many tests — acceptable, because cross-*request* concurrency (the semaphore) is
where real throughput comes from, and parallel tests would undermine the very
resource accounting the semaphore exists to protect.

---

## ADR-4 — Languages are pure data: a YAML registry with no Go code per language

**Context.** The highest-weighted criterion is plug-and-play language support,
including adding a language live in under 30 minutes. Anything that requires a
Go code change, a recompile, or a switch statement per language fails that test.

**Decision.** Every language is one block in `configs/languages.yaml`
(build/run commands, args with `{{source}}`/`{{artifact}}`/`{{flags}}`
templates, per-step limits, flag allowlist, optional env). Adding a language is:
one YAML block + one `scripts/lang_install/<id>.sh` + a `/readyz` probe entry.
No Go changes, no recompile of the binary.

**Consequences.** The registry loader and validator are the only code that knows
about languages generically; the rest of the system treats a language as data.
This is what makes the 30-minute demo-day add credible. The trade-off is that
the YAML schema and template-expansion rules become a contract we must validate
strictly (hence registry validation + startup probes), because a malformed block
is now a config error instead of a compile error.

---

## ADR-5 — Always HTTP 200 for a structurally valid request; outcomes live in the body

**Context.** A submission whose code crashes, times out, or produces wrong
output is *not* an HTTP error — the service did its job correctly. Mapping user
code outcomes onto HTTP status codes is a common and costly conformance mistake.

**Decision.** `200` for any structurally valid request regardless of user-code
result. `400` only for malformed requests (bad JSON, unknown language, oversize
source, bad filename, disallowed flag, limit violations). `5xx` *only* for
genuine infrastructure failure (nsjail missing, disk full, sandbox setup error).
All per-test and top-level outcomes are status strings in the JSON body.

**Consequences.** The API contract is unambiguous and matches the spec's
semantics exactly. Clients distinguish "your request was bad" (4xx) from "your
code did something" (200 + status) from "we broke" (5xx) cleanly. The status
vocabulary becomes load-bearing, so it lives in exactly one place
(`internal/validate/status.go`) as the single source of truth.

---

## ADR-6 — Pure `[]string` argv everywhere; the shell is never invoked

**Context.** The pyjail reference was vulnerable to shell-metacharacter
injection. Filenames and flags come straight from the request body.

**Decision.** Every external program (nsjail, compilers, interpreters) is run via
`exec.CommandContext` with a pre-built `[]string` argv. No `sh -c`, no string
concatenation to form command lines, ever. All filesystem operations go through
`os.*`, not shell utilities.

**Consequences.** Shell injection is structurally impossible — there is no shell
in the path to inject into. This constraint is absolute and is enforced as a
project rule, not a preference. It occasionally costs convenience (e.g. we build
argv slices by hand instead of writing a one-line shell pipeline), which is the
correct trade.

---

## ADR-7 — `os.MkdirTemp` per job for workspace uniqueness

**Context.** Under concurrent load, two jobs must never share a workspace
directory or they can read each other's source. The pyjail UID-range approach
collides under concurrency.

**Decision.** Each job calls `os.MkdirTemp("", "goboxd-*")`, which atomically
creates a guaranteed-unique directory. Never derive a path from a counter, PID,
or RNG. A `defer ws.Cleanup()` follows immediately, and a startup
`SweepOrphans()` removes directories left by crashes.

**Consequences.** No collision is possible even at high concurrency, and the
kernel — not our code — guarantees uniqueness. Cleanup is defended on two layers
(the defer for the normal path, the sweep for crashes), so the disk does not
fill with stale jails.

---

## ADR-8 — Real `memory_peak_kb` from cgroup v2, not an estimate

**Context.** A useful judge reports actual peak memory. Estimating it from
rlimits or sampling is inaccurate and easy to game.

**Decision.** Run each sandbox in a per-job cgroup v2 slice
(`goboxd-<N>`) and read `memory.peak` after `cmd.Wait()`. The same accounting
detects `memory_exceeded`. Swap is disabled in the slice so the number is a true
RSS peak.

**Consequences.** `memory_peak_kb` is a kernel-reported figure, not a guess, and
memory-limit enforcement and reporting share one mechanism. The cost is a hard
dependency on cgroup v2 being available and writable inside the container, which
we surface in `/info` (cgroup accounting probe) rather than failing silently.

---

## ADR-9 — Two endpoints: strict spec-conformant `/run` and a raw/evaluator `/v1/run`

**Context.** The spec for `/run` is precise (fixed filenames, strict
compare/whitespace semantics, fixed status vocabulary). But during development
and demos we also wanted a freer mode for experimentation and evaluator-style
use without weakening the conformant endpoint.

**Decision.** Keep `/run` strictly to spec. Add `/v1/run` for raw execution and
an evaluator mode, so experimentation never tempts us to loosen the scored
endpoint.

**Consequences.** The conformance-critical path stays clean and easy to audit
against the contract, while the extra capability lives behind a separate route.
The cost is two handlers to maintain; we accept it to protect the scored
endpoint from scope creep.

---

## ADR-10 — Optional load-shedding (503 + Retry-After) instead of an unbounded queue

**Context.** A semaphore with an unbounded wait queue degrades gracefully until
memory runs out; under a flood it can accumulate arbitrarily many parked
goroutines and pending requests.

**Decision.** Bound the queue depth. When full, shed load with `503` +
`Retry-After` rather than queueing indefinitely. Load-shedding is configurable so
it can be disabled where pure queueing is preferred.

**Consequences.** The service has a defined, advertised saturation behaviour
instead of an open-ended one, which is the honest thing to do under sustained
overload. Queue-depth tracking is atomic so `/info` reports it accurately.

---

## ADR-11 — Kafel seccomp deny-list in the sandbox

**Context.** Namespace isolation alone leaves dangerous syscalls reachable
(`ptrace`, `bpf`, `io_uring`, kernel-module ops, clock manipulation).

**Decision.** Apply a Kafel seccomp policy in `sandbox/nsjail.go` that denies
those syscall classes for both build and run phases.

**Consequences.** Several escape and host-interference vectors are closed at the
syscall boundary, beyond the five required holes. The cost is maintaining the
policy as language toolchains evolve; we keep it a deny-list of clearly-dangerous
calls rather than an allow-list, to avoid breaking legitimate compiler/runtime
syscalls.

---

## ADR-12 — nsjail built from source as a pinned git submodule

**Context.** Distro packages of nsjail lag and vary; reproducibility matters for
a submission a judge will rebuild.

**Decision.** Vendor nsjail as a git submodule pinned at a release tag and
compile it in a dedicated Docker build stage.

**Consequences.** Every build uses the exact same nsjail, and the version is
known and reportable (nsjail has no `--version` flag, so we read it from the
pinned source for `/readyz` and `/info`). The cost is a longer cold build
(~5 min) and the submodule-init step the Makefile handles automatically.

---

## ADR-13 — Container-aware `GOMAXPROCS` and concurrency default

**Context.** Inside a CPU-limited container, Go's default `GOMAXPROCS` and a
naive `runtime.NumCPU()` see the host's core count, not the container's quota,
which oversubscribes the scheduler and the sandbox semaphore.

**Decision.** Set `GOMAXPROCS` from the cgroup CPU quota and default
`MAX_CONCURRENT_JOBS` to `config.AvailableCPUs()` (cgroup quota, falling back to
`runtime.NumCPU()`).

**Consequences.** A CPU-limited container neither starves nor oversubscribes
itself, and concurrency scales to the resources actually granted rather than the
host's. The default is overridable by env for operators who know better.
