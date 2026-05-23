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

goboxd is an HTTP service written in Go that compiles and runs untrusted code inside isolated sandboxes and returns the result. Optional test cases can be supplied to assert behaviour against expected output. It is built for safe execution of code across many languages, with strict isolation, bounded concurrency, and a plug and play language registry.

## Features

- Plug and play language registry driven by YAML
- Process isolation using Linux namespaces and cgroups
- Bounded concurrency with request queuing
- Fully containerised for local development and deployment
- Per request resource limits for time, memory, and processes
- Liveness and readiness probes for orchestration

## Getting started

**Prerequisite:** Docker with Compose v2. No host Go installation needed.

```bash
make build   # build the image (nsjail is compiled from source)
make run     # start the service on :8080
```

Verify it's up:

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/info
```

Run a hello-world:

```bash
curl -s -X POST http://localhost:8080/run \
  -H 'Content-Type: application/json' \
  -d '{"language":"py3","source":"print(\"hello\")","tests":[{"stdin":"","expected_stdout":"hello\n"}]}'
```

### Makefile targets

| Target | What it does |
|--------|--------------|
| `make build` | Build the runtime Docker image |
| `make run` | Start the service on :8080 |
| `make test` | Run unit tests (no nsjail required) |
| `make integration` | Run end-to-end tests inside the container |
| `make lint` | Run golangci-lint |
| `make load` | Run the load-test benchmark (requires hey or k6) |

## HTTP framework

chi (`github.com/go-chi/chi/v5`). It gives a composable middleware chain without framework overhead and stays close to `net/http`, making handlers straightforward to test.

## Project structure

```
.
├── cmd/goboxd/   binary entry point
├── internal/     private application packages
├── docs/         api, languages, security, benchmarks, architecture
└── tests/        integration tests
```

## Contributing

Contributions are welcome. Open an issue to discuss substantial changes before sending a pull request.

## License

This project is distributed under the GNU General Public License v3.0. See [LICENSE](LICENSE) for the full text.
