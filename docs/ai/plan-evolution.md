# Plan Evolution

How the design and our understanding of the problem changed over the build. Each
entry is *what we expected → what we actually did → why it changed*. Dates track
the real commit history.

---

## 2026-05-23 — Build everything thin, end to end, in one pass

**Expected.** Stand up the full skeleton (config, registry, handler, runner,
sandbox, validate, stats, health) in one session and iterate.

**Actual.** Did exactly that — the first working version of all main packages
plus `configs/languages.yaml` for the seven required languages and a first set of
integration tests landed together.

**Why.** For a systems challenge, the architecture only proves itself end to end.
Getting a request to flow all the way to nsjail and back early exposed the real
problems (sandbox pathing, status parsing) far faster than building one package
to perfection in isolation. We deliberately took a Prometheus-metrics suggestion
*off* the table here as overkill.

---

## 2026-05-24 — Security and isolation are the hard part, not the API

**Expected.** Close the listed pyjail holes with input validation and move on.

**Actual.** Closed the validation holes, then went further: upgraded nsjail,
moved to cgroup v2, and added a Kafel seccomp deny-list. Bonus languages went in
the same day.

**Why.** Once the request flowed end to end, it was clear the scoring risk was
not "does the API match" but "can untrusted code break out or starve the host."
That reframed security from a checklist into the central engineering problem, and
pulled cgroup v2 / seccomp forward from "nice to have" to core.

---

## 2026-05-28 — Compiled languages broke our sandbox assumptions

**Expected.** The same nsjail invocation that runs an interpreter would run a
compiled artifact.

**Actual.** Spent real effort fixing the sandbox for compiled languages —
chroot, bind mounts, and `/dev` mounts that interpreters never needed but
compilers and their produced binaries do. Same window: added the interactive
playground SPA and wired cgroup v2 memory tracking.

**Why.** Interpreters run from a single file; compilers need their toolchain,
linker, and shared libraries reachable inside the jail, and the produced binary
needs a working `/dev`. The interpreter-shaped mental model was simply
incomplete. This is the single biggest "the plan was wrong" moment of the build.

---

## 2026-05-29 → 05-30 — Registry has to defend itself

**Expected.** Load the YAML, trust it, run.

**Actual.** Hardened registry validation and added startup probes; added
`probe_args` to the config and fixed version-string probes (lua, verilog) that
returned junk.

**Why.** Once languages are pure data (ADR-4), a bad block is a runtime failure
instead of a compile error. The 30-minute demo-day add is only credible if a
malformed addition fails *loudly at startup* via `/readyz`, not silently at the
first request. Validation moved from afterthought to part of the registry's
contract.

---

## 2026-05-31 — Two modes: protect the spec, allow experimentation

**Expected.** One `/run` endpoint serves every purpose.

**Actual.** Kept `/run` strictly spec-conformant and added `/v1/run` for raw
execution and an evaluator mode. Same window: real `memory_peak_kb` from cgroup
v2, real nsjail version reporting, optional load-shedding (503 + Retry-After),
and a periodic orphan sweep + `ReadHeaderTimeout`.

**Why.** We kept wanting a freer execution mode for demos and evaluation, and
every time we considered loosening `/run` to get it we were weakening the scored
endpoint. Splitting the route removed that pressure permanently (ADR-9). The
load-shedding decision came from realizing an unbounded wait queue has no honest
saturation story (ADR-10).

---

## 2026-06-01 — Docker build is a deliverable, not a detail

**Expected.** Add languages by editing the Dockerfile inline.

**Actual.** Moved language installation into per-language `scripts/lang_install/`
scripts, parallelized the nsjail build, consolidated apt layers, and added cache
mounts. Churned on Rust and Kotlin (commented out, re-enabled, script removed)
before settling.

**Why.** The Dockerfile *is* part of the plug-and-play story — adding a language
should touch one install script, not a growing monolithic RUN line. The
Rust/Kotlin churn taught us to treat a language as fully done only when its
install script, YAML block, `/readyz` probe, and an integration test all agree;
half-added languages are worse than absent ones (hence Kotlin is cleanly skipped,
not left half-wired).

---

## 2026-06-02 → 06-09 — Make the claims verifiable

**Expected.** The code being correct is enough.

**Actual.** Added CI (lint, test, build, vuln-check, gosec), a benchmarks
workflow, an adversarial sandbox containment test suite (`tests/sandbox/`) with
real attack programs, stderr output capping, atomic queue-depth tracking,
context-cancellation handling in job execution, and three more languages (Perl,
AWK, TypeScript).

**Why.** A judge does not take "it's secure" or "it's concurrent" on faith. The
adversarial suite turns the security claims into runnable evidence (fork bombs,
chroot/ptrace escape attempts, network attempts, output/memory bombs), CI turns
"it builds" into a green check, and the benchmark workflow turns the concurrency
claim into numbers. The plan's final phase shifted from *building* capability to
*proving* it.

---

## Threads that ran the whole way through

- **Resist sophistication-for-its-own-sake.** We repeatedly declined to build a
  worker pool, a persistent/priority queue, metrics, or auth. Each time the
  question was "does the spec or the threat model need this," and each time the
  honest answer was no (see ADR-2). The discipline of *not* building was as
  important as what we built.
- **End-to-end before perfect.** Every major capability was wired through the
  full request path early and hardened later, not the reverse.
- **Make every claim runnable.** Security → adversarial tests; concurrency →
  benchmarks; "it builds" → CI; "the language registry works" → startup probes
  and per-language integration tests.
