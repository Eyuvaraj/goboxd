package stats

import (
	"sync/atomic"
	"syscall"
	"time"
)

// Counters tracks global job statistics for the /info endpoint.
type Counters struct {
	jobsTotal          atomic.Int64
	jobsFailedInternal atomic.Int64
	inFlight           atomic.Int64
	lastInternalErrAt  atomic.Int64 // unix nano; 0 means never
}

func (c *Counters) IncTotal()            { c.jobsTotal.Add(1) }
func (c *Counters) IncFailed()           { c.jobsFailedInternal.Add(1); c.lastInternalErrAt.Store(time.Now().UnixNano()) }
func (c *Counters) IncInFlight()         { c.inFlight.Add(1) }
func (c *Counters) DecInFlight()         { c.inFlight.Add(-1) }
func (c *Counters) JobsTotal() int64     { return c.jobsTotal.Load() }
func (c *Counters) JobsFailed() int64    { return c.jobsFailedInternal.Load() }
func (c *Counters) InFlight() int64      { return c.inFlight.Load() }

// LastInternalErrorAt returns the timestamp of the last internal error, or zero time.
func (c *Counters) LastInternalErrorAt() time.Time {
	ns := c.lastInternalErrAt.Load()
	if ns == 0 {
		return time.Time{}
	}
	return time.Unix(0, ns)
}

// DiskFreeBytes returns the available bytes on the filesystem containing path.
// Returns -1 on error.
func DiskFreeBytes(path string) int64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return -1
	}
	return int64(stat.Bavail) * int64(stat.Bsize)
}
