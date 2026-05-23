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

// BuildResult matches the spec's build object.
type BuildResult struct {
	Status     string `json:"status"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	DurationMs int64  `json:"duration_ms"`
}

// TestResult matches the spec's per-test result object.
type TestResult struct {
	Status       string `json:"status"`
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr"`
	DurationMs   int64  `json:"duration_ms"`
	MemoryPeakKB int64  `json:"memory_peak_kb,omitempty"`
}

// JobRequest is the validated, parsed input to a job.
type JobRequest struct {
	Language       string
	Source         string
	SourceFilename string
	ArtifactFilename string
	BuildFlags     []string
	RunFlags       []string
	BuildLimits    config.LimitsDef
	RunLimits      config.LimitsDef
	Tests          []TestCase
}

// TestCase is one stdin/expected_stdout pair.
type TestCase struct {
	Stdin          string
	ExpectedStdout string
}

// Job orchestrates one code execution: build (optional) + all tests.
type Job struct {
	req        JobRequest
	lang       *config.LanguageDef
	ws         *sandbox.Workspace
	cfg        JobConfig
}

// JobConfig holds server-level settings needed by the job.
type JobConfig struct {
	NsjailPath     string
	MaxOutputBytes int64
	// BindMounts are additional read-only bind mounts passed to every nsjail call.
	// Typically the compiler/runtime binaries and system libs.
	BindMounts []string
}

func newJob(req JobRequest, lang *config.LanguageDef, ws *sandbox.Workspace, cfg JobConfig) *Job {
	return &Job{req: req, lang: lang, ws: ws, cfg: cfg}
}

// compile runs the build phase. For interpreted languages this is a no-op.
func (j *Job) compile(ctx context.Context) BuildResult {
	if !j.lang.IsCompiled() {
		return BuildResult{Status: validate.BuildStatusOK}
	}

	// Write source into workspace.
	srcPath := j.ws.SourcePath(j.req.SourceFilename)
	if err := os.WriteFile(srcPath, []byte(j.req.Source), 0o640); err != nil {
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
	}

	result, err := sandbox.Run(ctx, rcfg)
	if err != nil {
		return BuildResult{Status: validate.BuildStatusInternalError, Stderr: fmt.Sprintf("sandbox error: %v", err)}
	}

	status := sandbox.ParseBuildStatus(result.Stderr, result.ExitCode)
	return BuildResult{
		Status:     status,
		Stdout:     string(result.Stdout),
		Stderr:     string(result.Stderr),
		DurationMs: result.DurationMs,
	}
}

// runTests executes each test case. If build failed, returns not_executed for all.
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
		if err := os.WriteFile(srcPath, []byte(j.req.Source), 0o640); err != nil {
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
		if err := os.WriteFile(stdinPath, []byte(tc.Stdin), 0o640); err != nil {
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
		}

		result, runErr := sandbox.Run(ctx, rcfg)
		f.Close()

		if runErr != nil {
			results[i] = TestResult{Status: validate.StatusInternalError}
			continue
		}

		status := compareOutput(result, tc.ExpectedStdout)
		results[i] = TestResult{
			Status:     status,
			Stdout:     string(result.Stdout),
			Stderr:     string(result.Stderr),
			DurationMs: result.DurationMs,
		}
	}
	return results
}

// compareOutput determines the test status by comparing actual to expected stdout.
func compareOutput(result sandbox.RunResult, expected string) string {
	actual := result.Stdout
	exp := []byte(expected)

	if bytes.Equal(actual, exp) {
		return validate.StatusAccepted
	}
	if bytes.Equal(bytes.TrimSpace(actual), bytes.TrimSpace(exp)) {
		return validate.StatusWhitespaceMismatch
	}
	// Not a match — check for sandbox-level errors.
	return sandbox.ParseRunStatus(result.Stderr, result.ExitCode)
}

// buildBindMounts derives the bind mounts needed for a language.
// The compiler/runtime and common system paths are mounted read-only.
func buildBindMounts(lang *config.LanguageDef) []string {
	dirs := map[string]struct{}{}

	add := func(cmd string) {
		if cmd == "" {
			return
		}
		// Mount the binary's directory and likely lib directories.
		dirs[filepath.Dir(cmd)] = struct{}{}
	}

	if lang.Build != nil {
		add(lang.Build.Cmd)
	}
	add(lang.Run.Cmd)

	// Standard system paths needed by most programs.
	for _, d := range []string{"/bin", "/usr/bin", "/lib", "/lib64", "/usr/lib", "/etc/ld.so.cache"} {
		dirs[d] = struct{}{}
	}

	mounts := make([]string, 0, len(dirs))
	for d := range dirs {
		mounts = append(mounts, d)
	}
	return mounts
}

// stripPath returns the final component of a path for use as the run command
// when the cwd is the workspace.
func stripPath(cmd string) string {
	if strings.HasPrefix(cmd, "./") {
		return cmd
	}
	return cmd
}

// silence unused
var _ = stripPath
