package stats

import (
	"sync/atomic"
	"time"
)

// Counters holds lifetime job tallies. The live in-flight and queue-depth
// gauges are owned by the admission controller (internal/admit), which is the
// single authority on admission state; see handler.LiveStats.
type Counters struct {
	jobsTotal          atomic.Int64
	jobsFailedInternal atomic.Int64
	lastInternalErrAt  atomic.Int64 // unix nano; 0 means never
}

func (c *Counters) IncTotal() { c.jobsTotal.Add(1) }
func (c *Counters) IncFailed() {
	c.jobsFailedInternal.Add(1)
	c.lastInternalErrAt.Store(time.Now().UnixNano())
}
func (c *Counters) JobsTotal() int64  { return c.jobsTotal.Load() }
func (c *Counters) JobsFailed() int64 { return c.jobsFailedInternal.Load() }

func (c *Counters) LastInternalErrorAt() time.Time {
	ns := c.lastInternalErrAt.Load()
	if ns == 0 {
		return time.Time{}
	}
	return time.Unix(0, ns)
}
