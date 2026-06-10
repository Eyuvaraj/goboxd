# Concurrency and Load Behaviour

How goboxd bounds concurrent work, queues fairly, sheds load when asked, and why the design is a fixed semaphore rather than something cleverer. This is the authoritative treatment; [architecture.md](architecture.md) carries a short summary that links here.

---

## The Mechanism: a Counting Semaphore

Concurrency is gated by a single buffered channel used as a counting semaphore (`internal/runner/runner.go`):

```go
sem := make(chan struct{}, maxConcurrent)
for range maxConcurrent { sem <- struct{}{} }   // pre-fill: N tokens available
```

- **Acquire** by receiving (`<-sem`); **release** by sending (`sem <- struct{}{}`) in a `defer`. Each in-flight job holds exactly one token.
- **Capacity** is `MAX_CONCURRENT_JOBS`, defaulting to `config.AvailableCPUs()`: the cgroup v2 CPU quota when the container is CPU-limited, otherwise `runtime.NumCPU()`. The same value is set as `GOMAXPROCS`, so a quota-limited container is never oversubscribed at either the Go-scheduler or the job level.
- **Each request drives its own job in its own goroutine.** There is no worker pool. The request goroutine acquires a token, runs the job synchronously, and releases.

When all tokens are out, further `<-sem` receives block, parking the goroutine on Go's runtime channel waiter queue. That queue is FIFO, which gives fair ordering with no starvation.

---

## The Queue and Optional Load Shedding

By default the wait queue is **unbounded**: under a burst, requests queue and complete in arrival order rather than failing. The queue depth is tracked with an atomic counter (exposed at `/info` as `queue_size`), not by inspecting channel length.

When `MAX_QUEUE_DEPTH > 0`, goboxd **sheds load**: a request whose arrival would push the wait queue past that depth is rejected immediately with `503` and `Retry-After: 1` instead of queueing. The reservation is a single atomic increment-then-check, so a concurrent burst cannot slip past the cap:

```go
depth := counters.IncQueued()
if maxQueueDepth > 0 && depth > maxQueueDepth {
    counters.DecQueued()
    return ErrOverloaded        // handler returns 503 + Retry-After
}
select {
case <-sem:                     // got a slot
case <-ctx.Done():              // client disconnected while waiting
    counters.DecQueued()
    return ctx.Err()            // handler returns 503
}
```

Client disconnects are handled too: a cancelled request context unblocks the `select` and returns without ever starting a sandbox.

---

## Why a Fixed Semaphore, Not a Cleverer Scheduler

A submission's wall-time splits in two: nsjail setup/teardown (namespace and cgroup creation, bind mounts: latency, not compute) and the program's own execution.

- **CPU-bound submissions** (compiled languages, compute-heavy programs): the throughput-optimal in-flight count is the core count (the semaphore's default), and no scheduler can beat it, because the CPUs cannot do more work per second.
- **Setup-bound submissions** (trivial or I/O-bound programs): raising the limit above the core count overlaps setup latency and does increase throughput. Measured: doubling the limit on a hello-world workload roughly doubled req/s while CPU stayed half-idle.

`MAX_CONCURRENT_JOBS` is therefore an operator-tunable knob, defaulting to the core count because that is the safe optimum for CPU-bound work and because oversubscription multiplies peak memory (each in-flight job can reach its per-language memory cap).

**Specific design choices:**

- **Fixed limit, not adaptive concurrency (AIMD / gradient).** Adaptive limits exist to discover an unknown safe concurrency under shifting load. Here the safe point is known (core count for CPU-bound work) and is operator-overridable per deployment, so adaptation adds tuning machinery and failure modes for little gain.
- **FIFO, not priority or shortest-job-first.** A blocked channel receive already parks on a FIFO waiter queue: fair, starvation-free, zero code. Priority ordering would need a job-cost estimate that cannot be obtained before running the code, and invites starvation of expensive jobs.
- **Semaphore, not a worker pool.** A pool needs persistent goroutines and a dispatch channel to achieve the identical invariant (N or fewer concurrent jobs). Throughput is the same; the semaphore is less code and has no idle goroutines.
- **Sequential tests within a job.** nsjail process startup dominates, not goroutine scheduling. Running a job's tests sequentially in one workspace gives a deterministic file layout, avoids workspace races, and keeps one request from fanning out into N sandboxes and breaking the global concurrency bound.

A persistent or distributed queue is explicitly out of scope; bounding the in-memory queue (`MAX_QUEUE_DEPTH`) is the one backpressure lever worth having.

---

## Timeouts That Bound the Worst Case

- **HTTP `WriteTimeout`** = `reg.MaxJobDuration(MaxTests)` = `maxBuild + MaxTests x maxRun + 30s`: long enough for the largest legitimate request, finite so a rogue client cannot hold a connection forever.
- **`ReadHeaderTimeout` = 5s**: drops slowloris connections that dribble headers.
- **Graceful shutdown**: waits up to `MaxJobDuration + 5s` for in-flight jobs to finish before exiting.
- **Per-job `WaitDelay` = 5s** (`exec.Cmd`): if the context is cancelled or nsjail exits while a descendant still holds a pipe open, the pipes are force-closed so `cmd.Wait()` and the semaphore token it holds can never block indefinitely.

---

## Verification Under Load

- **`make load`** runs `scripts/load_test.sh` (`hey`) at 1/10/50/100 concurrent clients for py3 and cpp, reporting req/s and p50/p95/p99. Reference numbers, measurement caveats, and the two-regime analysis: [benchmarks.md](benchmarks.md).
- **`internal/runner/overload_test.go`** exercises the semaphore and queue-depth shedding under concurrent goroutines with the race detector.
- The judges run a sustained-load pass; the bounded queue and load-shedding give a defined saturation behaviour (latency grows, throughput plateaus, optionally `503`) rather than an open-ended one.

---

## Tuning Guide

| Symptom | Lever |
|---------|-------|
| CPU underutilised on trivial workloads | Raise `MAX_CONCURRENT_JOBS` above the core count (watch peak memory) |
| Host memory pressure under load | Lower `MAX_CONCURRENT_JOBS`, or lower per-language `memory_kb` |
| Want to reject instead of queue under a flood | Set `MAX_QUEUE_DEPTH` to a finite value (clients get `503 + Retry-After`) |
| CPU-limited container oversubscribed | Nothing; the default already reads the cgroup quota |

All knobs above are environment variables. Defaults and full descriptions: [configuration.md](configuration.md)
