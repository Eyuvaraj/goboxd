//go:build !linux

package stats

// DiskFreeBytes is a stub for non-Linux platforms; the real implementation
// (diskfree_linux.go) uses syscall.Statfs. Lets the package build on macOS.
func DiskFreeBytes(_ string) int64 { return -1 }
