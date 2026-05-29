# Benchmarks

Results from a clean Docker container (`make build && make run`) on the measurement host. All tests hit `POST /run` with a Python 3 hello-world payload (interpreted language, no compile step, single test case).

### Test Payload

```json
{
  "language": "py3",
  "source": "print('Hello, World!')",
  "tests": [
    {
      "stdin": "",
      "expected_stdout": "Hello, World!\n"
    }
  ]
}
```

---

## Results

| Concurrent Clients | Requests | Req/s | p50 (ms) | p95 (ms) | p99 (ms) | Errors |
|--------------------|----------|-------|----------|----------|----------|--------|
| 1                  | 200      | 61.5  | 15.1     | 19.0     | 42.6     | 0      |
| 10                 | 200      | 241.7 | 37.0     | 68.9     | 80.5     | 0      |
| 50                 | 500      | 234.0 | 206.9    | 259.8    | 279.8    | 0      |
| 100                | 500      | 241.3 | 401.1    | 475.9    | 499.6    | 0      |

All responses returned HTTP 200 at every concurrency level. Requests queue (not fail) when all `MAX_CONCURRENT_JOBS` slots are busy — the semaphore holds correctly under load.

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
