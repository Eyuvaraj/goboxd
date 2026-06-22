package runner

import (
	"context"
	"errors"
	"fmt"

	"github.com/thesouldev/goboxd/internal/admit"
	"github.com/thesouldev/goboxd/internal/config"
	"github.com/thesouldev/goboxd/internal/registry"
	"github.com/thesouldev/goboxd/internal/sandbox"
	"github.com/thesouldev/goboxd/internal/stats"
	"github.com/thesouldev/goboxd/internal/validate"
)

// ErrOverloaded is returned by Submit when the admission queue is already at
// MaxQueueDepth. The handler maps it to 503 + Retry-After. Disabled when
// MaxQueueDepth is 0.
var ErrOverloaded = errors.New("server overloaded: queue full")

type Response struct {
	Status string       `json:"status"`
	Build  BuildResult  `json:"build"`
	Tests  []TestResult `json:"tests"`
}

type Runner struct {
	ctrl     *admit.Controller
	reg      *registry.Registry
	jobCfg   JobConfig
	counters *stats.Counters
	jailDir  string
}

func New(maxConcurrent int, reg *registry.Registry, cfg config.Server, counters *stats.Counters) *Runner {
	ctrl := admit.New(admit.Config{
		CPU:           maxConcurrent,
		MemKB:         cfg.MemBudgetKB,
		MaxQueueDepth: cfg.MaxQueueDepth,
	})
	return &Runner{
		ctrl: ctrl,
		reg:  reg,
		jobCfg: JobConfig{
			NsjailPath:     cfg.NsjailPath,
			MaxOutputBytes: int64(cfg.MaxOutputBytes),
			JailDir:        cfg.JailDir,
		},
		counters: counters,
		jailDir:  cfg.JailDir,
	}
}

// Close stops the admission controller. Call after the HTTP server has drained
// in-flight requests.
func (r *Runner) Close() { r.ctrl.Close() }

// InFlight reports jobs currently holding a slot; QueueSize reports callers
// blocked waiting for one. Both satisfy handler.LiveStats for /info.
func (r *Runner) InFlight() int64  { return r.ctrl.InFlight() }
func (r *Runner) QueueSize() int64 { return r.ctrl.Queued() }

// Submit blocks until a concurrency slot is free, runs the job, then returns.
func (r *Runner) Submit(ctx context.Context, req JobRequest) (Response, error) {
	lang := r.reg.Get(req.Language)
	if lang == nil {
		return Response{}, fmt.Errorf("unknown language: %s", req.Language)
	}

	var evalLang *config.LanguageDef
	if req.Evaluator != nil {
		if evalLang = r.reg.Get(req.Evaluator.Language); evalLang == nil {
			return Response{}, fmt.Errorf("unknown evaluator language: %s", req.Evaluator.Language)
		}
	}

	// Reserve one CPU permit plus the job's peak memory; the controller blocks
	// until both fit, or sheds with ErrOverloaded when the queue is full.
	reservation := admit.Resources{CPU: 1, MemKB: reservationKB(lang, evalLang, req)}
	release, err := r.ctrl.Acquire(ctx, reservation)
	if err != nil {
		if errors.Is(err, admit.ErrOverloaded) {
			return Response{}, ErrOverloaded
		}
		return Response{}, err
	}
	defer release()
	r.counters.IncTotal()

	ws, err := sandbox.NewWorkspace(r.jailDir)
	if err != nil {
		r.counters.IncFailed()
		return Response{}, fmt.Errorf("creating workspace: %w", err)
	}
	defer ws.Cleanup()

	job := newJob(req, lang, evalLang, ws, r.jobCfg)

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

// reservationKB is a job's peak memory: the max across its phases, not the sum,
// because build, run, and evaluator steps execute sequentially. 0 if no phase
// sets a limit.
func reservationKB(lang, evalLang *config.LanguageDef, req JobRequest) int64 {
	peak := sandbox.MergeLimits(lang.Run.Limits, req.RunLimits).MemoryKB
	if lang.Build != nil {
		if b := sandbox.MergeLimits(lang.Build.Limits, req.BuildLimits).MemoryKB; b > peak {
			peak = b
		}
	}
	if req.Evaluator != nil && evalLang != nil {
		if er := sandbox.MergeLimits(evalLang.Run.Limits, req.Evaluator.RunLimits).MemoryKB; er > peak {
			peak = er
		}
		if evalLang.Build != nil {
			if eb := sandbox.MergeLimits(evalLang.Build.Limits, req.Evaluator.BuildLimits).MemoryKB; eb > peak {
				peak = eb
			}
		}
	}
	return int64(peak)
}
