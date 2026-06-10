![CI](https://github.com/Eyuvaraj/goboxd/actions/workflows/ci.yml/badge.svg)

# goboxd

An HTTP service that compiles or interprets untrusted source code inside an `nsjail` sandbox, runs it against test cases, and returns per-test results. Built for the goboxd hackathon at Paradox IIT Madras 2026.

---

## Running

**Requires:** Docker with Compose v2. `nsjail` is compiled from source at image-build time. The container runs unprivileged with two added capabilities (`SYS_ADMIN`, `SYS_PTRACE`), `seccomp`/`systempaths` unconfined, and the cgroup-v2 hierarchy mounted read-write (all wired in [`docker-compose.yml`](docker-compose.yml)).

```bash
make build        # build the image (~5 min cold)
make run          # start service on :8080
make test         # unit tests (no Docker needed)
make integration  # end-to-end tests (requires make run)
make lint         # golangci-lint
make load         # load benchmarks (requires hey or k6)
```

---

## API

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/run` | Execute source code against test cases |
| `GET`  | `/healthz` | Liveness check |
| `GET`  | `/readyz` | Readiness: nsjail + every language probe |
| `GET`  | `/info` | Build info and live stats |
| `GET`  | [`/docs/`](http://localhost:8080/docs/) | Interactive API reference |
| `GET`  | [`/playground/`](http://localhost:8080/playground/) | Browser test interface |

- Structurally valid requests always return **HTTP 200**; the outcome is in the body `status`.
- Request errors return **400**. Only infrastructure failures return **5xx**, never user-code failures.

---

## Languages

`py3` · `bash` · `js` · `c` · `cpp` · `java` · `verilog` · `ruby` · `lua` · `ocaml` · `go`

Adding one is two edits: a YAML block in [`configs/languages.yaml`](configs/languages.yaml) and an `apt-get install` line in the [`Dockerfile`](Dockerfile). No Go code changes.

## Design

`chi` was chosen over `gin` and `echo` because its handlers are plain `http.Handler` values: no custom context types, no interface lock-in, full stdlib-middleware compatibility. `gorilla/mux` was the other candidate but has been unmaintained since 2022.

- Each request runs in an `nsjail` sandbox; the kernel enforces namespaces, cgroups, seccomp, and filesystem isolation; goboxd only manages the job lifecycle.
- Concurrency is a `chan struct{}` semaphore: requests queue under load, they never fail.
- Languages are declarative; the engine expands `{{source}}`, `{{artifact}}`, and `{{flags}}` generically.

## Docs

| File | Contents |
|------|----------|
| [`docs/features.md`](docs/features.md) | Feature overview and documentation index |
| [`docs/api.md`](docs/api.md) | Endpoints, request/response schema, status vocabulary |
| [`docs/architecture.md`](docs/architecture.md) | Request lifecycle and package layout |
| [`docs/concurrency.md`](docs/concurrency.md) | Semaphore, queueing, and load behaviour |
| [`docs/security.md`](docs/security.md) | Sandbox hardening (7 holes + seccomp) |
| [`docs/languages.md`](docs/languages.md) · [`adding-a-language.md`](docs/adding-a-language.md) | Language registry, YAML schema, and the add-a-language runbook |
| [`docs/configuration.md`](docs/configuration.md) | Environment variables and container runtime requirements |
| [`docs/ci-cd.md`](docs/ci-cd.md) | CI pipeline, gosec, govulncheck, supply chain |
| [`docs/benchmarks.md`](docs/benchmarks.md) · [`docs/swagger.yaml`](docs/swagger.yaml) | Load-test results · OpenAPI schema |
| [`docs/ai/`](docs/ai/) | AI usage log and decision records |
