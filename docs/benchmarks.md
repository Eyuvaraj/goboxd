# Benchmarks

Results from a clean Docker container (`make build && make run`) on the measurement host.

---

## Python 3 — interpreted, no compile step

### Test Payload

```json
{
  "language": "py3",
  "source": "print('Hello, World!')",
  "tests": [{ "stdin": "", "expected_stdout": "Hello, World!\n" }]
}
```

| Concurrent Clients | Requests | Req/s | p50 (ms) | p95 (ms) | p99 (ms) | Errors |
|--------------------|----------|-------|----------|----------|----------|--------|
| 1                  | 200      | 50.3  | 18.0     | 37.4     | 44.9     | 0      |
| 10                 | 200      | 201.2 | 42.2     | 93.5     | 121.9    | 0      |
| 50                 | 200      | 212.3 | 223.6    | 289.0    | 303.6    | 0      |
| 100                | 200      | 203.0 | 451.0    | 521.7    | 539.2    | 0      |

All responses returned HTTP 200 at every concurrency level. Requests queue (not fail) when all `MAX_CONCURRENT_JOBS` slots are busy — the semaphore holds correctly under load.

---

## C++ — compiled (g++), single test case

### Test Payload

```json
{
  "language": "cpp",
  "source": "#include <iostream>\nint main(){std::cout<<\"Hello, World!\\n\";}",
  "source_filename": "solution.cpp",
  "artifact_filename": "solution",
  "tests": [{ "stdin": "", "expected_stdout": "Hello, World!\n" }]
}
```

| Concurrent Clients | Requests | Req/s | p50 (ms) | p95 (ms) | p99 (ms)  | Errors |
|--------------------|----------|-------|----------|----------|-----------|--------|
| 1                  | 100      | 4.1   | 242.0    | 260.0    | 300.4     | 0      |
| 10                 | 100      | 14.1  | 426.5    | 1705.8   | 1791.3    | 0      |
| 50                 | 100      | 14.6  | 3017.6   | 3649.3   | 3829.8    | 0      |
| 100                | 100      | 14.0  | 3304.8   | 6673.0   | 7123.8    | 0      |

Each request includes a full compile + run cycle (g++ invoked inside nsjail per request). At 1 client, per-request latency is dominated by the ~240 ms g++ compile time. At 10+ clients, requests queue on the semaphore once all `MAX_CONCURRENT_JOBS` slots are held by in-progress compile jobs — p95/p99 grows proportionally with concurrency while req/s plateaus, which is the expected bounded-queue behaviour.

---

## Test Environment

- **Host:** MacBook Air (Apple M4, 10-core CPU, 16 GB RAM)
- **Docker resource limits:** None (default)
- **`MAX_CONCURRENT_JOBS`:** 10
- **Load tool:** [hey](https://github.com/rakyll/hey)

---

## How to Reproduce

```bash
make build
make run          # in one terminal
bash scripts/load_test.sh   # in another terminal (requires hey or k6 in PATH)
```

Install hey: `go install github.com/rakyll/hey@latest`
