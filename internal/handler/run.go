package handler

import (
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

// RunHandler handles POST /run.
type RunHandler struct {
	runner *runner.Runner
	reg    *registry.Registry
	cfg    config.Server
}

func NewRunHandler(r *runner.Runner, reg *registry.Registry, cfg config.Server) *RunHandler {
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
			"language "+strQ(req.Language)+" is not registered")
		return
	}

	// --- Validate source ---
	if req.Source == "" {
		writeError(w, http.StatusBadRequest, "missing_source", "source is required")
		return
	}
	if err := validate.SourceSize(req.Source, h.cfg.MaxSourceBytes); err != nil {
		writeError(w, http.StatusBadRequest, "source_too_large", err.Error())
		return
	}

	// --- Validate filenames (security hole #1) ---
	srcFilename := req.SourceFilename
	if lang.SourceFilenameStrategy == "from_request" {
		if srcFilename == "" {
			writeError(w, http.StatusBadRequest, "missing_source_filename",
				"source_filename is required for this language")
			return
		}
		if err := validate.Filename(srcFilename); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_filename", err.Error())
			return
		}
	} else {
		srcFilename = lang.SourceFilename
	}

	artFilename := req.ArtifactFilename
	if lang.ArtifactFilenameStrategy == "from_request" {
		if artFilename == "" {
			writeError(w, http.StatusBadRequest, "missing_artifact_filename",
				"artifact_filename is required for this language")
			return
		}
		if err := validate.Filename(artFilename); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_filename", err.Error())
			return
		}
	} else {
		artFilename = lang.ArtifactFilename
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
	if req.Run != nil && len(lang.Run.FlagAllowlist) > 0 {
		runFlags = req.Run.Flags
		if err := validate.Flags(runFlags, lang.Run.FlagAllowlist); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_flag", err.Error())
			return
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

func writeError(w http.ResponseWriter, code int, errCode, msg string) {
	writeJSON(w, code, ErrorResponse{
		Error: ErrorDetail{Code: errCode, Message: msg},
	})
}

func strQ(s string) string { return `"` + s + `"` }
