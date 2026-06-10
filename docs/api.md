# API Reference

Two execution endpoints, three health endpoints. Structurally valid requests always return `200`; outcomes live in the body.

---

## Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/run` | Execute code against test cases (competition contract) |
| `POST` | `/v1/run` | Superset: raw mode, evaluator mode, `exit_code` |
| `GET` | `/healthz` | Liveness |
| `GET` | `/readyz` | Readiness check: nsjail and all language probes |
| `GET` | `/info` | Build info, language list, live stats |
| `GET` | `/playground/` | Interactive browser UI |
| `GET` | `/docs/` | Swagger UI |

---

## POST /run

<details>
<summary><strong>Request fields</strong></summary>

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `language` | string | yes | Language ID (see [languages.md](languages.md)) |
| `source` | string | yes | UTF-8 source code |
| `source_filename` | string | see notes | Required for `from_request` languages (Java). Validated even for fixed-filename languages. |
| `artifact_filename` | string | see notes | Required when the compiled artifact name must match the class name |
| `tests` | array | yes | At least one element |
| `tests[].stdin` | string | yes | Standard input fed to the program |
| `tests[].expected_stdout` | string | yes | Expected output for comparison |
| `build.flags` | array | no | Compiler flags, validated against the language allow-list |
| `build.limits` | object | no | Partial override: `wall_time_s`, `memory_kb`, `max_processes` |
| `run.flags` | array | no | Runtime flags, validated against the allow-list |
| `run.limits` | object | no | Same fields as `build.limits` |

</details>

<details>
<summary><strong>Response fields</strong> (always <code>200 OK</code> for structurally valid requests)</summary>

| Field | Type | Notes |
|-------|------|-------|
| `status` | string | Top-level aggregated outcome |
| `build.status` | string | `ok`, `failed`, or `internal_error` |
| `build.duration_ms` | integer | Build phase wall time |
| `tests[].status` | string | Per-test outcome (see Status Vocabulary below) |
| `tests[].stdout` / `.stderr` | string | Captured output, truncated at `MAX_OUTPUT_BYTES` |
| `tests[].duration_ms` | integer | Per-test wall time |
| `tests[].memory_peak_kb` | integer | Peak RSS from cgroup `memory.peak` |

</details>

---

## POST /v1/run

Superset of `/run`. Adds `exit_code` to each test result. Execution mode is selected by the payload shape:

| Mode | How to trigger | Behaviour |
|------|---------------|-----------|
| **verifier** | `tests` present (default) | Grade each test by stdout comparison |
| **raw** | `tests` empty | Run once against top-level `stdin`; no grading |
| **evaluator** | `evaluator` block present | Grade each test with a custom program |

<details>
<summary><strong>Evaluator contract</strong></summary>

The evaluator is compiled once, then run per test in its own jail. Working directory contains:

| File | Contents |
|------|----------|
| `input` | test's `stdin` |
| `expected` | test's `expected_stdout` |
| `output` | candidate's actual stdout |

Must write one JSON line to stdout:
```json
{"verdict": "accepted", "score": 0.9, "message": "optional note"}
```

- `verdict: accepted` maps to test status `accepted`; anything else maps to `wrong_output`
- Candidate crash or timeout is reported as-is; evaluator is not called
- Evaluator crash or invalid JSON yields `internal_error` for that test

</details>

---

## Status Vocabulary

### Top-level status

| Status | When |
|--------|------|
| `accepted` | Build ok and all tests passed |
| `build_failed` | Compilation failed; all tests become `not_executed` |
| `wrong_output` | First failing test had wrong output |
| `output_whitespace_mismatch` | First failing test matched except trailing whitespace |
| `time_exceeded` | First failing test hit the wall-time limit |
| `memory_exceeded` | First failing test was killed by the cgroup OOM killer |
| `runtime_error` | First failing test crashed or received a fatal signal |
| `not_executed` | Test was skipped because an earlier step failed |
| `internal_error` | Infrastructure failure in the sandbox backend |

**Aggregation rule:** top-level = status of the first non-`accepted` test, in order. Build failure short-circuits: all tests become `not_executed`, top-level becomes `build_failed`.

Test-level status uses the same vocabulary minus `build_failed`.

---

## HTTP Status Codes

| Code | When |
|------|------|
| `200` | Any structurally valid request, regardless of user code outcome |
| `400` | Bad JSON, unknown language, disallowed flag, size exceeded, etc. |
| `503` | Load shedding: wait queue exceeded `MAX_QUEUE_DEPTH`; includes `Retry-After: 1` |
| `5xx` | Infrastructure failure only, never for user code errors |

---

## Error Codes

All `400` responses use this envelope:
```json
{"error": {"code": "snake_case_code", "message": "human-readable description"}}
```

| Code | Trigger |
|------|---------|
| `invalid_json` | Malformed request body |
| `unknown_language` | Language ID not in registry |
| `source_too_large` | Source exceeds `MAX_SOURCE_BYTES` (default 256 KiB) |
| `invalid_filename` | Path traversal, disallowed characters, or malformed name |
| `missing_source_filename` | `from_request` strategy but field absent |
| `missing_artifact_filename` | Compiled language needs artifact name but field absent |
| `invalid_flag` | Flag not on the language's `flag_allowlist` |
| `invalid_limits` | Override exceeds the language maximum |
| `invalid_test_count` | Zero tests or over `MAX_TESTS` |
| `stdin_too_large` | A `stdin` value exceeds `MAX_STDIN_BYTES` |
| `expected_too_large` | An `expected_stdout` value exceeds `MAX_STDIN_BYTES` |
| `internal_error` | Sandbox setup failure |
