# goboxd

An HTTP service that accepts source code, compiles or interprets it inside an nsjail sandbox, runs it against test cases, and returns per-test results. Built in Go for Paradox 2026.

HTTP routing uses [chi](https://github.com/go-chi/chi) because its handlers are plain `net/http`-compatible functions with no framework-specific context types, and middleware composition is explicit. gin and echo were considered but both require wrapping the request in framework types that make handler testing and future router changes unnecessarily complicated.

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

Verify it is up:

```
curl http://localhost:8080/healthz
```

The container must run with `--privileged` (already set in docker-compose.yml) for nsjail namespace support. nsjail will not run on macOS or Windows directly; use Docker Desktop or a Linux VM.

## API

| Method | Path | Description |
|--------|------|-------------|
| `GET`  | `/healthz` | Liveness — 200 if the process is up |
| `GET`  | `/readyz`  | Readiness — probes nsjail and every language binary |
| `GET`  | `/info`    | Build info, language list, limits, live stats |
| `POST` | `/run`     | Execute source code against test cases |

HTTP 200 is returned for all structurally valid requests. Execution outcomes are in the response body. Only 400 (validation), 500 (server fault), and 503 (cancelled) are HTTP-level errors. See [docs/api.md](docs/api.md) for the full schema.

## Languages

In-scope: `py3`, `bash`, `js`, `c`, `cpp`, `java`, `verilog`.  
Bonus: `ruby`, `lua`, `rust`, `kotlin`, `ocaml`.

Adding a language is one YAML block in `configs/languages.yaml` plus a toolchain install in the Dockerfile. No Go code change. See [docs/languages.md](docs/languages.md).

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP listen port |
| `NSJAIL_PATH` | `/usr/local/bin/nsjail` | nsjail binary path |
| `JAIL_DIR` | `/tmp/goboxd` | Sandbox workspace root |
| `LANGUAGE_FILE` | `/etc/goboxd/languages.yaml` | Language registry |
| `MAX_SOURCE_BYTES` | `262144` | Source size cap |
| `MAX_TESTS` | `50` | Test cases per request |
| `MAX_CONCURRENT_JOBS` | `num CPUs` | Concurrent execution slots |
| `MAX_OUTPUT_BYTES` | `262144` | Captured stdout cap per phase |
| `MAX_STDIN_BYTES` | `65536` | stdin cap per test case |

## Docs

- [docs/api.md](docs/api.md) — full API contract and error codes
- [docs/architecture.md](docs/architecture.md) — package layout, request lifecycle, concurrency model
- [docs/security.md](docs/security.md) — seven security holes and where each is fixed
- [docs/languages.md](docs/languages.md) — language registry schema
- [docs/benchmarks.md](docs/benchmarks.md) — load-test results

## License

GPL v3. See [LICENSE](LICENSE).
