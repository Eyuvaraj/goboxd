# goboxd

An HTTP service that accepts source code, compiles or interprets it inside an nsjail sandbox, runs it against test cases, and returns per-test results. Built for the goboxd hackathon at Paradox IIT Madras 2026.

HTTP routing uses [chi](https://github.com/go-chi/chi) because its handlers are plain `net/http`-compatible functions with no framework-specific context types, and middleware composition is explicit. Gin and Echo were considered but both require wrapping requests in framework types that complicate handler testing.

## Running

Docker with Compose v2 is the only prerequisite. nsjail is compiled from source inside the build stage.

```
make build        # build the image (~5 min first time; nsjail 3.6 compiled from source)
make run          # start the service on :8080
make test         # unit tests (no Docker required)
make integration  # end-to-end tests (requires make run in another terminal)
make lint         # golangci-lint
make load         # load-test benchmark (requires hey or k6 in PATH)
```

The container must run with `--privileged` (already set in docker-compose.yml) for nsjail namespace support.

**Sandbox:** Each job runs inside [nsjail](https://github.com/google/nsjail) — a Linux process isolation tool that uses namespaces, cgroups, and `seccomp` to confine untrusted code. [Kafel](https://github.com/google/kafel) is used to define the seccomp policy that blocks dangerous syscalls (`ptrace`, `bpf`, `io_uring`, clock manipulation, kernel module ops) while allowing only what compilers and interpreters need.

Once the service is up, open `http://localhost:8080/playground/` to try it interactively.

## API

| Method | Path | Description |
|--------|------|-------------|
| `GET`  | `/healthz` | Liveness — 200 if the process is up |
| `GET`  | `/readyz`  | Readiness — probes nsjail and every language binary |
| `GET`  | `/info`    | Build info, language list, limits, live stats |
| `POST` | `/run`     | Execute source code against test cases |

HTTP 200 is returned for all structurally valid requests. Execution outcomes are in the response body. Only 400 (validation), 500 (server fault), and 503 (cancelled) are HTTP-level errors. See [docs/swagger.yaml](docs/swagger.yaml) for the full schema.

## Languages

| Language | ID | Type |
|---|---|---|
| Python 3 | `py3` | required |
| Bash | `bash` | required |
| JavaScript (Node) | `js` | required |
| C | `c` | required |
| C++ | `cpp` | required |
| Java | `java` | required |
| Verilog | `verilog` | required |
| Ruby | `ruby` | bonus |
| Lua | `lua` | bonus |
| Rust | `rust` | bonus |
| Kotlin | `kotlin` | bonus |
| OCaml | `ocaml` | bonus |
| Go | `go` | bonus |

Adding a language is one YAML block in `configs/languages.yaml` plus a toolchain install in the Dockerfile. No Go code change. See [docs/languages.md](docs/languages.md).

## Docs

- [docs/swagger.yaml](docs/swagger.yaml) — full API schema (Swagger/OpenAPI)
- [docs/architecture.md](docs/architecture.md) — package layout, request lifecycle, concurrency model
- [docs/security.md](docs/security.md) — seven security holes, their fixes, and seccomp hardening
- [docs/languages.md](docs/languages.md) — language registry schema and demo-day add flow
- [docs/benchmarks.md](docs/benchmarks.md) — load-test results at 1/10/50/100 concurrent clients

The playground at `/playground/` is a browser UI for writing and running code against the live service — useful for manual testing and demo day.

## License

GPL v3. See [LICENSE](LICENSE).
