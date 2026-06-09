package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Workspace is a per-job temporary directory. Always call Cleanup via defer.
type Workspace struct {
	Dir string
}

func NewWorkspace(jailDir string) (*Workspace, error) {
	if err := os.MkdirAll(jailDir, 0o750); err != nil {
		return nil, fmt.Errorf("creating jail dir %s: %w", jailDir, err)
	}
	dir, err := os.MkdirTemp(jailDir, "goboxd-*")
	if err != nil {
		return nil, fmt.Errorf("creating workspace: %w", err)
	}
	// nsjail mounts procfs at /proc inside the chroot; the mount point must exist first.
	if err := os.Mkdir(filepath.Join(dir, "proc"), 0o555); err != nil { //nolint:gosec // proc mount point must be world-readable inside the chroot
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("creating proc mount point: %w", err)
	}
	// /lib64 is a symlink on modern Linux. nsjail can't bind-mount symlinks directly,
	// so recreate the same symlink inside the workspace.
	if target, err := os.Readlink("/lib64"); err == nil {
		_ = os.Symlink(target, filepath.Join(dir, "lib64"))
	}
	return &Workspace{Dir: dir}, nil
}

func (ws *Workspace) Cleanup() {
	_ = os.RemoveAll(ws.Dir)
}

func (ws *Workspace) TestDir(i int) (string, error) {
	dir := filepath.Join(ws.Dir, fmt.Sprintf("test_%d", i))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	return dir, nil
}

func (ws *Workspace) SourcePath(filename string) string {
	return filepath.Join(ws.Dir, filename)
}

// SweepOrphans removes goboxd-* workspace directories older than maxAge.
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
