package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thesouldev/goboxd/internal/config"
	"github.com/thesouldev/goboxd/internal/sandbox"
	"github.com/thesouldev/goboxd/internal/validate"
)

type BuildResult struct {
	Status     string `json:"status"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	DurationMs int64  `json:"duration_ms"`
}

type TestResult struct {
	Status       string `json:"status"`
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr"`
	ExitCode     int    `json:"exit_code"`
	DurationMs   int64  `json:"duration_ms"`
	MemoryPeakKB int64  `json:"memory_peak_kb"`
	// Set only in evaluator mode (see EvaluatorJob).
	Verdict string
	Score   *float64
	Message string
}

type JobRequest struct {
	Language         string
	Source           string
	SourceFilename   string
	ArtifactFilename string
	BuildFlags       []string
	RunFlags         []string
	BuildLimits      config.LimitsDef
	RunLimits        config.LimitsDef
	Tests            []TestCase
	// Raw reports execution outcome only, without comparing stdout to an
	// expected value. Set when the request carries no test cases.
	Raw bool
	// Evaluator, when set, grades each test with a custom program instead of
	// comparing stdout to ExpectedStdout. Mutually exclusive with Raw.
	Evaluator *EvaluatorJob
}

type TestCase struct {
	Stdin          string
	ExpectedStdout string
}

// EvaluatorJob is a scoring program supplied with the request. Per test it is
// run in its own jail with three files in its working directory — "input"
// (the test stdin), "expected" (the test expected_stdout), and "output" (the
// candidate's actual stdout) — and must print a JSON verdict to stdout:
//
//	{"verdict": "accepted" | "rejected", "score": <0..1>, "message": "..."}
type EvaluatorJob struct {
	Language         string
	Source           string
	SourceFilename   string
	ArtifactFilename string
	BuildFlags       []string
	RunFlags         []string
	BuildLimits      config.LimitsDef
	RunLimits        config.LimitsDef
}

type Job struct {
	req      JobRequest
	lang     *config.LanguageDef
	evalLang *config.LanguageDef // resolved evaluator language; nil unless evaluator mode
	ws       *sandbox.Workspace
	cfg      JobConfig
}

type JobConfig struct {
	NsjailPath     string
	MaxOutputBytes int64
	BindMounts     []string
	JailDir        string // parent dir for the evaluator's own workspace
}

func newJob(req JobRequest, lang, evalLang *config.LanguageDef, ws *sandbox.Workspace, cfg JobConfig) *Job {
	return &Job{req: req, lang: lang, evalLang: evalLang, ws: ws, cfg: cfg}
}

// compile runs the build phase; returns ok immediately for interpreted languages.
func (j *Job) compile(ctx context.Context) BuildResult {
	if !j.lang.IsCompiled() {
		return BuildResult{Status: validate.BuildStatusOK}
	}

	srcPath := j.ws.SourcePath(j.req.SourceFilename)
	if err := os.WriteFile(srcPath, []byte(j.req.Source), 0o644); err != nil {
		return BuildResult{Status: validate.BuildStatusInternalError, Stderr: err.Error()}
	}

	limits := sandbox.MergeLimits(j.lang.Build.Limits, j.req.BuildLimits)
	expandedArgs := sandbox.ExpandArgs(
		j.lang.Build.Args,
		j.req.SourceFilename,
		j.req.ArtifactFilename,
		j.req.BuildFlags,
	)

	rcfg := sandbox.RunConfig{
		NsjailPath:     j.cfg.NsjailPath,
		WorkspaceDir:   j.ws.Dir,
		Limits:         limits,
		Cmd:            j.lang.Build.Cmd,
		Args:           expandedArgs,
		MaxOutputBytes: j.cfg.MaxOutputBytes,
		BindMounts:     buildBindMounts(j.lang),
		Env:            j.lang.Env,
	}

	result, err := sandbox.Run(ctx, rcfg)
	if err != nil {
		return BuildResult{Status: validate.BuildStatusInternalError, Stderr: fmt.Sprintf("sandbox error: %v", err)}
	}

	status := sandbox.ParseBuildStatus(result.Log, result.ExitCode)
	if status == validate.BuildStatusOK && j.req.ArtifactFilename != "" {
		// Lock down the compiled artifact and source to prevent test[i] from
		// overwriting the binary before test[i+1] runs (shared workspace).
		_ = os.Chmod(j.ws.SourcePath(j.req.ArtifactFilename), 0o555)
		_ = os.Chmod(j.ws.SourcePath(j.req.SourceFilename), 0o444)
	}
	return BuildResult{
		Status:     status,
		Stdout:     string(result.Stdout),
		Stderr:     string(result.Stderr),
		DurationMs: result.DurationMs,
	}
}

func (j *Job) runTests(ctx context.Context, buildStatus string) []TestResult {
	results := make([]TestResult, len(j.req.Tests))

	if buildStatus != validate.BuildStatusOK {
		for i := range results {
			results[i] = TestResult{Status: validate.StatusNotExecuted}
		}
		return results
	}

	// Write source for interpreted languages (compiled ones did it in compile()).
	if !j.lang.IsCompiled() {
		srcPath := j.ws.SourcePath(j.req.SourceFilename)
		if err := os.WriteFile(srcPath, []byte(j.req.Source), 0o644); err != nil {
			for i := range results {
				results[i] = TestResult{Status: validate.StatusInternalError}
			}
			return results
		}
	}

	limits := sandbox.MergeLimits(j.lang.Run.Limits, j.req.RunLimits)
	expandedArgs := sandbox.ExpandArgs(
		j.lang.Run.Args,
		j.req.SourceFilename,
		j.req.ArtifactFilename,
		j.req.RunFlags,
	)

	// Evaluator mode: prepare the scoring program once, reused across tests.
	var ec *evalContext
	if j.req.Evaluator != nil {
		var setupErr string
		ec, setupErr = j.setupEvaluator(ctx)
		if setupErr != "" {
			for i := range results {
				results[i] = TestResult{Status: validate.StatusInternalError, Message: setupErr}
			}
			return results
		}
		defer ec.ws.Cleanup()
	}

	for i, tc := range j.req.Tests {
		testDir, err := j.ws.TestDir(i)
		if err != nil {
			results[i] = TestResult{Status: validate.StatusInternalError}
			continue
		}

		stdinPath := filepath.Join(testDir, "stdin")
		if err := os.WriteFile(stdinPath, []byte(tc.Stdin), 0o644); err != nil {
			results[i] = TestResult{Status: validate.StatusInternalError}
			continue
		}

		f, err := os.Open(stdinPath)
		if err != nil {
			results[i] = TestResult{Status: validate.StatusInternalError}
			continue
		}

		rcfg := sandbox.RunConfig{
			NsjailPath:     j.cfg.NsjailPath,
			WorkspaceDir:   j.ws.Dir,
			Limits:         limits,
			Cmd:            j.lang.Run.Cmd,
			Args:           expandedArgs,
			Stdin:          f,
			MaxOutputBytes: j.cfg.MaxOutputBytes,
			BindMounts:     buildBindMounts(j.lang),
			Env:            j.lang.Env,
		}

		result, runErr := sandbox.Run(ctx, rcfg)
		_ = f.Close()

		if runErr != nil {
			results[i] = TestResult{Status: validate.StatusInternalError}
			continue
		}

		tr := TestResult{
			Stdout:       string(result.Stdout),
			Stderr:       string(result.Stderr),
			ExitCode:     result.ExitCode,
			DurationMs:   result.DurationMs,
			MemoryPeakKB: result.MemoryPeakKB,
		}
		sandboxStatus := sandbox.ParseRunStatus(result.Log, result.ExitCode, result.OOMKilled)
		switch {
		case ec != nil:
			// Only grade output the candidate actually produced; a crash or
			// timeout is reported as-is, the evaluator never sees it.
			if sandboxStatus == validate.StatusAccepted {
				tr.Status, tr.Verdict, tr.Score, tr.Message = j.gradeWithEvaluator(ctx, ec, tc, result.Stdout)
			} else {
				tr.Status = sandboxStatus
			}
		case j.req.Raw:
			tr.Status = sandboxStatus
		default:
			tr.Status = compareOutput(result, tc.ExpectedStdout)
		}
		results[i] = tr
	}
	return results
}

// evalContext is the prepared evaluator — its own jail workspace plus the
// resolved run limits and argv — built once and reused for every test.
type evalContext struct {
	ws     *sandbox.Workspace
	limits config.LimitsDef
	args   []string
}

// setupEvaluator writes the evaluator source into a fresh workspace and, for
// compiled languages, builds it. It returns a non-empty error string if the
// evaluator could not be prepared. Like all sandboxed execution it requires
// nsjail and Linux cgroup v2 — it cannot run on macOS.
func (j *Job) setupEvaluator(ctx context.Context) (*evalContext, string) {
	ev := j.req.Evaluator
	ws, err := sandbox.NewWorkspace(j.cfg.JailDir)
	if err != nil {
		return nil, "evaluator workspace: " + err.Error()
	}

	if err := os.WriteFile(ws.SourcePath(ev.SourceFilename), []byte(ev.Source), 0o644); err != nil {
		ws.Cleanup()
		return nil, "writing evaluator source: " + err.Error()
	}

	if j.evalLang.IsCompiled() {
		res, err := sandbox.Run(ctx, sandbox.RunConfig{
			NsjailPath:     j.cfg.NsjailPath,
			WorkspaceDir:   ws.Dir,
			Limits:         sandbox.MergeLimits(j.evalLang.Build.Limits, ev.BuildLimits),
			Cmd:            j.evalLang.Build.Cmd,
			Args:           sandbox.ExpandArgs(j.evalLang.Build.Args, ev.SourceFilename, ev.ArtifactFilename, ev.BuildFlags),
			MaxOutputBytes: j.cfg.MaxOutputBytes,
			BindMounts:     buildBindMounts(j.evalLang),
			Env:            j.evalLang.Env,
		})
		if err != nil || sandbox.ParseBuildStatus(res.Log, res.ExitCode) != validate.BuildStatusOK {
			ws.Cleanup()
			return nil, "evaluator build failed: " + strings.TrimSpace(string(res.Stderr))
		}
		_ = os.Chmod(ws.SourcePath(ev.ArtifactFilename), 0o555)
	}

	return &evalContext{
		ws:     ws,
		limits: sandbox.MergeLimits(j.evalLang.Run.Limits, ev.RunLimits),
		args:   sandbox.ExpandArgs(j.evalLang.Run.Args, ev.SourceFilename, ev.ArtifactFilename, ev.RunFlags),
	}, ""
}

// gradeWithEvaluator runs the evaluator against one test and maps its verdict to
// a test status. The evaluator reads "input", "expected", and "output" from its
// working directory and writes a JSON verdict to stdout (see EvaluatorJob).
func (j *Job) gradeWithEvaluator(ctx context.Context, ec *evalContext, tc TestCase, candidateOut []byte) (string, string, *float64, string) {
	for name, data := range map[string][]byte{
		"input":    []byte(tc.Stdin),
		"expected": []byte(tc.ExpectedStdout),
		"output":   candidateOut,
	} {
		if err := os.WriteFile(filepath.Join(ec.ws.Dir, name), data, 0o644); err != nil {
			return validate.StatusInternalError, "", nil, "evaluator io: " + err.Error()
		}
	}

	res, err := sandbox.Run(ctx, sandbox.RunConfig{
		NsjailPath:     j.cfg.NsjailPath,
		WorkspaceDir:   ec.ws.Dir,
		Limits:         ec.limits,
		Cmd:            j.evalLang.Run.Cmd,
		Args:           ec.args,
		MaxOutputBytes: j.cfg.MaxOutputBytes,
		BindMounts:     buildBindMounts(j.evalLang),
		Env:            j.evalLang.Env,
	})
	if err != nil || sandbox.ParseRunStatus(res.Log, res.ExitCode, res.OOMKilled) != validate.StatusAccepted {
		return validate.StatusInternalError, "", nil, "evaluator did not complete: " + strings.TrimSpace(string(res.Stderr))
	}

	var v struct {
		Verdict string   `json:"verdict"`
		Score   *float64 `json:"score"`
		Message string   `json:"message"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(res.Stdout), &v); err != nil || v.Verdict == "" {
		return validate.StatusInternalError, "", nil, "evaluator emitted an invalid verdict"
	}

	status := validate.StatusWrongOutput
	if v.Verdict == "accepted" {
		status = validate.StatusAccepted
	}
	return status, v.Verdict, v.Score, v.Message
}

// compareOutput returns test status. Sandbox failures take precedence.
// Strips trailing whitespace to catch common newline differences without
// masking leading-whitespace differences.
func compareOutput(result sandbox.RunResult, expected string) string {
	sandboxStatus := sandbox.ParseRunStatus(result.Log, result.ExitCode, result.OOMKilled)
	if sandboxStatus != validate.StatusAccepted {
		return sandboxStatus
	}

	actual := result.Stdout
	exp := []byte(expected)
	if bytes.Equal(actual, exp) {
		return validate.StatusAccepted
	}
	if bytes.Equal(bytes.TrimRight(actual, " \t\r\n"), bytes.TrimRight(exp, " \t\r\n")) {
		return validate.StatusWhitespaceMismatch
	}
	return validate.StatusWrongOutput
}

// buildBindMounts returns read-only host directories to bind-mount into the jail.
// Never add a path whose parent is already in the set: nsjail's MS_RDONLY remount
// of a child mount over its parent fails with EINVAL.
func buildBindMounts(lang *config.LanguageDef) []string {
	dirs := map[string]struct{}{}

	// /dev is needed for /dev/null and /dev/urandom; bound read-only via -R.
	for _, d := range []string{"/bin", "/usr", "/lib", "/etc", "/dev", "/var"} {
		dirs[d] = struct{}{}
	}

	// addIfNotCovered skips cmd's directory if a parent is already in dirs.
	addIfNotCovered := func(cmd string) {
		if cmd == "" {
			return
		}
		d := filepath.Dir(cmd)
		if d == "/" {
			return
		}
		for existing := range dirs {
			if d == existing || strings.HasPrefix(d, existing+"/") {
				return
			}
		}
		dirs[d] = struct{}{}
	}

	if lang.Build != nil {
		addIfNotCovered(lang.Build.Cmd)
	}
	// For compiled languages, Run.Cmd is the artifact path (e.g. /solution) inside
	// the workspace — not a host binary. Its Dir is "/", which cannot be bind-mounted.
	if !lang.IsCompiled() {
		addIfNotCovered(lang.Run.Cmd)
	}

	// Per-language extra mounts declared in YAML (e.g. /opt/swift for languages
	// installed outside the standard Debian tree). Covered-parent check still applies.
	for _, m := range lang.BindMounts {
		addIfNotCovered(m)
	}

	mounts := make([]string, 0, len(dirs))
	for d := range dirs {
		mounts = append(mounts, d)
	}
	return mounts
}
