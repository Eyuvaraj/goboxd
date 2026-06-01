# goboxd

An HTTP service that compiles or interprets untrusted source code inside an `nsjail` sandbox,
runs it against test cases, and returns per-test results.
Built for the goboxd hackathon at Paradox IIT Madras 2026.

---

## Running

**Requirements:** Docker with Compose v2 · `nsjail` compiles from source at image build time. The container runs unprivileged with two added capabilities (`SYS_ADMIN`, `SYS_PTRACE`), `seccomp`/`systempaths` unconfined, and the cgroup-v2 hierarchy mounted read-write so nsjail can create per-job cgroups — all wired in `docker-compose.yml`.

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

| Method | Path             | Description               |
|--------|------------------|---------------------------|
| `POST` | `/run`           | Execute source code       |
| `GET`  | `/healthz`       | Liveness check            |
| `GET`  | `/readyz`        | Runtime readiness check   |
| `GET`  | `/info`          | Build info and live stats |
| `GET`  | [`/docs/`](http://localhost:8080/docs/)            | Interactive API reference |
| `GET`  | [`/playground/`](http://localhost:8080/playground/) | Browser test interface   |

Structurally valid requests always return **HTTP 200**. Execution outcomes live in the response body.
Validation failures return `400`; infrastructure errors return `5xx`.

---

## Languages

`py3` · `bash` · `js` · `c` · `cpp` · `java` · `verilog` · `ruby` · `lua` · `ocaml` · `go`

Adding a language takes one YAML block in [`configs/languages.yaml`](configs/languages.yaml)
and one `apt-get install` in the [`Dockerfile`](Dockerfile), with no Go code changes.

---

## Design

We use `chi` as the HTTP router because it wraps plain `net/http` handlers without abstraction overhead.
Each request runs inside an `nsjail` sandbox — namespaces, cgroups, seccomp, and filesystem isolation are handled by the kernel; goboxd only manages the job lifecycle.
Concurrency is a `chan struct{}` semaphore; requests queue under load, they never fail.
Languages live in YAML and the engine expands `{{source}}`, `{{artifact}}`, `{{flags}}` generically.
Adding a language is two file edits with no Go code change.

---

## Docs

| File | Contents |
|------|----------|
| [`docs/architecture.md`](docs/architecture.md) | Request lifecycle and concurrency |
| [`docs/security.md`](docs/security.md)         | Sandbox hardening                 |
| [`docs/languages.md`](docs/languages.md)       | Language registry and YAML schema |
| [`docs/benchmarks.md`](docs/benchmarks.md)     | Load-test results                 |
| [`docs/swagger.yaml`](docs/swagger.yaml)       | OpenAPI schema                    |
| [`docs/ai/prompts.md`](docs/ai/prompts.md)     | AI usage log                      |

---

> **Try it live:** run `make run` and open [**http://localhost:8080/playground/**](http://localhost:8080/playground/)
