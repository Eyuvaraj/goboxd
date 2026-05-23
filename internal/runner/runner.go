package runner

import (
	"context"
	"fmt"

	"github.com/thesouldev/goboxd/internal/config"
	"github.com/thesouldev/goboxd/internal/registry"
	"github.com/thesouldev/goboxd/internal/sandbox"
	"github.com/thesouldev/goboxd/internal/stats"
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

	// Block until a slot is available (fixes bounded concurrency requirement).
	select {
	case <-r.sem:
		// got a slot
	case <-ctx.Done():
		return Response{}, ctx.Err()
	}

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

	return Response{
		Status: TopLevelStatus(buildResult.Status, testResults),
		Build:  buildResult,
		Tests:  testResults,
	}, nil
}
