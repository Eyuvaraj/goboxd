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

type Response struct {
	Status string       `json:"status"`
	Build  BuildResult  `json:"build"`
	Tests  []TestResult `json:"tests"`
}

type Runner struct {
	sem      chan struct{}
	reg      *registry.Registry
	jobCfg   JobConfig
	counters *stats.Counters
	jailDir  string
}

func New(maxConcurrent int, reg *registry.Registry, cfg config.Server, counters *stats.Counters) *Runner {
	sem := make(chan struct{}, maxConcurrent)
	for range maxConcurrent {
		sem <- struct{}{}
	}
	return &Runner{
		sem: sem,
		reg: reg,
		jobCfg: JobConfig{
			NsjailPath:     cfg.NsjailPath,
			MaxOutputBytes: int64(cfg.MaxOutputBytes),
		},
		counters: counters,
		jailDir:  cfg.JailDir,
	}
}

// Submit blocks until a concurrency slot is free, runs the job, then returns.
func (r *Runner) Submit(ctx context.Context, req JobRequest) (Response, error) {
	lang := r.reg.Get(req.Language)
	if lang == nil {
		return Response{}, fmt.Errorf("unknown language: %s", req.Language)
	}

	r.counters.IncQueued()
	select {
	case <-r.sem:
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
	defer ws.Cleanup()

	job := newJob(req, lang, ws, r.jobCfg)

	buildResult := job.compile(ctx)
	testResults := job.runTests(ctx, buildResult.Status)

	resp := Response{
		Status: TopLevelStatus(buildResult.Status, testResults),
		Build:  buildResult,
		Tests:  testResults,
	}

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
