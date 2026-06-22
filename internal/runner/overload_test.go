package runner_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/thesouldev/goboxd/internal/config"
	"github.com/thesouldev/goboxd/internal/registry"
	"github.com/thesouldev/goboxd/internal/runner"
	"github.com/thesouldev/goboxd/internal/stats"
)

const overloadLangYAML = `
languages:
  - id: py3
    name: Python 3
    source_filename: solution.py
    run:
      cmd: /usr/bin/python3
      args: ["{{source}}"]
      limits:
        wall_time_s: 5
        memory_kb: 102400
        max_processes: 64
`

func testRegistry(t *testing.T) *registry.Registry {
	t.Helper()
	path := filepath.Join(t.TempDir(), "langs.yaml")
	if err := os.WriteFile(path, []byte(overloadLangYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	reg, err := registry.Load(path)
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	return reg
}

// A request that arrives when the queue is already full is shed with
// ErrOverloaded before any sandbox work, so this needs no nsjail and is
// deterministic on any platform. CPU budget 0 means nothing can ever run, so
// the first submission parks as the one allowed waiter and the second is shed.
func TestSubmit_ShedsWhenQueueFull(t *testing.T) {
	counters := &stats.Counters{}
	r := runner.New(0, testRegistry(t), config.Server{MaxQueueDepth: 1}, counters)
	defer r.Close()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() { _, _ = r.Submit(ctx, runner.JobRequest{Language: "py3", Source: "x"}) }()

	// Wait until the first submission is registered as the sole waiter — no sleep.
	for r.QueueSize() < 1 {
		runtime.Gosched()
	}

	_, err := r.Submit(context.Background(), runner.JobRequest{Language: "py3", Source: "x"})
	if !errors.Is(err, runner.ErrOverloaded) {
		t.Fatalf("want ErrOverloaded, got %v", err)
	}
}

// With the default (MaxQueueDepth == 0) the queue is unbounded and load is never
// shed. CPU budget 0 keeps the slot unavailable, so the request would wait on
// the queue; the cancelled context releases it with the context error, never
// ErrOverloaded.
func TestSubmit_UnboundedByDefault(t *testing.T) {
	counters := &stats.Counters{}
	r := runner.New(0, testRegistry(t), config.Server{MaxQueueDepth: 0}, counters)
	defer r.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.Submit(ctx, runner.JobRequest{Language: "py3", Source: "x"})
	if errors.Is(err, runner.ErrOverloaded) {
		t.Fatal("default config must not shed load")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}
