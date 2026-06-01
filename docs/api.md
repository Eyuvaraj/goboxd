# goboxd API Reference

This document provides a concise reference for the goboxd REST API. goboxd acts as an execution backend, receiving code and tests, running them securely in an isolated sandbox, and returning structured results.

---

## Core Execution

### `POST /run`
Evaluates untrusted source code against a set of test cases. This is the primary endpoint for standard sandbox execution.

**Request Body (JSON)**
- `language` (string, required): The target language ID (e.g., `py3`, `cpp`, `java`). Must match a registered language.
- `source` (string, required): The source code to execute. UTF-8 encoded.
- `source_filename` (string, optional): Required only for languages with explicit filename strategies (e.g., Java). Must be a single path component.
- `artifact_filename` (string, optional): Required only if the compiled artifact requires a specific name.
- `tests` (array, required): List of test cases. At least one required.
  - `stdin` (string): Standard input fed to the program.
  - `expected_stdout` (string): The expected output for comparison.
- `build` / `run` (objects, optional): Allow per-request overrides.
  - `limits` (object): Partial overrides for `wall_time_s`, `memory_kb`, `max_processes`.
  - `flags` (array): Compiler or runtime flags. Must pass the language's allow-list.

**Response**
Always returns HTTP `200 OK` once execution completes, even if the user's code fails to compile or crashes.

- `status` (string): The aggregated top-level outcome.
- `build` (object): Results of the compilation phase (if applicable).
  - `status` (string): `ok`, `failed`, or `internal_error`.
  - `duration_ms` (integer): Build time in milliseconds.
- `tests` (array): Results for each test case, in the requested order.
  - `status` (string): See **Status Vocabulary** below.
  - `stdout` / `stderr` (strings): Captured output (truncated if oversized).
  - `duration_ms` (integer): Execution time in milliseconds.
  - `memory_peak_kb` (integer): Peak memory consumption.

### `POST /v1/run`
An extended execution endpoint supporting **raw mode** (running code without tests) and **evaluator mode** (using custom evaluation scripts). Includes `exit_code` fields in the response.

---

## System Health & Telemetry

### `GET /healthz`
Liveness probe. Indicates if the HTTP server process is running.
- **200 OK**: Process is up.

### `GET /readyz`
Readiness probe. Verifies that the sandbox environment is healthy and all configured languages are available.
- **200 OK**: nsjail is executable and all language compilers/interpreters pass their smoke tests.
- **503 Service Unavailable**: Degraded state. Returns JSON detailing which specific dependencies are missing or failing.

### `GET /info`
System metadata and live statistics.
- **200 OK**: Returns JSON with:
  - **Build info**: version, commit hash, Go version.
  - **Sandbox state**: nsjail version and path.
  - **Languages**: List of all loaded languages with their default resource limits.
  - **System Limits**: `max_source_bytes`, `max_tests`, `max_concurrent_jobs`.
  - **Live stats**: Current in-flight jobs, queue depth, total jobs processed, failure counts, and disk space remaining in the jail directory.

---

## Status Vocabulary

goboxd uses a strict taxonomy of status codes to categorize execution outcomes.

### Top-Level Status
The aggregate result of the request:
- `accepted`: Build succeeded, and all tests passed.
- `build_failed`: Compilation failed. No tests were executed.
- `wrong_output`: At least one test failed due to output mismatch.
- `output_whitespace_mismatch`: Output matched exactly, except for trailing whitespace.
- `time_exceeded`: A test ran past its wall-time limit.
- `memory_exceeded`: A test was killed by the cgroup OOM killer.
- `runtime_error`: A test crashed or was killed by a signal.
- `internal_error`: The sandbox backend encountered an infrastructure failure.

### Test-Level Status
Applies to individual items in the `tests` array:
- `accepted`
- `wrong_output`
- `output_whitespace_mismatch`
- `time_exceeded`
- `memory_exceeded`
- `runtime_error`
- `not_executed` (used when the build failed or a prior step aborted execution)
- `internal_error`

---

## Error Handling

Client errors preventing execution return HTTP `400 Bad Request`.

**Common Error Codes:**
- `invalid_json`: Malformed request body.
- `unknown_language`: The requested language ID is not configured.
- `source_too_large`: Source code exceeds the server limit.
- `invalid_filename`: Path traversal attempt or invalid characters.
- `invalid_flag`: Supplied flag is not on the language's allow-list.
- `invalid_limits`: Requested resource overrides exceed permitted maximums.

HTTP `5xx` errors are strictly reserved for server-side infrastructure failures (e.g., disk full, nsjail setup failure, queue exhaustion) and never indicate user code issues.
