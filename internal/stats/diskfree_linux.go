//go:build linux

package stats

import "syscall"

// DiskFreeBytes returns available bytes on the filesystem at path, or -1 on error.
func DiskFreeBytes(path string) int64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return -1
	}
	return int64(stat.Bavail) * int64(stat.Bsize)
}
