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
make run                                # in one terminal
make load                               # in another (requires vegeta + jq)
python3 docs/loadtest/plot.py docs/loadtest/results.csv docs/loadtest
```

Install vegeta: `brew install vegeta`  
Install jq: `brew install jq`

---

## Stage 3 — MemoryHog Load Test (Breaking-Point)

Stress test using `MemoryHog.java` (150 MB heap allocation + 1 s hold) with the service pinned to **2 vCPU / 2 GB RAM** via `docker-compose.override.yml`.

**Tool:** vegeta 12.13.0 · **Timeout:** 10 s · **Duration per step:** 30 s

| Target RPS | Actual Throughput | Success (200) | Error % | p50 (ms) | p95 (ms) | p99 (ms) |
|---:|---:|---:|---:|---:|---:|---:|
| 5 | 3.09 | 123/150 | **18.00%** ← BREAK | 7,049 | 10,001 | 10,001 |
| 10 | 1.23 | 49/300 | 83.67% | 10,000 | 10,001 | 10,002 |
| 25 | 0.70 | 28/750 | 96.27% | 10,000 | 10,001 | 10,002 |
| 50 | 0.30 | 12/1,500 | 99.20% | 10,000 | 10,001 | 10,004 |
| 75 | 0.78 | 31/2,250 | 98.62% | 10,000 | 10,001 | 10,001 |
| 100 | 0.43 | 17/3,000 | 99.43% | 10,000 | 10,001 | 10,002 |
| 150 | 0.28 | 11/4,500 | 99.76% | 10,000 | 10,001 | 10,001 |
| 200 | 0.50 | 20/6,000 | 99.67% | 10,000 | 10,001 | 10,002 |
| 300 | 0.43 | 17/9,000 | 99.81% | 10,000 | 10,001 | 10,001 |
| 400 | 0.43 | 17/12,000 | 99.86% | 10,000 | 10,001 | 10,002 |

**Breaking point: 5 req/s.** Each MemoryHog JVM uses ~200–230 MB RSS and holds memory for 1 s. With a 2-slot concurrency semaphore (= vCPU count) and 2 GB RAM, requests queue behind the 2 in-flight jobs and exceed the 10 s client timeout. At ≥75 req/s, TCP connection resets appear alongside timeouts — the accept backlog saturates, but the service recovers cleanly after load drops.

→ Full results, graphs, and root-cause analysis: [`docs/loadtest/README.md`](loadtest/README.md)  
→ Raw data: [`docs/loadtest/results.csv`](loadtest/results.csv)
