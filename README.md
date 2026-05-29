# goboxd

An HTTP service that accepts source code, compiles or interprets it inside an `nsjail` sandbox, runs it against test cases, and returns per-test execution results. Built for the goboxd hackathon at Paradox IIT Madras 2026.

**Playground:** http://localhost:8080/playground/

---

## Overview

`goboxd` safely executes untrusted code using Linux namespaces, cgroups, seccomp, and Kafel sandbox policies. Each execution runs inside an isolated `nsjail` sandbox with strict resource and syscall limits.

Blocked syscalls include: `ptrace`, `bpf`, `io_uring`, clock manipulation, and kernel module operations.

## Why `chi`?

`chi` uses plain `net/http` handlers, explicit middleware composition, and avoids framework-specific request wrappers, making handler testing simpler and cleaner.

---

# Running

## Requirements

* Docker with Compose v2
* Container must run with `--privileged` (already configured)
* `nsjail` is compiled from source during image build

## Commands

```bash id="3n6w9v"
make build        # build the image
make run          # start service on :8080
make test         # unit tests
make integration  # end-to-end tests
make lint         # golangci-lint
make load         # load benchmarks
```

`make integration` requires `make run`. `make load` requires `hey` or `k6`.

---

# API

| Method | Path       | Description               |
| ------ | ---------- | ------------------------- |
| `GET`  | `/healthz` | Liveness probe            |
| `GET`  | `/readyz`  | Runtime readiness checks  |
| `GET`  | `/info`    | Build info and live stats |
| `POST` | `/run`     | Execute source code       |

Structurally valid requests always return `HTTP 200`. Execution results are returned in the response body.

HTTP-level errors: `400` validation, `500` internal error, `503` cancelled/unavailable.

See `docs/swagger.yaml` for the complete API schema.

---

# Supported Languages

`py3` `bash` `js` `c` `cpp` `java` `verilog` `ruby` `lua` `ocaml`

Adding a language only requires:

1. A YAML block in `configs/languages.yaml`
2. Installing the toolchain in the Dockerfile

No Go code changes are required.

---

# Sandbox Model

Each execution runs inside `nsjail` with process isolation, filesystem isolation, resource limits, and restricted syscalls enforced using namespaces, cgroups, seccomp, and Kafel.

---

# Documentation

| File                   | Description                       |
| ---------------------- | --------------------------------- |
| `docs/swagger.yaml`    | OpenAPI schema                    |
| `docs/architecture.md` | Request lifecycle and concurrency |
| `docs/security.md`     | Sandbox hardening and security    |
| `docs/languages.md`    | Language registry                 |
| `docs/benchmarks.md`   | Load-test benchmarks              |

---

# Playground

Browser playground for manual testing and demonstrations:
http://localhost:8080/playground/
