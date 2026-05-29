package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/thesouldev/goboxd/internal/config"
	"github.com/thesouldev/goboxd/internal/logctx"
	"github.com/thesouldev/goboxd/internal/registry"
	"github.com/thesouldev/goboxd/internal/runner"
	"github.com/thesouldev/goboxd/internal/validate"
)

// Submitter abstracts runner.Runner.Submit to allow dependency injection in tests.
type Submitter interface {
	Submit(ctx context.Context, req runner.JobRequest) (runner.Response, error)
}

// RunHandler handles POST /run.
type RunHandler struct {
	runner Submitter
	reg    *registry.Registry
	cfg    config.Server
}

func NewRunHandler(r Submitter, reg *registry.Registry, cfg config.Server) *RunHandler {
	return &RunHandler{runner: r, reg: reg, cfg: cfg}
}

// ServeHTTP godoc
//
//	@Summary		Execute code in a sandbox
//	@Description	Compiles (if needed) and runs the submitted source against one or more test cases inside an nsjail sandbox.
//	@Description
//	@Description	**Result encoding:** HTTP 200 is returned for all structurally valid requests. Execution outcomes (build failure, wrong output, TLE, MLE, runtime error) are encoded in the `status` fields of the response body — not as HTTP error codes.
//	@Description
//	@Description	**Filename requirements:** Some languages (e.g. Java) require `source_filename` and `artifact_filename` to match the public class name. The `strategy` field in the language definition controls this.
//	@Description
//	@Description	**Flag allowlists:** Build and run flags are filtered against a per-language allowlist. Disallowed flags return 400 `invalid_flag`.
//	@Description
//	@Description	**Limit overrides:** Per-request limits may only reduce a language's configured maximum — attempting to exceed the language ceiling returns 400 `invalid_limits`.
//	@Description
//	@Description	---
//	@Description
//	@Description	**Sample request (C++, one test case):**
//	@Description	```json
//	@Description	{
//	@Description	  "language": "cpp",
//	@Description	  "source": "#include <iostream>\nint main(){std::cout<<\"hi\";}",
//	@Description	  "source_filename": "solution.cpp",
//	@Description	  "artifact_filename": "solution",
//	@Description	  "build": {
//	@Description	    "limits": { "wall_time_s": 5, "memory_kb": 1048576, "max_processes": 100 },
//	@Description	    "flags": ["-O2"]
//	@Description	  },
//	@Description	  "run": {
//	@Description	    "limits": { "wall_time_s": 3, "memory_kb": 524288, "max_processes": 64 },
//	@Description	    "flags": []
//	@Description	  },
//	@Description	  "tests": [
//	@Description	    { "stdin": "1\n", "expected_stdout": "hi" }
//	@Description	  ]
//	@Description	}
//	@Description	```
//	@Description
//	@Description	**Sample response** (code printed `"HI"` instead of `"hi"` — wrong_output):
//	@Description	```json
//	@Description	{
//	@Description	  "status": "wrong_output",
//	@Description	  "build": { "status": "ok", "stdout": "", "stderr": "", "duration_ms": 412 },
//	@Description	  "tests": [
//	@Description	    { "status": "wrong_output", "stdout": "HI", "stderr": "", "duration_ms": 38, "memory_peak_kb": 8192 }
//	@Description	  ]
//	@Description	}
//	@Description	```
//	@Tags			execution
//	@Accept			json
//	@Produce		json
//	@Param			body	body		RunRequest		true	"Code execution request"
//	@Success		200		{object}	RunResponse		"Execution completed (check body status fields for pass/fail)"
//	@Failure		400		{object}	ErrorResponse	"Validation error — see error.code for the specific cause"
//	@Failure		500		{object}	ErrorResponse	"Internal server error (nsjail fault, disk full, etc.)"
//	@Failure		503		{string}	string			"Server at capacity or request cancelled by client"
//	@Router			/run [post]
func (h *RunHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	// --- Validate language ---
	lang := h.reg.Get(req.Language)
	if lang == nil {
		writeError(w, http.StatusBadRequest, "unknown_language",
			"language "+strconv.Quote(req.Language)+" is not registered")
		return
	}

	// --- Validate source ---
	if err := validate.SourceSize(req.Source, h.cfg.MaxSourceBytes); err != nil {
		writeError(w, http.StatusBadRequest, "source_too_large", err.Error())
		return
	}

	// --- Validate filenames (security hole #1) ---
	srcFilename, aerr := resolveFilename("source", lang.SourceFilenameStrategy, req.SourceFilename, lang.SourceFilename)
	if aerr != nil {
		writeError(w, http.StatusBadRequest, aerr.code, aerr.message)
		return
	}
	artFilename, aerr := resolveFilename("artifact", lang.ArtifactFilenameStrategy, req.ArtifactFilename, lang.ArtifactFilename)
	if aerr != nil {
		writeError(w, http.StatusBadRequest, aerr.code, aerr.message)
		return
	}

	// --- Validate flags (security hole #3) ---
	var buildFlags, runFlags []string
	if req.Build != nil {
		buildFlags = req.Build.Flags
		if lang.Build != nil && len(buildFlags) > 0 {
			if err := validate.Flags(buildFlags, lang.Build.FlagAllowlist); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_flag", err.Error())
				return
			}
		}
	}
	if req.Run != nil {
		runFlags = req.Run.Flags
		if len(runFlags) > 0 {
			if err := validate.Flags(runFlags, lang.Run.FlagAllowlist); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_flag", err.Error())
				return
			}
		}
	}

	// --- Validate tests ---
	if err := validate.TestCount(len(req.Tests), h.cfg.MaxTests); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_test_count", err.Error())
		return
	}
	tests := make([]runner.TestCase, len(req.Tests))
	for i, tc := range req.Tests {
		if err := validate.StdinSize(tc.Stdin, h.cfg.MaxStdinBytes); err != nil {
			writeError(w, http.StatusBadRequest, "stdin_too_large",
				"test "+strconv.Itoa(i)+": "+err.Error())
			return
		}
		if err := validate.ExpectedSize(tc.ExpectedStdout, h.cfg.MaxStdinBytes); err != nil {
			writeError(w, http.StatusBadRequest, "expected_too_large",
				"test "+strconv.Itoa(i)+": "+err.Error())
			return
		}
		tests[i] = runner.TestCase{
			Stdin:          tc.Stdin,
			ExpectedStdout: tc.ExpectedStdout,
		}
	}

	// --- Validate and merge per-request limits ---
	// Clients may not exceed the language's configured defaults (prevents semaphore starvation).
	var buildLimits, runLimits config.LimitsDef
	if req.Build != nil {
		buildLimits = req.Build.Limits
		if lang.Build != nil {
			if err := validate.Limits(buildLimits, lang.Build.Limits); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_limits", "build.limits: "+err.Error())
				return
			}
		}
	}
	if req.Run != nil {
		runLimits = req.Run.Limits
		if err := validate.Limits(runLimits, lang.Run.Limits); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_limits", "run.limits: "+err.Error())
			return
		}
	}

	// --- Submit job ---
	jobReq := runner.JobRequest{
		Language:         req.Language,
		Source:           req.Source,
		SourceFilename:   srcFilename,
		ArtifactFilename: artFilename,
		BuildFlags:       buildFlags,
		RunFlags:         runFlags,
		BuildLimits:      buildLimits,
		RunLimits:        runLimits,
		Tests:            tests,
	}

	resp, err := h.runner.Submit(r.Context(), jobReq)
	if err != nil {
		if errors.Is(err, r.Context().Err()) {
			http.Error(w, "request cancelled", http.StatusServiceUnavailable)
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	// Write execution fields into context so StructuredLogger can include them.
	accepted := 0
	for _, t := range resp.Tests {
		if t.Status == validate.StatusAccepted {
			accepted++
		}
	}
	*r = *r.WithContext(logctx.Set(r.Context(), logctx.Fields{
		Language:        req.Language,
		ExecStatus:      resp.Status,
		BuildDurationMs: resp.Build.DurationMs,
		TestsTotal:      len(resp.Tests),
		TestsAccepted:   accepted,
	}))

	// Map runner response to HTTP response types.
	out := RunResponse{
		Status: resp.Status,
		Build: BuildResult{
			Status:     resp.Build.Status,
			Stdout:     resp.Build.Stdout,
			Stderr:     resp.Build.Stderr,
			DurationMs: resp.Build.DurationMs,
		},
		Tests: make([]TestResult, len(resp.Tests)),
	}
	for i, t := range resp.Tests {
		out.Tests[i] = TestResult{
			Status:       t.Status,
			Stdout:       t.Stdout,
			Stderr:       t.Stderr,
			DurationMs:   t.DurationMs,
			MemoryPeakKB: t.MemoryPeakKB,
		}
	}

	writeJSON(w, http.StatusOK, out)
}

// apiErr is a validation failure carrying the response error code and message.
type apiErr struct {
	code    string
	message string
}

// resolveFilename returns the filename to use for a phase. When the language's
// strategy is "from_request" the client must supply a valid name; otherwise the
// language's fixed filename is used. field is "source" or "artifact" and selects
// the error code on missing input. A non-nil result means the request is invalid.
func resolveFilename(field, strategy, requested, fixed string) (string, *apiErr) {
	if strategy != "from_request" {
		return fixed, nil
	}
	if requested == "" {
		return "", &apiErr{
			code:    "missing_" + field + "_filename",
			message: field + "_filename is required for this language",
		}
	}
	if err := validate.Filename(requested); err != nil {
		return "", &apiErr{code: "invalid_filename", message: err.Error()}
	}
	return requested, nil
}

func writeError(w http.ResponseWriter, code int, errCode, msg string) {
	writeJSON(w, code, ErrorResponse{
		Error: ErrorDetail{Code: errCode, Message: msg},
	})
}
