# AI Usage Log

## [2026-05-28] Build and audit APIs, write comprehensive unit tests

**Prompt:** "can you build and test all the apis and its output and verify them and do the needful updates... and audit the code and write tests"

**Response summary:**
- Audited all packages: handler, runner, sandbox, validate, registry, config, stats, logctx
- Found `internal/handler` had zero test coverage; `validate` was missing StdinSize, ExpectedSize, Limits tests
- Found `compareOutput` used `bytes.TrimSpace` (strips leading + trailing whitespace) instead of `bytes.TrimRight` (trailing only) — spec says "stripping trailing whitespace"
- Added `Submitter` interface to `handler/run.go` to decouple the handler from `*runner.Runner` and enable dependency injection in tests
- Wrote `internal/handler/run_test.go`: 24 tests covering all 400 validation paths (invalid_json, unknown_language, missing_source, source_too_large, missing_source_filename, missing_artifact_filename, invalid_filename, invalid_flag for build and run phases, invalid_test_count, stdin_too_large, expected_too_large, invalid_limits for both build and run), plus 200/500/503 success and error paths
- Wrote `internal/handler/health_test.go`: 5 tests for /healthz (200+body), /readyz (structure + 503-when-degraded), /info (structure, live stats)
- Added TestStdinSize, TestExpectedSize, TestLimits to `internal/validate/request_test.go`
- Added `internal/runner/compare_test.go`: 7 tests for compareOutput including the leading-whitespace case that distinguishes wrong_output from output_whitespace_mismatch
- Fixed `compareOutput` in `runner/job.go`: replaced `bytes.TrimSpace` with `bytes.TrimRight(…, " \t\r\n")` to match spec semantics

**What we used / didn't use:** All changes were kept. The Submitter interface pattern is standard Go; the whitespace fix aligns code with the spec. All 44 unit tests pass.
