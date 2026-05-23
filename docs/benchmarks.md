# Benchmarks

Results from a clean Docker run (`make build && make run`) on the measurement host.
All tests use `POST /run` with the py3 hello-world payload.

**Payload:**
```json
{
  "language": "py3",
  "source": "print('Hello, World!')",
  "tests": [{"stdin": "", "expected_stdout": "Hello, World!\n"}]
}
```

## Results

<!-- Fill in after running: bash scripts/load_test.sh -->

| Clients | Req/s | p50 (ms) | p95 (ms) | p99 (ms) |
|---------|-------|----------|----------|----------|
| 1       | TBD   | TBD      | TBD      | TBD      |
| 10      | TBD   | TBD      | TBD      | TBD      |
| 50      | TBD   | TBD      | TBD      | TBD      |
| 100     | TBD   | TBD      | TBD      | TBD      |

**Measurement host:** TBD (e.g. M2 MacBook Pro, 8-core, 16 GB RAM)
**Docker resource limit:** none (default)
**MAX_CONCURRENT_JOBS:** TBD

## How to reproduce

```bash
make build
make run          # in one terminal
bash scripts/load_test.sh   # in another (requires hey or k6)
```
