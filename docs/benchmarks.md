# Benchmarks

Results from a clean Docker container (`make build && make run`) on the measurement host.

---

## Python 3 (interpreted, no compile step)

**Test payload:**
```json
{
  "language": "py3",
  "source": "print('Hello, World!')",
  "tests": [{ "stdin": "", "expected_stdout": "Hello, World!\n" }]
}
```

| Concurrent Clients | Requests | Req/s | p50 (ms) | p95 (ms) | p99 (ms) | Errors |
|--------------------|----------|-------|----------|----------|----------|--------|
| 1                  | 200      | 129.2 | 7.2      | 9.5      | 14.5     | 0      |
| 10                 | 200      | 486.1 | 18.5     | 32.3     | 43.5     | 0      |
| 50                 | 200      | 503.1 | 93.8     | 108.7    | 114.4    | 0      |
| 100                | 200      | 437.4 | 202.5    | 258.8    | 277.5    | 0      |

All responses returned HTTP 200 at every concurrency level. Requests queue when all `MAX_CONCURRENT_JOBS` slots are busy; the semaphore holds correctly under load.

---

## C++ (compiled with g++), single test case

**Test payload:**
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
| 1                  | 100      | 7.7   | 127.8    | 147.1    | 163.3     | 0      |
| 10                 | 100      | 32.0  | 222.8    | 459.3    | 847.2     | 0      |
| 50                 | 100      | 30.9  | 1428.6   | 1946.8   | 2001.5    | 0      |
| 100                | 100      | 31.8  | 1749.0   | 3081.8   | 3148.3    | 0      |

Each request includes a full compile + run cycle. At 1 client, latency is dominated by the ~130 ms `g++` compile time. At 10+ clients, requests queue on the semaphore as slots fill with in-progress compile jobs. Throughput plateaus while p95/p99 grows proportionally, which is the expected bounded-queue behaviour.

---

## Test Environment

| Parameter | Value |
|-----------|-------|
| Host | MacBook Air (Apple M4, 10-core CPU, 16 GB RAM) |
| Docker resource limits | None (default) |
| `MAX_CONCURRENT_JOBS` | 10 |
| Load tool | [hey](https://github.com/rakyll/hey) |

---

## Reproduce

```bash
make build
make run                      # in one terminal
bash scripts/load_test.sh     # in another terminal (requires hey or k6)
```

Install `hey`: `go install github.com/rakyll/hey@latest`
