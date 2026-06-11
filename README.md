# goboxd

An HTTP service that compiles or interprets untrusted source code inside an `nsjail` sandbox, runs it against test cases, and returns per-test results. Built for the goboxd hackathon at Paradox IIT Madras 2026.

---

## Running

**Requires:** Docker with Compose v2. `nsjail` is compiled from source at image-build time. The container runs unprivileged with two added capabilities (`SYS_ADMIN`, `SYS_PTRACE`), `seccomp`/`systempaths` unconfined, and the cgroup-v2 hierarchy mounted read-write (all wired in [`docker-compose.yml`](docker-compose.yml)).

```bash
make build        # build the image (~10 min cold)
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

**In-Scope:** `py3` · `bash` · `js` · `c` · `cpp` · `java` · `verilog`

**Additional:** `ruby` · `lua` · `ocaml` · `rust` · `kotlin` · `go`

Adding a language requires:

1. A definition in [`configs/languages.yaml`](configs/languages.yaml)
2. An installer in [`scripts/lang_install/`](scripts/lang_install/)

No Go code changes are required.

See: [`docs/adding-a-language.md`](docs/adding-a-language.md)

## Design

`chi` was chosen over `gin` and `echo` because its handlers are plain `http.Handler` values: no custom context types, no interface lock-in, full stdlib-middleware compatibility.

- Each request runs in an `nsjail` sandbox; the kernel enforces namespaces, cgroups, seccomp, and filesystem isolation; goboxd only manages the job lifecycle.
- Concurrency is a `chan struct{}` semaphore: requests queue under load, they never fail.
- Languages are declarative; the engine expands `{{source}}`, `{{artifact}}`, and `{{flags}}` generically.

## Docs

* [Full documentation](https://drive.google.com/file/d/1MKdEqvTQ-d0YkdGD02Hrn8tt5qer3SYR/view?usp=sharing)
* [`docs/api.md`](docs/api.md): API contract
* [`docs/architecture.md`](docs/architecture.md): system architecture
* [`docs/security.md`](docs/security.md): sandboxing and security model
* [`docs/concurrency.md`](docs/concurrency.md): concurrency and load handling
* [`docs/languages.md`](docs/languages.md): language configuration
* [`docs/benchmarks.md`](docs/benchmarks.md): benchmark results

* [`docs/`](docs/): documentation index
* [`docs/ai/`](docs/ai/): AI usage log and decision records