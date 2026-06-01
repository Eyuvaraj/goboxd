//go:build linux

package stats_test

import (
	"testing"

	"github.com/thesouldev/goboxd/internal/stats"
)

func TestDiskFreeBytes_ValidPath(t *testing.T) {
	dir := t.TempDir()
	free := stats.DiskFreeBytes(dir)
	if free <= 0 {
		t.Errorf("DiskFreeBytes(%q): want > 0, got %d", dir, free)
	}
}
