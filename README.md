<div align="center">

# goboxd

**A Go HTTP service for executing untrusted code in isolated sandboxes.**

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.23-00ADD8.svg?logo=go&logoColor=white)](https://go.dev)
[![Docker](https://img.shields.io/badge/Docker-Required-2496ED.svg?logo=docker&logoColor=white)](https://www.docker.com)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](https://github.com/thesouldev/goboxd/pulls)

</div>

---

## Overview

goboxd is an HTTP service written in Go that compiles and runs untrusted code inside isolated sandboxes and returns the result. Optional test cases can be supplied to assert behaviour against expected output. It is built for safe execution of code across many languages, with strict isolation, bounded concurrency, and a plug-and-play language registry.

## Features

- Plug-and-play language registry driven by YAML
- Process isolation via nsjail (Linux namespaces + cgroups)
- Per-request resource limits for time, memory, and processes
- Bounded concurrency with request queuing
- Fully containerised — no host toolchain required
- Interactive Swagger UI at `/docs/`
- Liveness and readiness probes for orchestration

## Quick start

**Only prerequisite:** [Docker](https://docs.docker.com/get-docker/) with Compose v2.  
No Go, no nsjail, no language runtimes needed on the host — everything runs inside the container.

```bash
git clone https://github.com/thesouldev/goboxd.git
cd goboxd

make build   # build the image (~5 min first time — nsjail is compiled from source)
make run     # start the service on :8080
```

Verify it's up:

```bash
curl http://localhost:8080/healthz
# {"status":"ok"}
```

Open the interactive API docs in your browser:

```
http://localhost:8080/docs/index.html
```

Run a hello-world:

```bash
curl -s -X POST http://localhost:8080/run \
  -H 'Content-Type: application/json' \
  -d '{
    "language": "py3",
    "source": "print(input())",
    "tests": [{"stdin": "hello\n", "expected_stdout": "hello\n"}]
  }'
```

---

## Setting up a new environment

### Requirements

| Requirement | Notes |
|---|---|
| Linux host | nsjail requires Linux kernel ≥ 4.6 with user namespaces enabled. macOS/Windows users must use Docker Desktop or a Linux VM — nsjail will not run natively on those hosts. |
| Docker Engine ≥ 24 | Compose v2 (`docker compose`) is bundled from Docker Engine 23+ |
| Privileged containers | The runtime container runs with `--privileged` for nsjail namespace support |

### Step-by-step

**1. Clone the repository**

```bash
git clone https://github.com/thesouldev/goboxd.git
cd goboxd
```

**2. Build the Docker image**

```bash
make build
```

This builds a multi-stage image:
- Compiles nsjail 3.4 from source
- Compiles the Go binary with build metadata injected via `-ldflags`
- Assembles a slim Debian runtime image with all language toolchains

The first build takes ~5 minutes. Subsequent builds are fast thanks to layer caching.

**3. Start the service**

```bash
make run
```

The service starts on port `8080`. Press `Ctrl+C` to stop — it shuts down gracefully.

**4. Confirm all language runtimes are healthy**

```bash
curl -s http://localhost:8080/readyz | python3 -m json.tool
```

All entries under `languages` should show `"ok": true`. A `503` response means at least one language binary is missing from the image.

---

## API

Full interactive docs are available at **`/docs/index.html`** when the server is running.

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Liveness — returns `200` if the process is up |
| `GET` | `/readyz` | Readiness — probes nsjail and every language binary |
| `GET` | `/info` | Build info, language list, limits, and live stats |
| `POST` | `/run` | Execute source code against test cases |
| `GET` | `/docs/*` | Swagger UI |

### POST /run — minimal example

```json
{
  "language": "py3",
  "source": "print(input())",
  "tests": [
    { "stdin": "hello\n", "expected_stdout": "hello\n" }
  ]
}
```

### Compiled language example (C++)

```json
{
  "language": "cpp",
  "source": "#include <iostream>\nint main(){std::cout<<\"hello\\n\";}",
  "tests": [
    { "stdin": "", "expected_stdout": "hello\n" }
  ]
}
```

### Java (filename required)

Java requires `source_filename` and `artifact_filename` to match the public class name:

```json
{
  "language": "java",
  "source": "public class Main { public static void main(String[] args) { System.out.println(\"hello\"); } }",
  "source_filename": "Main.java",
  "artifact_filename": "Main",
  "tests": [
    { "stdin": "", "expected_stdout": "hello\n" }
  ]
}
```

### Response status vocabulary

| Field | Possible values |
|---|---|
| Top-level `status` | `accepted`, `build_failed`, `wrong_output`, `output_whitespace_mismatch`, `time_exceeded`, `memory_exceeded`, `runtime_error`, `internal_error` |
| `build.status` | `ok`, `failed`, `internal_error` |
| `tests[].status` | `accepted`, `wrong_output`, `output_whitespace_mismatch`, `time_exceeded`, `memory_exceeded`, `runtime_error`, `not_executed`, `internal_error` |

HTTP `200` is returned for all structurally valid requests — execution outcomes are in the body. Only `400` (validation), `500` (server fault), and `503` (capacity / cancelled) are HTTP-level errors.

---

## Supported languages

| ID | Language |
|---|---|
| `py3` | Python 3 |
| `js` | Node.js |
| `c` | C (gcc) |
| `cpp` | C++ (g++) |
| `java` | Java |
| `bash` | Bash |
| `verilog` | Verilog (iverilog) |

Language definitions live in [`configs/languages.yaml`](configs/languages.yaml). Adding a new language is a YAML change plus a toolchain install in the `Dockerfile`.

---

## Configuration

All settings are read from environment variables at startup. Defaults work out of the box.

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `NSJAIL_PATH` | `/usr/local/bin/nsjail` | Path to nsjail binary |
| `JAIL_DIR` | `/tmp/goboxd` | Working directory for sandbox jails |
| `LANGUAGE_FILE` | `/etc/goboxd/languages.yaml` | Path to language registry YAML |
| `MAX_SOURCE_BYTES` | `262144` (256 KiB) | Maximum source code size |
| `MAX_TESTS` | `50` | Maximum test cases per request |
| `MAX_CONCURRENT_JOBS` | `(num CPUs)` | Concurrent execution slots |
| `MAX_OUTPUT_BYTES` | `262144` (256 KiB) | Maximum stdout/stderr captured per phase |
| `MAX_STDIN_BYTES` | `65536` (64 KiB) | Maximum stdin size per test case |
| `ORPHAN_MAX_AGE_MINUTES` | `30` | Age after which stale jail directories are swept |

To override, set them in `docker-compose.yml` under `environment:` or pass them via `docker run -e`.

---

## Makefile targets

| Target | Description |
|---|---|
| `make build` | Build the runtime Docker image |
| `make run` | Start the service on :8080 |
| `make test` | Run unit tests (no nsjail required) |
| `make integration` | Run end-to-end tests inside the container |
| `make lint` | Run golangci-lint |
| `make swagger` | Regenerate Swagger docs from annotations (requires host Go + `swag` CLI) |
| `make load` | Run the load-test benchmark (requires `hey` or `k6` in PATH) |

---

## Project structure

```
.
├── cmd/goboxd/          binary entry point + swagger metadata
├── configs/             language registry YAML
├── docs/                generated swagger spec + reference docs
├── internal/
│   ├── config/          environment-based configuration
│   ├── handler/         HTTP handlers and request/response types
│   ├── registry/        language registry loader and runtime probes
│   ├── runner/          job runner with concurrency control
│   ├── sandbox/         nsjail workspace management
│   ├── stats/           runtime counters
│   └── validate/        request validation
└── tests/integration/   end-to-end tests
```

---

## Development

Unit tests run on any machine with Go — no nsjail or Docker needed:

```bash
go test ./internal/...
```

Integration tests require the containerised service to be running:

```bash
make run          # in one terminal
make integration  # in another
```

To update the Swagger docs after editing handler annotations:

```bash
make swagger      # requires host Go; installs swag CLI automatically if missing
```

---

## Contributing

Contributions are welcome. Open an issue to discuss substantial changes before sending a pull request.

## License

This project is distributed under the GNU General Public License v3.0. See [LICENSE](LICENSE) for the full text.
