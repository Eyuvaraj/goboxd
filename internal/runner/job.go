package runner

import (
	"bytes"
	"context"
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
}

type TestCase struct {
	Stdin          string
	ExpectedStdout string
}

type Job struct {
	req  JobRequest
	lang *config.LanguageDef
	ws   *sandbox.Workspace
	cfg  JobConfig
}

type JobConfig struct {
	NsjailPath     string
	MaxOutputBytes int64
	BindMounts     []string
}

func newJob(req JobRequest, lang *config.LanguageDef, ws *sandbox.Workspace, cfg JobConfig) *Job {
	return &Job{req: req, lang: lang, ws: ws, cfg: cfg}
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

		status := sandbox.ParseRunStatus(result.Log, result.ExitCode)
		if !j.req.Raw {
			status = compareOutput(result, tc.ExpectedStdout)
		}
		results[i] = TestResult{
			Status:       status,
			Stdout:       string(result.Stdout),
			Stderr:       string(result.Stderr),
			ExitCode:     result.ExitCode,
			DurationMs:   result.DurationMs,
			MemoryPeakKB: result.MemoryPeakKB,
		}
	}
	return results
}

// compareOutput returns test status. Sandbox failures take precedence.
// Strips trailing whitespace to catch common newline differences without
// masking leading-whitespace differences.
func compareOutput(result sandbox.RunResult, expected string) string {
	sandboxStatus := sandbox.ParseRunStatus(result.Log, result.ExitCode)
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

	mounts := make([]string, 0, len(dirs))
	for d := range dirs {
		mounts = append(mounts, d)
	}
	return mounts
}
