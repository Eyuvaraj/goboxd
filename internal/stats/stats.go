package stats

import (
	"sync/atomic"
	"time"
)

type Counters struct {
	jobsTotal          atomic.Int64
	jobsFailedInternal atomic.Int64
	inFlight           atomic.Int64
	queueSize          atomic.Int64 // goroutines blocked waiting for a semaphore slot
	lastInternalErrAt  atomic.Int64 // unix nano; 0 means never
}

func (c *Counters) IncTotal() { c.jobsTotal.Add(1) }
func (c *Counters) IncFailed() {
	c.jobsFailedInternal.Add(1)
	c.lastInternalErrAt.Store(time.Now().UnixNano())
}
func (c *Counters) IncInFlight()      { c.inFlight.Add(1) }
func (c *Counters) DecInFlight()      { c.inFlight.Add(-1) }
func (c *Counters) IncQueued() int64  { return c.queueSize.Add(1) } // returns new depth
func (c *Counters) DecQueued()        { c.queueSize.Add(-1) }
func (c *Counters) JobsTotal() int64  { return c.jobsTotal.Load() }
func (c *Counters) JobsFailed() int64 { return c.jobsFailedInternal.Load() }
func (c *Counters) InFlight() int64   { return c.inFlight.Load() }
func (c *Counters) QueueSize() int64  { return c.queueSize.Load() }

func (c *Counters) LastInternalErrorAt() time.Time {
	ns := c.lastInternalErrAt.Load()
	if ns == 0 {
		return time.Time{}
	}
	return time.Unix(0, ns)
}
