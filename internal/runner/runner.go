package runner

import (
	"context"
	"fmt"

	"github.com/thesouldev/goboxd/internal/config"
	"github.com/thesouldev/goboxd/internal/registry"
	"github.com/thesouldev/goboxd/internal/sandbox"
	"github.com/thesouldev/goboxd/internal/stats"
	"github.com/thesouldev/goboxd/internal/validate"
)

// Response is the full response returned by Runner.Submit.
type Response struct {
	Status string      `json:"status"`
	Build  BuildResult `json:"build"`
	Tests  []TestResult `json:"tests"`
}

// Runner manages the bounded concurrency semaphore and dispatches jobs.
type Runner struct {
	sem      chan struct{}
	reg      *registry.Registry
	jobCfg   JobConfig
	counters *stats.Counters
	jailDir  string
}

// New creates a Runner with at most maxConcurrent jobs running simultaneously.
func New(maxConcurrent int, reg *registry.Registry, cfg config.Server, counters *stats.Counters) *Runner {
	sem := make(chan struct{}, maxConcurrent)
	for i := 0; i < maxConcurrent; i++ {
		sem <- struct{}{}
	}
	return &Runner{
		sem:      sem,
		reg:      reg,
		jobCfg: JobConfig{
			NsjailPath:     cfg.NsjailPath,
			MaxOutputBytes: int64(cfg.MaxOutputBytes),
		},
		counters: counters,
		jailDir:  cfg.JailDir,
	}
}

// Submit acquires the semaphore (blocking until a slot is available), runs the
// job, releases the slot, and returns the result. Requests queue rather than
// fail when the limit is reached.
func (r *Runner) Submit(ctx context.Context, req JobRequest) (Response, error) {
	lang := r.reg.Get(req.Language)
	if lang == nil {
		return Response{}, fmt.Errorf("unknown language: %s", req.Language)
	}

	// Count as queued while waiting for a semaphore slot.
	r.counters.IncQueued()

	// Block until a slot is available (fixes bounded concurrency requirement).
	select {
	case <-r.sem:
		// got a slot
	case <-ctx.Done():
		r.counters.DecQueued()
		return Response{}, ctx.Err()
	}

	r.counters.DecQueued()
	r.counters.IncInFlight()
	r.counters.IncTotal()
	defer func() {
		r.sem <- struct{}{} // return slot
		r.counters.DecInFlight()
	}()

	ws, err := sandbox.NewWorkspace(r.jailDir)
	if err != nil {
		r.counters.IncFailed()
		return Response{}, fmt.Errorf("creating workspace: %w", err)
	}
	defer ws.Cleanup() // always runs — fixes hole #7

	job := newJob(req, lang, ws, r.jobCfg)

	buildResult := job.compile(ctx)
	testResults := job.runTests(ctx, buildResult.Status)

	resp := Response{
		Status: TopLevelStatus(buildResult.Status, testResults),
		Build:  buildResult,
		Tests:  testResults,
	}

	// Count job-level internal errors (sandbox failure, disk error, etc.)
	// beyond the workspace-creation failure already counted above.
	if resp.Build.Status == validate.BuildStatusInternalError {
		r.counters.IncFailed()
	} else {
		for _, t := range resp.Tests {
			if t.Status == validate.StatusInternalError {
				r.counters.IncFailed()
				break
			}
		}
	}

	return resp, nil
}
