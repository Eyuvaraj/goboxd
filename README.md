# goboxd

goboxd is an HTTP service that accepts source code, compiles or interprets it inside an [nsjail](https://github.com/google/nsjail) sandbox, runs it against test cases, and returns per-test execution results. Each job runs in a Linux namespace jail with cgroup resource limits, seccomp syscall filtering, and a [Kafel](https://github.com/google/kafel) policy that blocks dangerous syscalls (`ptrace`, `bpf`, `io_uring`, clock manipulation, kernel module ops).

HTTP routing uses [chi](https://github.com/go-chi/chi) because its handlers are plain `net/http`-compatible functions with no framework-specific context types, and middleware composition is explicit. Gin and Echo were considered but both require wrapping requests in framework types that complicate handler testing.

## Running

Docker with Compose v2 is the only prerequisite. nsjail is compiled from source inside the build stage; the container must run with `--privileged` (set in docker-compose.yml) for namespace support.

```
make build        # build the image (~5 min first time; nsjail compiled from source)
make run          # start the service on :8080
make test         # unit tests (no Docker required)
make integration  # end-to-end tests (requires make run in another terminal)
make lint         # golangci-lint
make load         # load-test benchmark (requires hey or k6 in PATH)
```

Open `http://localhost:8080/playground/` once the container is running for an interactive browser UI.

## API

| Method | Path | Description |
|--------|------|-------------|
| `GET`  | `/healthz` | Liveness — 200 if the process is up |
| `GET`  | `/readyz`  | Readiness — probes nsjail and every language binary |
| `GET`  | `/info`    | Build info, language list, limits, live stats |
| `POST` | `/run`     | Execute source code against test cases |

HTTP 200 for all valid requests; execution outcomes in the response body. 400 for validation errors, 5xx only for server faults. See [docs/swagger.yaml](docs/swagger.yaml) for the full schema.

## Languages

| Language | ID |
|---|---|
| Python 3 | `py3` |
| Bash | `bash` |
| JavaScript (Node) | `js` |
| C | `c` |
| C++ | `cpp` |
| Java | `java` |
| Verilog | `verilog` |
| Ruby | `ruby` |
| Lua | `lua` |
| OCaml | `ocaml` |

Adding a language is one YAML block in `configs/languages.yaml` plus a toolchain install in the Dockerfile. No Go code change. See [docs/languages.md](docs/languages.md).

## Docs

- [docs/swagger.yaml](docs/swagger.yaml) — full API schema (Swagger/OpenAPI)
- [docs/architecture.md](docs/architecture.md) — package layout, request lifecycle, concurrency model
- [docs/security.md](docs/security.md) — seven security holes, their fixes, and seccomp hardening
- [docs/languages.md](docs/languages.md) — language registry schema and demo-day add flow
- [docs/benchmarks.md](docs/benchmarks.md) — load-test results at 1/10/50/100 concurrent clients

## License

GPL v3. See [LICENSE](LICENSE).
