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
	Language         string
	Source           string
	SourceFilename   string
	ArtifactFilename string
	BuildFlags       []string
	RunFlags         []string
	BuildLimits      config.LimitsDef
	RunLimits        config.LimitsDef
	Tests            []TestCase
}

// TestCase is one stdin/expected_stdout pair.
type TestCase struct {
	Stdin          string
	ExpectedStdout string
}

// Job orchestrates one code execution: build (optional) + all tests.
type Job struct {
	req  JobRequest
	lang *config.LanguageDef
	ws   *sandbox.Workspace
	cfg  JobConfig
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
	}

	result, err := sandbox.Run(ctx, rcfg)
	if err != nil {
		return BuildResult{Status: validate.BuildStatusInternalError, Stderr: fmt.Sprintf("sandbox error: %v", err)}
	}

	status := sandbox.ParseBuildStatus(result.Log, result.ExitCode)
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
		}

		result, runErr := sandbox.Run(ctx, rcfg)
		_ = f.Close()

		if runErr != nil {
			results[i] = TestResult{Status: validate.StatusInternalError}
			continue
		}

		status := compareOutput(result, tc.ExpectedStdout)
		// Append the nsjail log to stderr on non-accepted outcomes so callers
		// can see exactly what nsjail reported (mount errors, exec failures, etc.).
		stderr := string(result.Stderr)
		if status != validate.StatusAccepted && len(result.Log) > 0 {
			stderr += "\n[nsjail]\n" + string(result.Log)
		}
		results[i] = TestResult{
			Status:     status,
			Stdout:     string(result.Stdout),
			Stderr:     stderr,
			DurationMs: result.DurationMs,
		}
	}
	return results
}

// compareOutput determines the test status by comparing actual to expected stdout.
// Sandbox-level errors (TLE, OOM, signal) take precedence over output comparison.
func compareOutput(result sandbox.RunResult, expected string) string {
	// Surface sandbox failures first: a TLE/OOM/signal masks whether output matched.
	sandboxStatus := sandbox.ParseRunStatus(result.Log, result.ExitCode)
	if sandboxStatus != validate.StatusAccepted {
		return sandboxStatus
	}

	actual := result.Stdout
	exp := []byte(expected)
	if bytes.Equal(actual, exp) {
		return validate.StatusAccepted
	}
	if bytes.Equal(bytes.TrimSpace(actual), bytes.TrimSpace(exp)) {
		return validate.StatusWhitespaceMismatch
	}
	return validate.StatusWrongOutput
}

// buildBindMounts derives the read-only bind mounts needed for a language.
// --rw in buildArgv remounts the chroot root as writable so compilers can
// write artifacts into the workspace; the bind mounts themselves stay -R.
//
// Overlap rule: never include a path whose parent is already in the set.
// nsjail bind-mounts each path individually and then remounts it read-only.
// If both /usr/bin and /usr are passed, /usr is mounted on top of the
// /usr/bin bind mount inside the jail, and the subsequent MS_RDONLY remount
// of /usr/bin fails with EINVAL because the kernel sees a different mount ID.
func buildBindMounts(lang *config.LanguageDef) []string {
	dirs := map[string]struct{}{}

	// Broad mounts cover all standard toolchain locations.
	//   /usr  — binaries + shared libraries (includes /usr/lib, /usr/bin, etc.)
	//   /etc  — ld.so.cache, nsswitch.conf, passwd (required by many runtimes)
	//   /dev  — /dev/null, /dev/urandom, /dev/tty (Python/Java read these at start)
	//   /var  — some runtimes write lock/log files here
	for _, d := range []string{"/bin", "/usr", "/lib", "/etc", "/dev", "/var"} {
		dirs[d] = struct{}{}
	}

	// addIfNotCovered adds d only when no existing entry is a parent of d.
	// This prevents redundant sub-mounts that trigger the EINVAL remount bug.
	addIfNotCovered := func(cmd string) {
		if cmd == "" {
			return
		}
		d := filepath.Dir(cmd)
		for existing := range dirs {
			if d == existing || strings.HasPrefix(d, existing+"/") {
				return // already covered by a parent mount
			}
		}
		dirs[d] = struct{}{}
	}

	if lang.Build != nil {
		addIfNotCovered(lang.Build.Cmd)
	}
	addIfNotCovered(lang.Run.Cmd)

	mounts := make([]string, 0, len(dirs))
	for d := range dirs {
		mounts = append(mounts, d)
	}
	return mounts
}
