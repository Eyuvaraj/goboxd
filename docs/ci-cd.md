# CI/CD and Supply Chain

How goboxd is built, tested, scanned, and benchmarked automatically. Two GitHub Actions workflows live in `.github/workflows/`; both check out the nsjail and kafel submodules recursively and read the Go version from `go.mod`.

---

## CI Pipeline

**File:** `.github/workflows/ci.yml`

**Triggers:** push to `master` or any `team/**` branch; pull requests targeting `master`.

**Runner:** single `test` job on `ubuntu-latest`, with `submodules: recursive` checkout and Go set up from `go.mod` with module caching enabled.

Stages run in this order; any failure fails the build:

| # | Stage | Command | What it guards |
|---|-------|---------|----------------|
| 1 | Unit tests | `go test ./internal/... ./cmd/...` | Logic correctness; no Docker or nsjail needed |
| 2 | Vulnerability scan | `golang/govulncheck-action@v1 ./...` | Known CVEs reachable from our code |
| 3 | Build tools image | `docker/build-push-action` (target: `builder`) | Compiles nsjail, Go toolchain, linters; cached |
| 4 | Build runtime image | `docker/build-push-action` (target: `runtime`) | The shippable image; cached |
| 5 | Lint | `make lint` (golangci-lint in the tools container) | Style, static analysis, gosec |
| 6 | Integration tests | `docker compose up -d goboxd` then `make integration` | Live end-to-end path through nsjail |

**Why the layer cache matters.** nsjail compiles from source (~5 minutes cold). The `type=gha` build cache persists Docker layers across runs, so that cost is paid once and reused on subsequent pushes.

**Why integration tests run against a real container.** nsjail needs Linux namespaces, cgroup v2, and seccomp; it cannot run on the bare runner the way unit tests do. Stage 6 starts the actual `goboxd` service via Compose and POSTs to it, so the path that matters most (code to nsjail to result) is verified for real, not mocked.

---

## Static Security Analysis: gosec

gosec runs as one of the linters configured in `.golangci.yml` (`govet`, `staticcheck`, `errcheck`, `unused`, `ineffassign`, `misspell`, `gosec`). It flags insecure Go patterns: command injection, unchecked file permissions, unsafe integer conversions, and tainted path joins.

Two categories of suppression are deliberate and documented at the source:

- **G204 (subprocess launched with a variable argv)** is globally excluded in `.golangci.yml`. Every `exec` call takes a pre-validated `[]string`: filenames pass `validate.Filename`, flags pass `validate.Flags`, and there is no shell anywhere. Building the argv from variables is the design; G204 would fire on every call site for a risk that the validation layer already removes.
- **Per-line `//nolint:gosec`** annotations sit on the cgroup, `/proc`, and stdin file operations in `internal/sandbox`. Each carries a comment explaining that the path is constructed internally (from `MkdirTemp` or a monotonic run counter) and is never attacker-controlled.

`_test.go` files are excluded from `errcheck` and `gosec` to keep tests terse.

---

## Dependency Vulnerability Scanning: govulncheck

`govulncheck` (CI stage 2) is more precise than a plain dependency audit: it checks the Go vulnerability database against the symbols the binary actually calls, so it only fails on vulnerabilities that are reachable from goboxd's code path, not merely present in the dependency graph.

The dependency surface is intentionally tiny: chi and yaml.v3 at runtime, swaggo only for docs generation. This keeps the signal clean.

---

## Benchmarks Workflow

**File:** `.github/workflows/benchmarks.yml`

**Trigger:** manual (`workflow_dispatch`) only.

Steps: build the runtime image, start via `docker compose up -d goboxd`, poll `/healthz` until ready, then run `make load` (the `hey`-based `scripts/load_test.sh`, which sweeps 1/10/50/100 concurrent clients for py3 and cpp).

Not part of the push/PR pipeline: throughput and latency are sensitive to the host (shared CI runners are noisy and not representative), so running it on every commit would produce misleading numbers. Reference results on a known machine: [benchmarks.md](benchmarks.md).

---

## Supply Chain and Reproducibility

- **Pinned toolchain.** Go `1.26.4` via the `go.mod` directive and the Dockerfile `GO_VERSION` arg.
- **Pinned sandbox.** nsjail at git tag `3.4` as a submodule, built from source (never an apt package or a prebuilt binary). Kafel (`20200831-34-g1af0975`) is a nested submodule of nsjail, compiled into it.
- **Pinned dependencies.** `go.mod` and `go.sum` lock chi `v5.2.2` and yaml.v3 `v3.0.1`; `go mod download` runs as its own Docker layer for cache efficiency.
- **Reproducible binary.** Built with `CGO_ENABLED=0 -trimpath -ldflags="-s -w"`; `Version` and `Commit` are injected via `-X` ldflags and surfaced at `/info`.
- **Lean runtime image.** The final stage carries only the binary, nsjail, the Go toolchain (for the `go` language), and the language runtimes; none of the build tools or linters.

---

<!-- nav-footer -->
<sub>[← Documentation index](README.md) · [API](api.md) · [Architecture](architecture.md) · [Concurrency](concurrency.md) · [Security](security.md) · [Languages](languages.md) · [Configuration](configuration.md)</sub>
