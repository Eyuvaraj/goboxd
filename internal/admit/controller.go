// Package admit gates concurrency with a single-owner, two-resource scheduler.
//
// One goroutine (loop) owns the free CPU/memory budget and a FIFO waiter list;
// callers reserve over channels rather than sharing the state. That makes it
// deadlock-free (one owner), starvation-free (strict FIFO), and able to admit
// against CPU and memory at once. Rationale: docs/concurrency.md and ADR-14.
package admit

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

// ErrOverloaded is returned by Acquire when the queue is at MaxQueueDepth; the
// caller maps it to HTTP 503.
var ErrOverloaded = errors.New("admission queue full")

// errCancelled signals that a queued ticket was dropped because its caller's
// context was cancelled. It never escapes the package.
var errCancelled = errors.New("admit: cancelled")

// Resources is a reservation: CPU permits and memory in KiB.
type Resources struct {
	CPU   int
	MemKB int64
}

// Config sets the total budget and queue bound.
type Config struct {
	CPU           int   // total CPU permits
	MemKB         int64 // total memory budget; <= 0 disables the memory dimension
	MaxQueueDepth int   // waiters allowed before shedding; 0 = unbounded
}

type ticket struct {
	need   Resources
	result chan error // buffered(1); loop sends once: nil=grant, else reject/cancel
}

// Controller is the scheduler. Construct with New; the owner goroutine runs
// until Close.
type Controller struct {
	admitCh  chan *ticket
	cancelCh chan *ticket
	relCh    chan Resources
	closeCh  chan struct{}

	total    Resources
	maxQueue int

	queued   atomic.Int64 // waiters; gauge for /info
	inFlight atomic.Int64 // granted, not yet released
}

// New starts a Controller and launches its owner goroutine.
func New(cfg Config) *Controller {
	c := &Controller{
		admitCh:  make(chan *ticket),
		cancelCh: make(chan *ticket),
		relCh:    make(chan Resources),
		closeCh:  make(chan struct{}),
		total:    Resources{CPU: cfg.CPU, MemKB: cfg.MemKB},
		maxQueue: cfg.MaxQueueDepth,
	}
	go c.loop()
	return c
}

// Close stops the owner goroutine; still-queued waiters get ErrOverloaded.
func (c *Controller) Close() { close(c.closeCh) }

// Queued reports callers currently blocked in the wait queue.
func (c *Controller) Queued() int64 { return c.queued.Load() }

// InFlight reports granted reservations not yet released.
func (c *Controller) InFlight() int64 { return c.inFlight.Load() }

// Acquire reserves need, blocking in FIFO order until it fits. On success it
// returns a release func to call exactly once (extra calls are no-ops). Returns
// ErrOverloaded if the queue is full, or ctx.Err() if ctx is cancelled first.
func (c *Controller) Acquire(ctx context.Context, need Resources) (func(), error) {
	if need.CPU < 1 {
		need.CPU = 1
	}
	// Clamp an oversized job to the whole budget so it can still run (alone)
	// rather than wait forever against a smaller ceiling.
	if c.total.MemKB > 0 && need.MemKB > c.total.MemKB {
		need.MemKB = c.total.MemKB
	}

	t := &ticket{need: need, result: make(chan error, 1)}

	select {
	case c.admitCh <- t:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.closeCh:
		return nil, ErrOverloaded
	}

	select {
	case err := <-t.result:
		if err != nil {
			return nil, err
		}
		return c.releaseFunc(need), nil
	case <-ctx.Done():
		// Ask the loop to cancel; it answers authoritatively. A grant may have
		// raced ahead, in which case we refund the slot.
		select {
		case c.cancelCh <- t:
		case <-c.closeCh:
		}
		if err := <-t.result; err == nil {
			c.releaseFunc(need)()
		}
		return nil, ctx.Err()
	}
}

func (c *Controller) releaseFunc(need Resources) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			select {
			case c.relCh <- need:
			case <-c.closeCh:
			}
		})
	}
}

// loop is the sole owner of the free budget and waiter list.
func (c *Controller) loop() {
	free := c.total
	var waiters []*ticket

	// Grant the head while resources permit. Strict FIFO: a large head blocks
	// smaller jobs behind it, so cheap jobs can't jump an expensive one.
	drain := func() {
		for len(waiters) > 0 {
			head := waiters[0]
			if head.need.CPU > free.CPU {
				return
			}
			if c.total.MemKB > 0 && head.need.MemKB > free.MemKB {
				return
			}
			waiters = waiters[1:]
			c.queued.Add(-1)
			free.CPU -= head.need.CPU
			if c.total.MemKB > 0 {
				free.MemKB -= head.need.MemKB
			}
			c.inFlight.Add(1)
			head.result <- nil
		}
	}

	for {
		select {
		case <-c.closeCh:
			for _, w := range waiters {
				w.result <- ErrOverloaded
			}
			return

		case t := <-c.admitCh:
			if c.maxQueue > 0 && len(waiters) >= c.maxQueue {
				t.result <- ErrOverloaded
				continue
			}
			waiters = append(waiters, t)
			c.queued.Add(1)
			drain()

		case t := <-c.cancelCh:
			for i, w := range waiters {
				if w == t {
					waiters = append(waiters[:i], waiters[i+1:]...)
					c.queued.Add(-1)
					t.result <- errCancelled
					break
				}
			}
			// not found => already granted; its release refunds the budget

		case r := <-c.relCh:
			free.CPU += r.CPU
			if c.total.MemKB > 0 {
				free.MemKB += r.MemKB
			}
			c.inFlight.Add(-1)
			drain()
		}
	}
}
