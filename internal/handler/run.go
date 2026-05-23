package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/thesouldev/goboxd/internal/config"
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
				"test "+itoa(i)+": "+err.Error())
			return
		}
		tests[i] = runner.TestCase{
			Stdin:          tc.Stdin,
			ExpectedStdout: tc.ExpectedStdout,
		}
	}

	// --- Merge per-request limits ---
	var buildLimits, runLimits config.LimitsDef
	if req.Build != nil {
		buildLimits = req.Build.Limits
	}
	if req.Run != nil {
		runLimits = req.Run.Limits
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

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
