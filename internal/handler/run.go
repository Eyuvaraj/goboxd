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

// ServeHTTP handles POST /run — the competition contract (COMPETITION.md §4.2),
// where tests is required and the response has no exit_code.
func (h *RunHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, false)
}

// ServeHTTPV1 handles POST /v1/run. It adds raw execution — an empty tests
// array runs the program once against req.Stdin and reports the outcome
// without grading — and an exit_code on each test result.
func (h *RunHandler) ServeHTTPV1(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, true)
}

func (h *RunHandler) serve(w http.ResponseWriter, r *http.Request, v1 bool) {
	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusBadRequest, "source_too_large", "request body too large")
			return
		}
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

	// Evaluator mode is /v1/run only and requires tests to grade.
	var evaluator *runner.EvaluatorJob
	if v1 && req.Evaluator != nil {
		evaluator, aerr = h.buildEvaluator(&req)
		if aerr != nil {
			writeError(w, http.StatusBadRequest, aerr.code, aerr.message)
			return
		}
	}

	// Raw execution is /v1/run only: an empty tests array runs once against stdin.
	raw := v1 && len(req.Tests) == 0 && evaluator == nil
	tests, aerr := h.buildTests(&req, raw)
	if aerr != nil {
		writeError(w, http.StatusBadRequest, aerr.code, aerr.message)
		return
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
		Raw:              raw,
		Evaluator:        evaluator,
	}

	resp, err := h.runner.Submit(r.Context(), jobReq)
	if err != nil {
		if errors.Is(err, runner.ErrOverloaded) {
			w.Header().Set("Retry-After", "1")
			writeError(w, http.StatusServiceUnavailable, "overloaded", "server is at capacity, retry shortly")
			return
		}
		if errors.Is(err, r.Context().Err()) {
			http.Error(w, "request cancelled", http.StatusServiceUnavailable)
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	accepted := 0
	totalCpuMs := resp.Build.CpuMs
	for _, t := range resp.Tests {
		if t.Status == validate.StatusAccepted {
			accepted++
		}
		totalCpuMs += t.CpuMs
	}
	*r = *r.WithContext(logctx.Set(r.Context(), logctx.Fields{
		Language:        req.Language,
		ExecStatus:      resp.Status,
		BuildDurationMs: resp.Build.DurationMs,
		TotalCpuMs:      totalCpuMs,
		TestsTotal:      len(resp.Tests),
		TestsAccepted:   accepted,
	}))

	if v1 {
		writeJSON(w, http.StatusOK, toV1Response(resp))
	} else {
		writeJSON(w, http.StatusOK, toRunResponse(resp))
	}
}

// buildTests validates and converts the request's test cases. In raw mode it
// returns a single case fed from req.Stdin; otherwise at least one test is
// required and each stdin/expected_stdout pair is size-checked.
func (h *RunHandler) buildTests(req *RunRequest, raw bool) ([]runner.TestCase, *apiErr) {
	if raw {
		if err := validate.StdinSize(req.Stdin, h.cfg.MaxStdinBytes); err != nil {
			return nil, &apiErr{"stdin_too_large", err.Error()}
		}
		return []runner.TestCase{{Stdin: req.Stdin}}, nil
	}

	if err := validate.TestCount(len(req.Tests), h.cfg.MaxTests); err != nil {
		return nil, &apiErr{"invalid_test_count", err.Error()}
	}
	tests := make([]runner.TestCase, len(req.Tests))
	for i, tc := range req.Tests {
		if err := validate.StdinSize(tc.Stdin, h.cfg.MaxStdinBytes); err != nil {
			return nil, &apiErr{"stdin_too_large", "test " + strconv.Itoa(i) + ": " + err.Error()}
		}
		if err := validate.ExpectedSize(tc.ExpectedStdout, h.cfg.MaxStdinBytes); err != nil {
			return nil, &apiErr{"expected_too_large", "test " + strconv.Itoa(i) + ": " + err.Error()}
		}
		tests[i] = runner.TestCase{Stdin: tc.Stdin, ExpectedStdout: tc.ExpectedStdout}
	}
	return tests, nil
}

// buildEvaluator validates the evaluator spec and resolves it into a job. It
// reuses the same filename/flag/limit checks as the candidate program.
func (h *RunHandler) buildEvaluator(req *RunRequest) (*runner.EvaluatorJob, *apiErr) {
	e := req.Evaluator
	lang := h.reg.Get(e.Language)
	if lang == nil {
		return nil, &apiErr{"unknown_language", "evaluator language " + strconv.Quote(e.Language) + " is not registered"}
	}
	if err := validate.SourceSize(e.Source, h.cfg.MaxSourceBytes); err != nil {
		return nil, &apiErr{"source_too_large", "evaluator: " + err.Error()}
	}

	srcFilename, aerr := resolveFilename("source", lang.SourceFilenameStrategy, e.SourceFilename, lang.SourceFilename)
	if aerr != nil {
		return nil, aerr
	}
	artFilename, aerr := resolveFilename("artifact", lang.ArtifactFilenameStrategy, e.ArtifactFilename, lang.ArtifactFilename)
	if aerr != nil {
		return nil, aerr
	}

	var buildFlags, runFlags []string
	var buildLimits, runLimits config.LimitsDef
	if e.Build != nil && lang.Build != nil {
		buildFlags = e.Build.Flags
		buildLimits = e.Build.Limits
		if len(buildFlags) > 0 {
			if err := validate.Flags(buildFlags, lang.Build.FlagAllowlist); err != nil {
				return nil, &apiErr{"invalid_flag", "evaluator build: " + err.Error()}
			}
		}
		if err := validate.Limits(buildLimits, lang.Build.Limits); err != nil {
			return nil, &apiErr{"invalid_limits", "evaluator build.limits: " + err.Error()}
		}
	}
	if e.Run != nil {
		runFlags = e.Run.Flags
		runLimits = e.Run.Limits
		if len(runFlags) > 0 {
			if err := validate.Flags(runFlags, lang.Run.FlagAllowlist); err != nil {
				return nil, &apiErr{"invalid_flag", "evaluator run: " + err.Error()}
			}
		}
		if err := validate.Limits(runLimits, lang.Run.Limits); err != nil {
			return nil, &apiErr{"invalid_limits", "evaluator run.limits: " + err.Error()}
		}
	}

	return &runner.EvaluatorJob{
		Language:         e.Language,
		Source:           e.Source,
		SourceFilename:   srcFilename,
		ArtifactFilename: artFilename,
		BuildFlags:       buildFlags,
		RunFlags:         runFlags,
		BuildLimits:      buildLimits,
		RunLimits:        runLimits,
	}, nil
}

func toBuildResult(b runner.BuildResult) BuildResult {
	return BuildResult{
		Status:     b.Status,
		Stdout:     b.Stdout,
		Stderr:     b.Stderr,
		DurationMs: b.DurationMs,
	}
}

func toRunResponse(resp runner.Response) RunResponse {
	out := RunResponse{
		Status: resp.Status,
		Build:  toBuildResult(resp.Build),
		Tests:  make([]TestResult, len(resp.Tests)),
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
	return out
}

func toV1Response(resp runner.Response) V1RunResponse {
	out := V1RunResponse{
		Status: resp.Status,
		Build:  toBuildResult(resp.Build),
		Tests:  make([]V1TestResult, len(resp.Tests)),
	}
	for i, t := range resp.Tests {
		out.Tests[i] = V1TestResult{
			Status:       t.Status,
			Stdout:       t.Stdout,
			Stderr:       t.Stderr,
			ExitCode:     t.ExitCode,
			DurationMs:   t.DurationMs,
			MemoryPeakKB: t.MemoryPeakKB,
			Verdict:      t.Verdict,
			Score:        t.Score,
			Message:      t.Message,
		}
	}
	return out
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
		// The language uses a fixed server-side filename, so the client value is
		// not used. Still reject a malformed one if supplied, per the contract's
		// "must be a single path component" rule, rather than silently ignoring it.
		if requested != "" {
			if err := validate.Filename(requested); err != nil {
				return "", &apiErr{code: "invalid_filename", message: err.Error()}
			}
		}
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
