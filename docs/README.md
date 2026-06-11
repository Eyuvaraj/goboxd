# goboxd documentation

A map of what goboxd does and where each area is documented in depth. New here? Skim this page, then jump to the file you need — the full index is at the bottom.

---

## Execution

| Feature | Notes |
|---------|-------|
| `POST /run` | Strict competition contract. Compiles (if needed) and runs untrusted source against test cases inside an nsjail sandbox. |
| `POST /v1/run` | Superset: adds `exit_code`, raw mode (no grading), and evaluator mode (custom grader). |
| 15 languages | 7 required + 8 additional, all data-driven from one YAML file. See [languages.md](languages.md). |
| Per-request overrides | Build/run `flags` (validated against a per-language allow-list) and `limits` (capped at the language maximum). |
| Exact status vocabulary | `accepted`, `wrong_output`, `output_whitespace_mismatch`, `time_exceeded`, `memory_exceeded`, `runtime_error`, `build_failed`, `not_executed`, `internal_error`. Deterministic aggregation rule. See [api.md](api.md). |

---

## Language Registry

- Every language is one block in `configs/languages.yaml`: build/run commands, argument templates (`{{source}}`, `{{artifact}}`, `{{flags}}`), per-phase limits, flag allow-list, optional env and bind mounts.
- Adding a language requires one YAML block and one install script, with no Go code change.
- Strict startup validation: malformed entries fail loudly, unknown YAML keys are rejected.
- `/readyz` reflects the registered set automatically.

Step-by-step runbook: [adding-a-language.md](adding-a-language.md)

---

## Sandboxing and Security

- nsjail (built from source, pinned at tag 3.4) with Linux namespaces (PID, mount, network, UTS, IPC, user), cgroup v2 resource limits, and a Kafel seccomp deny-list.
- All seven reference vulnerabilities closed, plus seccomp filtering as an additional layer.
- A 15-probe adversarial containment suite proves the sandbox actually holds.

Full write-up: [security.md](security.md) | Probe documentation: [sandbox-tests.md](sandbox-tests.md)

---

## Concurrency and Load

- A bounded counting semaphore caps in-flight jobs at the CPU count by default.
- Excess requests queue fairly (FIFO) rather than failing.
- Optional load shedding via `MAX_QUEUE_DEPTH`: returns `503 + Retry-After` when the wait queue is full.
- Benchmarked at 1/10/50/100 concurrent clients.

Deep dive and tuning guide: [concurrency.md](concurrency.md) | Numbers: [benchmarks.md](benchmarks.md)

---

## Health and Observability

| Endpoint | Behaviour |
|----------|-----------|
| `GET /healthz` | Liveness: `200` if the process is up |
| `GET /readyz` | Readiness: `200` only if nsjail and every language pass a smoke probe; `503` with per-language breakdown otherwise. Cached with a 30 s TTL. |
| `GET /info` | Build info, nsjail path/version, full language list with versions and default limits, service limits, and live stats (in-flight jobs, queue size, totals, error count, disk free) |
| Structured JSON logs | One line per request: request ID, method, path, status, duration, language, exec status, build time, test counts |

---

## Tooling and Delivery

- **Reproducible Docker build**: multi-stage (nsjail-builder, Go builder, lean runtime). nsjail and Kafel are pinned git submodules.
- **CI/CD**: unit tests, govulncheck, image build, lint (including gosec), and live integration tests on every push and PR; a manual benchmarks workflow.
- **Interactive playground** at `/playground/` and **Swagger UI** at `/docs/`.
- **Makefile**: `build`, `run`, `test`, `integration`, `lint`, `load`, `swagger`.

See [ci-cd.md](ci-cd.md)

---

## Configuration

Every setting is an environment variable: port, paths, size caps, concurrency, queue depth, orphan-sweep age. Full table with defaults and container privilege requirements: [configuration.md](configuration.md)

---

## Documentation Index

| File | Contents |
|------|----------|
| [api.md](api.md) | Endpoints, request/response schema, status vocabulary, error codes |
| [architecture.md](architecture.md) | Request lifecycle, package layout, nsjail invocation |
| [concurrency.md](concurrency.md) | Semaphore mechanics, queueing, load shedding, tuning guide |
| [security.md](security.md) | 7 vulnerability fixes, seccomp policy, verification |
| [sandbox-tests.md](sandbox-tests.md) | Adversarial probe suite: 15 attack programs |
| [languages.md](languages.md) | Language catalog and YAML schema |
| [adding-a-language.md](adding-a-language.md) | Step-by-step runbook for adding a language |
| [configuration.md](configuration.md) | Environment variables and container runtime requirements |
| [ci-cd.md](ci-cd.md) | CI pipeline, gosec, govulncheck, supply chain |
| [benchmarks.md](benchmarks.md) | Load-test results at 1/10/50/100 concurrent clients |
| [swagger.yaml](swagger.yaml) | OpenAPI spec |
| [ai/](ai/) | AI usage log and decision records |
