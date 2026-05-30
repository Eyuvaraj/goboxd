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

// Submitter allows dependency injection in tests.
type Submitter interface {
	Submit(ctx context.Context, req runner.JobRequest) (runner.Response, error)
}

type RunHandler struct {
	runner Submitter
	reg    *registry.Registry
	cfg    config.Server
}

func NewRunHandler(r Submitter, reg *registry.Registry, cfg config.Server) *RunHandler {
	return &RunHandler{runner: r, reg: reg, cfg: cfg}
}

func (h *RunHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	lang := h.reg.Get(req.Language)
	if lang == nil {
		writeError(w, http.StatusBadRequest, "unknown_language",
			"language "+strconv.Quote(req.Language)+" is not registered")
		return
	}

	if err := validate.SourceSize(req.Source, h.cfg.MaxSourceBytes); err != nil {
		writeError(w, http.StatusBadRequest, "source_too_large", err.Error())
		return
	}

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

	if err := validate.TestCount(len(req.Tests), h.cfg.MaxTests); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_test_count", err.Error())
		return
	}
	var tests []runner.TestCase
	if len(req.Tests) == 0 {
		if err := validate.StdinSize(req.Stdin, h.cfg.MaxStdinBytes); err != nil {
			writeError(w, http.StatusBadRequest, "stdin_too_large", err.Error())
			return
		}
		tests = []runner.TestCase{{Stdin: req.Stdin}}
	} else {
		tests = make([]runner.TestCase, len(req.Tests))
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
	}

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
		Raw:              len(req.Tests) == 0,
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
			ExitCode:     t.ExitCode,
			DurationMs:   t.DurationMs,
			MemoryPeakKB: t.MemoryPeakKB,
		}
	}

	writeJSON(w, http.StatusOK, out)
}

type apiErr struct {
	code    string
	message string
}

// resolveFilename returns the filename for a phase. If strategy is "from_request",
// the client must supply a non-empty, valid name; otherwise the language's fixed
// filename is used.
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
