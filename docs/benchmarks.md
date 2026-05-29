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
| 1                  | 200      | 50.3  | 18.0     | 37.4     | 44.9     | 0      |
| 10                 | 200      | 201.2 | 42.2     | 93.5     | 121.9    | 0      |
| 50                 | 200      | 212.3 | 223.6    | 289.0    | 303.6    | 0      |
| 100                | 200      | 203.0 | 451.0    | 521.7    | 539.2    | 0      |

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
| 1                  | 100      | 4.1   | 242.0    | 260.0    | 300.4     | 0      |
| 10                 | 100      | 14.1  | 426.5    | 1705.8   | 1791.3    | 0      |
| 50                 | 100      | 14.6  | 3017.6   | 3649.3   | 3829.8    | 0      |
| 100                | 100      | 14.0  | 3304.8   | 6673.0   | 7123.8    | 0      |

Each request includes a full compile + run cycle. At 1 client, latency is dominated by the ~240 ms `g++` compile time. At 10+ clients, requests queue on the semaphore as slots fill with in-progress compile jobs. Throughput plateaus while p95/p99 grows proportionally, which is the expected bounded-queue behaviour.

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
