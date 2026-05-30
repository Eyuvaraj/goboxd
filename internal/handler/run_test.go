package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/thesouldev/goboxd/internal/config"
	"github.com/thesouldev/goboxd/internal/handler"
	"github.com/thesouldev/goboxd/internal/registry"
	"github.com/thesouldev/goboxd/internal/runner"
	"github.com/thesouldev/goboxd/internal/validate"
)

// testLangsYAML is a minimal language set covering all handler validation paths.
const testLangsYAML = `
languages:
  - id: py3
    name: Python 3
    source_filename: solution.py
    run:
      cmd: /usr/bin/python3
      args: ["{{source}}"]
      limits:
        wall_time_s: 10
        memory_kb: 102400
        max_processes: 100

  - id: c
    name: C
    source_filename: solution.c
    artifact_filename: solution
    build:
      cmd: /usr/bin/gcc
      args: ["{{flags}}", "-o", "{{artifact}}", "{{source}}"]
      limits:
        wall_time_s: 10
        memory_kb: 524288
        max_processes: 100
      flag_allowlist:
        - "-O2"
    run:
      cmd: /solution
      args: []
      limits:
        wall_time_s: 10
        memory_kb: 262144
        max_processes: 64

  - id: java
    name: Java
    source_filename_strategy: from_request
    artifact_filename_strategy: from_request
    build:
      cmd: /usr/bin/javac
      args: ["{{source}}"]
      limits:
        wall_time_s: 30
        memory_kb: 524288
        max_processes: 100
      flag_allowlist:
        - "-encoding"
        - "UTF-8"
    run:
      cmd: /usr/bin/java
      args: ["{{artifact}}"]
      limits:
        wall_time_s: 10
        memory_kb: 524288
        max_processes: 100

  - id: runflags
    name: Run Flags Test
    source_filename: solution.sh
    run:
      cmd: /bin/bash
      args: ["{{flags}}", "{{source}}"]
      limits:
        wall_time_s: 5
        memory_kb: 1024
        max_processes: 10
      flag_allowlist:
        - "-e"
`

// mockSubmitter satisfies handler.Submitter for unit tests.
type mockSubmitter struct {
	resp   runner.Response
	err    error
	gotReq runner.JobRequest
}

func (m *mockSubmitter) Submit(_ context.Context, req runner.JobRequest) (runner.Response, error) {
	m.gotReq = req
	return m.resp, m.err
}

func newTestRegistry(t *testing.T) *registry.Registry {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "langs-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(testLangsYAML)
	_ = f.Close()
	reg, err := registry.Load(f.Name())
	if err != nil {
		t.Fatalf("load test registry: %v", err)
	}
	return reg
}

func newTestRunHandler(t *testing.T, sub handler.Submitter) http.Handler {
	t.Helper()
	return handler.NewRunHandler(sub, newTestRegistry(t), config.Server{
		MaxSourceBytes: 256 * 1024,
		MaxTests:       50,
		MaxStdinBytes:  64 * 1024,
	})
}

// postJSON sends a JSON POST to the handler and returns the recorder.
func postJSON(t *testing.T, h http.Handler, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func decodeErrResp(t *testing.T, w *httptest.ResponseRecorder) handler.ErrorResponse {
	t.Helper()
	var er handler.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &er); err != nil {
		t.Fatalf("decode error response (body=%s): %v", w.Body.String(), err)
	}
	return er
}

func assertErrorCode(t *testing.T, w *httptest.ResponseRecorder, wantHTTP int, wantCode string) {
	t.Helper()
	if w.Code != wantHTTP {
		t.Fatalf("HTTP status: want %d, got %d (body: %s)", wantHTTP, w.Code, w.Body.String())
	}
	er := decodeErrResp(t, w)
	if er.Error.Code != wantCode {
		t.Fatalf("error code: want %q, got %q (message: %s)", wantCode, er.Error.Code, er.Error.Message)
	}
}

// ── 400 validation paths (all return before Submit is called) ────────────────

func TestRunHandler_InvalidJSON(t *testing.T) {
	h := newTestRunHandler(t, nil)
	req := httptest.NewRequest(http.MethodPost, "/run", strings.NewReader("not-json{"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertErrorCode(t, w, http.StatusBadRequest, "invalid_json")
}

func TestRunHandler_UnknownLanguage(t *testing.T) {
	h := newTestRunHandler(t, nil)
	w := postJSON(t, h, map[string]any{
		"language": "cobol",
		"source":   "IDENTIFICATION DIVISION.",
		"tests":    []map[string]any{{"stdin": "", "expected_stdout": ""}},
	})
	assertErrorCode(t, w, http.StatusBadRequest, "unknown_language")
}

func TestRunHandler_EmptySource(t *testing.T) {
	// Empty source is valid — the pipeline handles it (interpreted languages run
	// an empty file; compiled languages get a build_failed). Verify it reaches
	// the runner and returns 200 with the runner's result.
	mock := &mockSubmitter{resp: runner.Response{
		Status: validate.StatusAccepted,
		Build:  runner.BuildResult{Status: validate.BuildStatusOK},
		Tests:  []runner.TestResult{{Status: validate.StatusAccepted}},
	}}
	h := newTestRunHandler(t, mock)
	w := postJSON(t, h, map[string]any{
		"language": "py3",
		"source":   "",
		"tests":    []map[string]any{{"stdin": "", "expected_stdout": ""}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d; empty source should reach the runner", w.Code)
	}
}

func TestRunHandler_SourceTooLarge(t *testing.T) {
	h := newTestRunHandler(t, nil)
	large := strings.Repeat("x", 256*1024+1)
	w := postJSON(t, h, map[string]any{
		"language": "py3",
		"source":   large,
		"tests":    []map[string]any{{"stdin": "", "expected_stdout": ""}},
	})
	assertErrorCode(t, w, http.StatusBadRequest, "source_too_large")
}

func TestRunHandler_MissingSourceFilename(t *testing.T) {
	h := newTestRunHandler(t, nil)
	w := postJSON(t, h, map[string]any{
		"language": "java",
		"source":   "public class Main {}",
		"tests":    []map[string]any{{"stdin": "", "expected_stdout": ""}},
	})
	assertErrorCode(t, w, http.StatusBadRequest, "missing_source_filename")
}

func TestRunHandler_InvalidSourceFilename(t *testing.T) {
	h := newTestRunHandler(t, nil)
	w := postJSON(t, h, map[string]any{
		"language":        "java",
		"source":          "public class X {}",
		"source_filename": "../../etc/passwd",
		"tests":           []map[string]any{{"stdin": "", "expected_stdout": ""}},
	})
	assertErrorCode(t, w, http.StatusBadRequest, "invalid_filename")
}

func TestRunHandler_MissingArtifactFilename(t *testing.T) {
	h := newTestRunHandler(t, nil)
	w := postJSON(t, h, map[string]any{
		"language":        "java",
		"source":          "public class Main {}",
		"source_filename": "Main.java",
		// artifact_filename intentionally omitted
		"tests": []map[string]any{{"stdin": "", "expected_stdout": ""}},
	})
	assertErrorCode(t, w, http.StatusBadRequest, "missing_artifact_filename")
}

func TestRunHandler_InvalidArtifactFilename(t *testing.T) {
	h := newTestRunHandler(t, nil)
	w := postJSON(t, h, map[string]any{
		"language":          "java",
		"source":            "public class Main {}",
		"source_filename":   "Main.java",
		"artifact_filename": "../escape",
		"tests":             []map[string]any{{"stdin": "", "expected_stdout": ""}},
	})
	assertErrorCode(t, w, http.StatusBadRequest, "invalid_filename")
}

func TestRunHandler_DisallowedBuildFlag(t *testing.T) {
	h := newTestRunHandler(t, nil)
	w := postJSON(t, h, map[string]any{
		"language": "c",
		"source":   `#include <stdio.h> int main(){return 0;}`,
		"build":    map[string]any{"flags": []string{"-fplugin=evil.so"}},
		"tests":    []map[string]any{{"stdin": "", "expected_stdout": ""}},
	})
	assertErrorCode(t, w, http.StatusBadRequest, "invalid_flag")
}

func TestRunHandler_DisallowedRunFlag(t *testing.T) {
	h := newTestRunHandler(t, nil)
	w := postJSON(t, h, map[string]any{
		"language": "runflags",
		"source":   "echo hi",
		"run":      map[string]any{"flags": []string{"-x"}}, // -x not in allowlist
		"tests":    []map[string]any{{"stdin": "", "expected_stdout": "hi\n"}},
	})
	assertErrorCode(t, w, http.StatusBadRequest, "invalid_flag")
}

// No tests means raw execution mode: run once, report the outcome without grading.
func TestRunHandler_RawMode(t *testing.T) {
	sub := &mockSubmitter{resp: runner.Response{
		Status: validate.StatusAccepted,
		Build:  runner.BuildResult{Status: validate.BuildStatusOK},
		Tests:  []runner.TestResult{{Status: validate.StatusAccepted, Stdout: "hi\n", ExitCode: 0}},
	}}
	h := newTestRunHandler(t, sub)
	w := postJSON(t, h, map[string]any{
		"language": "py3",
		"source":   "print('hi')",
		"stdin":    "ignored\n",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (body: %s)", w.Code, w.Body.String())
	}
	if !sub.gotReq.Raw {
		t.Fatal("expected Raw=true when no tests are supplied")
	}
	if len(sub.gotReq.Tests) != 1 || sub.gotReq.Tests[0].Stdin != "ignored\n" {
		t.Fatalf("raw mode should forward a single stdin, got %+v", sub.gotReq.Tests)
	}
}

func TestRunHandler_TooManyTests(t *testing.T) {
	h := newTestRunHandler(t, nil)
	tests := make([]map[string]any, 51)
	for i := range tests {
		tests[i] = map[string]any{"stdin": "", "expected_stdout": ""}
	}
	w := postJSON(t, h, map[string]any{
		"language": "py3",
		"source":   "print('hi')",
		"tests":    tests,
	})
	assertErrorCode(t, w, http.StatusBadRequest, "invalid_test_count")
}

func TestRunHandler_StdinTooLarge(t *testing.T) {
	h := newTestRunHandler(t, nil)
	large := strings.Repeat("x", 64*1024+1)
	w := postJSON(t, h, map[string]any{
		"language": "py3",
		"source":   "print(input())",
		"tests":    []map[string]any{{"stdin": large, "expected_stdout": ""}},
	})
	assertErrorCode(t, w, http.StatusBadRequest, "stdin_too_large")
}

func TestRunHandler_ExpectedTooLarge(t *testing.T) {
	h := newTestRunHandler(t, nil)
	large := strings.Repeat("x", 64*1024+1)
	w := postJSON(t, h, map[string]any{
		"language": "py3",
		"source":   "print('hi')",
		"tests":    []map[string]any{{"stdin": "", "expected_stdout": large}},
	})
	assertErrorCode(t, w, http.StatusBadRequest, "expected_too_large")
}

func TestRunHandler_BuildLimitsTooHigh(t *testing.T) {
	h := newTestRunHandler(t, nil)
	w := postJSON(t, h, map[string]any{
		"language": "c",
		"source":   `#include <stdio.h> int main(){return 0;}`,
		"build":    map[string]any{"limits": map[string]any{"wall_time_s": 99999}},
		"tests":    []map[string]any{{"stdin": "", "expected_stdout": ""}},
	})
	assertErrorCode(t, w, http.StatusBadRequest, "invalid_limits")
}

func TestRunHandler_RunLimitsTooHigh(t *testing.T) {
	h := newTestRunHandler(t, nil)
	w := postJSON(t, h, map[string]any{
		"language": "py3",
		"source":   "print('hi')",
		"run":      map[string]any{"limits": map[string]any{"wall_time_s": 99999}},
		"tests":    []map[string]any{{"stdin": "", "expected_stdout": "hi\n"}},
	})
	assertErrorCode(t, w, http.StatusBadRequest, "invalid_limits")
}

// ── 200 / 500 / 503 paths (require submitter) ─────────────────────────────────

func TestRunHandler_Success(t *testing.T) {
	sub := &mockSubmitter{resp: runner.Response{
		Status: validate.StatusAccepted,
		Build:  runner.BuildResult{Status: validate.BuildStatusOK},
		Tests:  []runner.TestResult{{Status: validate.StatusAccepted, Stdout: "hello\n"}},
	}}
	h := newTestRunHandler(t, sub)
	w := postJSON(t, h, map[string]any{
		"language": "py3",
		"source":   "print('hello')",
		"tests":    []map[string]any{{"stdin": "", "expected_stdout": "hello\n"}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (body: %s)", w.Code, w.Body.String())
	}
	var resp handler.RunResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != validate.StatusAccepted {
		t.Fatalf("status: want %q, got %q", validate.StatusAccepted, resp.Status)
	}
	if resp.Build.Status != validate.BuildStatusOK {
		t.Fatalf("build.status: want %q, got %q", validate.BuildStatusOK, resp.Build.Status)
	}
	if len(resp.Tests) != 1 || resp.Tests[0].Status != validate.StatusAccepted {
		t.Fatalf("tests[0].status: want %q, got %+v", validate.StatusAccepted, resp.Tests)
	}
}

func TestRunHandler_InternalError(t *testing.T) {
	sub := &mockSubmitter{err: context.DeadlineExceeded}
	h := newTestRunHandler(t, sub)

	// Submit returns a non-context error → 500
	sub.err = &internalErr{"nsjail not found"}
	w := postJSON(t, h, map[string]any{
		"language": "py3",
		"source":   "print('hi')",
		"tests":    []map[string]any{{"stdin": "", "expected_stdout": "hi\n"}},
	})
	assertErrorCode(t, w, http.StatusInternalServerError, "internal_error")
}

func TestRunHandler_ContextCancelled(t *testing.T) {
	// The submitter returns the context error; the handler sees ctx.Err() matches → 503.
	sub := &mockSubmitter{err: context.Canceled}
	h := newTestRunHandler(t, sub)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled before the request is handled

	b, _ := json.Marshal(map[string]any{
		"language": "py3",
		"source":   "print('hi')",
		"tests":    []map[string]any{{"stdin": "", "expected_stdout": "hi\n"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestRunHandler_BuildFailed(t *testing.T) {
	sub := &mockSubmitter{resp: runner.Response{
		Status: validate.StatusBuildFailed,
		Build:  runner.BuildResult{Status: validate.BuildStatusFailed, Stderr: "syntax error"},
		Tests:  []runner.TestResult{{Status: validate.StatusNotExecuted}},
	}}
	h := newTestRunHandler(t, sub)
	w := postJSON(t, h, map[string]any{
		"language": "c",
		"source":   "this is not C",
		"tests":    []map[string]any{{"stdin": "", "expected_stdout": ""}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("build failure must return HTTP 200, got %d", w.Code)
	}
	var resp handler.RunResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != validate.StatusBuildFailed {
		t.Fatalf("status: want %q, got %q", validate.StatusBuildFailed, resp.Status)
	}
	if resp.Tests[0].Status != validate.StatusNotExecuted {
		t.Fatalf("tests[0].status: want %q, got %q", validate.StatusNotExecuted, resp.Tests[0].Status)
	}
}

func TestRunHandler_WrongOutput(t *testing.T) {
	sub := &mockSubmitter{resp: runner.Response{
		Status: validate.StatusWrongOutput,
		Build:  runner.BuildResult{Status: validate.BuildStatusOK},
		Tests:  []runner.TestResult{{Status: validate.StatusWrongOutput, Stdout: "actual\n"}},
	}}
	h := newTestRunHandler(t, sub)
	w := postJSON(t, h, map[string]any{
		"language": "py3",
		"source":   "print('actual')",
		"tests":    []map[string]any{{"stdin": "", "expected_stdout": "expected\n"}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("wrong output must return HTTP 200, got %d", w.Code)
	}
	var resp handler.RunResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != validate.StatusWrongOutput {
		t.Fatalf("status: want %q, got %q", validate.StatusWrongOutput, resp.Status)
	}
}

func TestRunHandler_MultipleTests_FirstFailWins(t *testing.T) {
	sub := &mockSubmitter{resp: runner.Response{
		Status: validate.StatusWrongOutput,
		Build:  runner.BuildResult{Status: validate.BuildStatusOK},
		Tests: []runner.TestResult{
			{Status: validate.StatusAccepted},
			{Status: validate.StatusWrongOutput},
			{Status: validate.StatusTimeExceeded},
		},
	}}
	h := newTestRunHandler(t, sub)
	tests := make([]map[string]any, 3)
	for i := range tests {
		tests[i] = map[string]any{"stdin": "", "expected_stdout": ""}
	}
	w := postJSON(t, h, map[string]any{
		"language": "py3",
		"source":   "print('x')",
		"tests":    tests,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var resp handler.RunResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != validate.StatusWrongOutput {
		t.Fatalf("top-level status: want %q, got %q", validate.StatusWrongOutput, resp.Status)
	}
}

// internalErr is a sentinel error for testing 500 responses.
type internalErr struct{ msg string }

func (e *internalErr) Error() string { return e.msg }
