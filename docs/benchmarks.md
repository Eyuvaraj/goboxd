# Benchmarks

This document outlines the performance benchmarks for `goboxd`. 

Results are derived from a clean Docker container (`make build && make run`) running on the measurement host. All tests simulate a Python 3 "Hello, World!" execution via the `POST /run` endpoint.

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

| Concurrent Clients | Req/s | p50 (ms) | p95 (ms) | p99 (ms) |
|--------------------|-------|----------|----------|----------|
| 1                  | 60.6  | 15.9     | 19.0     | 44.1     |
| 10                 | 248.5 | 33.1     | 61.8     | 100.1    |
| 50                 | 240.1 | 200.8    | 247.0    | 260.3    |
| 100                | 226.9 | 402.3    | 452.9    | 464.7    |

### Test Environment Specifications

- **Measurement Host:** MacBook Air (M4, 10-core CPU, 16 GB RAM)
- **Docker Resource Limits:** None (default configuration)
- **`MAX_CONCURRENT_JOBS`:** 10

---

## How to Reproduce

To run these benchmarks on your own infrastructure, you will need a load testing tool like [`hey`](https://github.com/rakyll/hey) or [`k6`](https://k6.io/).

1. **Start the server:**
   In your first terminal, build and run the Docker image:
   ```bash
   make build
   make run
   ```

2. **Execute the load test:**
   In a second terminal, run the provided load testing script:
   ```bash
   bash scripts/load_test.sh
   ```
