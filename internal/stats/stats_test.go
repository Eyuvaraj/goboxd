package stats_test

import (
	"os"
	"testing"
	"time"

	"github.com/thesouldev/goboxd/internal/stats"
)

func TestCounters_JobsTotal(t *testing.T) {
	c := &stats.Counters{}
	if c.JobsTotal() != 0 {
		t.Fatalf("initial JobsTotal: want 0, got %d", c.JobsTotal())
	}
	c.IncTotal()
	c.IncTotal()
	if c.JobsTotal() != 2 {
		t.Fatalf("after 2 IncTotal: want 2, got %d", c.JobsTotal())
	}
}

func TestCounters_JobsFailed(t *testing.T) {
	c := &stats.Counters{}
	if c.JobsFailed() != 0 {
		t.Fatalf("initial JobsFailed: want 0, got %d", c.JobsFailed())
	}
	before := time.Now()
	c.IncFailed()
	after := time.Now()

	if c.JobsFailed() != 1 {
		t.Fatalf("after IncFailed: want 1, got %d", c.JobsFailed())
	}
	ts := c.LastInternalErrorAt()
	if ts.IsZero() {
		t.Error("LastInternalErrorAt should be non-zero after IncFailed")
	}
	if ts.Before(before) || ts.After(after) {
		t.Errorf("LastInternalErrorAt %v not between %v and %v", ts, before, after)
	}
}

func TestCounters_LastInternalErrorAt_ZeroInitially(t *testing.T) {
	c := &stats.Counters{}
	if !c.LastInternalErrorAt().IsZero() {
		t.Error("LastInternalErrorAt should be zero time before any failure")
	}
}


func TestDiskFreeBytes_InvalidPath(t *testing.T) {
	free := stats.DiskFreeBytes(os.DevNull + "/nonexistent")
	if free != -1 {
		t.Errorf("DiskFreeBytes(invalid): want -1, got %d", free)
	}
}
