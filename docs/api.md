# API Reference

All requests and responses use `application/json`. The server listens on `:8080` by default.

## POST /run

Compile (if applicable) and run source code against one or more test cases.

**Request body:**

```json
{
  "language": "cpp",
  "source": "#include <iostream>\nint main(){std::cout<<\"hi\";}",
  "source_filename": "solution.cpp",
  "artifact_filename": "solution",
  "build": {
    "limits": { "wall_time_s": 5, "memory_kb": 1048576, "max_processes": 100 },
    "flags": ["-O2"]
  },
  "run": {
    "limits": { "wall_time_s": 3, "memory_kb": 524288, "max_processes": 64 },
    "flags": []
  },
  "tests": [
    { "stdin": "1\n", "expected_stdout": "hi" }
  ]
}
```

| Field | Required | Notes |
|-------|----------|-------|
| `language` | Yes | Must match a registered language id |
| `source` | Yes | UTF-8, max 256 KiB |
| `source_filename` | Conditional | Required when `source_filename_strategy: from_request` (e.g. Java) |
| `artifact_filename` | Conditional | Required when `artifact_filename_strategy: from_request` (e.g. Java) |
| `build.limits` | No | Partial override of language defaults |
| `build.flags` | No | Extra build flags; filtered against per-language allowlist |
| `run.limits` | No | Partial override of language defaults |
| `run.flags` | No | Extra run flags; filtered against per-language allowlist |
| `tests` | Yes | At least 1, max 50 |
| `tests[].stdin` | No | Fed to the program's stdin, max 64 KiB |
| `tests[].expected_stdout` | No | Compared to the program's stdout |

**Response (200):**

```json
{
  "status": "accepted",
  "build": {
    "status": "ok",
    "stdout": "",
    "stderr": "",
    "duration_ms": 412
  },
  "tests": [
    {
      "status": "accepted",
      "stdout": "hi",
      "stderr": "",
      "duration_ms": 38,
      "memory_peak_kb": 0
    }
  ]
}
```

**Status vocabulary:**

| Scope | Values |
|-------|--------|
| Top-level | `accepted`, `build_failed`, `wrong_output`, `output_whitespace_mismatch`, `time_exceeded`, `memory_exceeded`, `runtime_error`, `internal_error` |
| `build.status` | `ok`, `failed`, `internal_error` |
| `tests[].status` | `accepted`, `wrong_output`, `output_whitespace_mismatch`, `time_exceeded`, `memory_exceeded`, `runtime_error`, `not_executed`, `internal_error` |

Top-level `accepted` requires `build.status == ok` and every test `accepted`. Otherwise it is the first non-`accepted` test status. If build fails, top-level is `build_failed` and all tests are `not_executed`.

**Errors (400):**

```json
{ "error": { "code": "unknown_language", "message": "language \"cobol\" is not registered" } }
```

| Code | Cause |
|------|-------|
| `invalid_json` | Malformed request body |
| `unknown_language` | Language id not registered |
| `missing_source` | `source` field absent or empty |
| `source_too_large` | Source exceeds 256 KiB |
| `missing_source_filename` | Required by language, not provided |
| `missing_artifact_filename` | Required by language, not provided |
| `invalid_filename` | Filename fails safety check |
| `invalid_flag` | Flag not in language allowlist |
| `invalid_limits` | Limit override exceeds language-configured maximum |
| `invalid_test_count` | 0 tests or exceeds 50 |
| `stdin_too_large` | Test stdin exceeds 64 KiB |
| `expected_too_large` | Test `expected_stdout` exceeds 64 KiB |

`5xx` is reserved for server failures (nsjail missing, disk full). Never returned because user code crashed.

---

## GET /healthz

Liveness. Returns `200 {"status":"ok"}` if the process is up.

---

## GET /readyz

Readiness. Probes nsjail and every language binary. Returns `200` if all pass, `503` otherwise.

```json
{
  "status": "degraded",
  "nsjail": { "ok": true, "version": "nsjail version: 3.6" },
  "languages": {
    "py3":  { "ok": true,  "version": "Python 3.11.9" },
    "java": { "ok": false, "error": "java not found at /usr/bin/java: ..." }
  }
}
```

---

## GET /info

Always `200`. Returns build metadata, nsjail info, language list, server limits, and runtime stats.

```json
{
  "build_info": { "version": "0.1.0", "commit": "abc1234", "go_version": "go1.23.0" },
  "nsjail": { "path": "/usr/local/bin/nsjail", "version": "nsjail version: 3.6" },
  "languages": [
    {
      "id": "py3",
      "name": "Python 3",
      "version": "Python 3.11.9",
      "default_run_limits": { "wall_time_s": 10, "memory_kb": 102400, "max_processes": 100 }
    }
  ],
  "limits": {
    "max_source_bytes": 262144,
    "max_tests": 50,
    "max_concurrent_jobs": 8
  },
  "stats": {
    "in_flight_jobs": 2,
    "jobs_total": 1042,
    "jobs_failed_internal": 0,
    "last_internal_error_at": null,
    "disk_free_bytes_jail_dir": 53687091200
  }
}
```
