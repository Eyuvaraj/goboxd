package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Workspace is a per-job temporary directory created atomically.
// Cleanup must be called on every exit path (use defer).
type Workspace struct {
	Dir string
}

// NewWorkspace creates a unique temporary directory under jailDir.
// os.MkdirTemp guarantees atomicity — no UID scheme, no collision possible.
func NewWorkspace(jailDir string) (*Workspace, error) {
	if err := os.MkdirAll(jailDir, 0o750); err != nil {
		return nil, fmt.Errorf("creating jail dir %s: %w", jailDir, err)
	}
	dir, err := os.MkdirTemp(jailDir, "goboxd-*")
	if err != nil {
		return nil, fmt.Errorf("creating workspace: %w", err)
	}
	// nsjail mounts a procfs at /proc inside the chroot when creating a PID
	// namespace; the mount point must exist before nsjail starts.
	if err := os.Mkdir(filepath.Join(dir, "proc"), 0o555); err != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("creating proc mount point: %w", err)
	}
	// /lib64 is a symlink on modern Linux (e.g. → usr/lib/x86_64-linux-gnu).
	// nsjail cannot bind-mount symlinks directly (it detects them as dir:false
	// and fails to create the mount point). Recreate the same symlink inside
	// the workspace so /lib64/ld-linux-x86-64.so.2 resolves through /lib.
	if target, err := os.Readlink("/lib64"); err == nil {
		_ = os.Symlink(target, filepath.Join(dir, "lib64"))
	}
	return &Workspace{Dir: dir}, nil
}

// Cleanup removes the workspace directory tree. Safe to call multiple times.
func (ws *Workspace) Cleanup() {
	// Pure Go — no shell involved (fixes hole #2).
	_ = os.RemoveAll(ws.Dir)
}

// TestDir returns the path for test i's stdin/stdout directory,
// creating it if it does not exist.
func (ws *Workspace) TestDir(i int) (string, error) {
	dir := filepath.Join(ws.Dir, fmt.Sprintf("test_%d", i))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	return dir, nil
}

// SourcePath returns the path where source code should be written.
func (ws *Workspace) SourcePath(filename string) string {
	return filepath.Join(ws.Dir, filename)
}

// SweepOrphans removes workspace directories under jailDir that are older
// than maxAge. Called once at startup to clean up after unclean shutdowns.
func SweepOrphans(jailDir string, maxAge time.Duration) {
	entries, err := os.ReadDir(jailDir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-maxAge)
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "goboxd-") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.RemoveAll(filepath.Join(jailDir, e.Name()))
		}
	}
}
