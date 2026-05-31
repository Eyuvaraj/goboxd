package sandbox_test

import (
	"testing"

	"github.com/thesouldev/goboxd/internal/sandbox"
	"github.com/thesouldev/goboxd/internal/validate"
)

func TestParseRunStatus(t *testing.T) {
	tests := []struct {
		name      string
		stderr    string
		exitCode  int
		oomKilled bool
		want      string
	}{
		{"time exceeded", "run time >= time limit", 1, false, validate.StatusTimeExceeded},
		// A cgroup v2 OOM reaches us only as SIGKILL; memory.events oom_kill is the
		// authoritative signal, so oomKilled wins over the bare "killed by signal".
		{"cgroupv2 oom", "killed by signal 9", 9, true, validate.StatusMemoryExceeded},
		{"sigxcpu", "killed by signal 24 (SIGXCPU)", 24, false, validate.StatusTimeExceeded},
		{"signal segfault", "killed by signal 11", 139, false, validate.StatusRuntimeError},
		// SIGKILL with no oom_kill recorded is an ordinary kill, not memory_exceeded.
		{"signal 9 no oom", "killed by signal 9", 9, false, validate.StatusRuntimeError},
		{"exit zero", "exited with status: 0", 0, false, validate.StatusAccepted},
		{"exit nonzero", "exited with status: 1", 1, false, validate.StatusRuntimeError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sandbox.ParseRunStatus([]byte(tc.stderr), tc.exitCode, tc.oomKilled)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseBuildStatus(t *testing.T) {
	// Success path.
	if s := sandbox.ParseBuildStatus([]byte(""), 0); s != validate.BuildStatusOK {
		t.Errorf("exit 0: got %q", s)
	}

	// Compiler ran and failed — nsjail info lines contain "nsjail" but no "[E][".
	// This was the regression: the old check matched "nsjail" → internal_error.
	realNsjailInfoLog := []byte("[I][2026-05-23T12:00:00+0000] nsjail[1234]: PID 5678 terminated, exit code=1\n")
	if s := sandbox.ParseBuildStatus(realNsjailInfoLog, 1); s != validate.BuildStatusFailed {
		t.Errorf("compiler error with real nsjail log: got %q, want %q", s, validate.BuildStatusFailed)
	}

	// nsjail itself failed — log contains "[E][" error-level line.
	if s := sandbox.ParseBuildStatus([]byte("[E][nsjail.cc:123] execveat failed"), 255); s != validate.BuildStatusInternalError {
		t.Errorf("nsjail exec error: got %q", s)
	}

	// Plain compiler stderr (no "[E][") with non-zero exit.
	if s := sandbox.ParseBuildStatus([]byte("error: syntax error"), 1); s != validate.BuildStatusFailed {
		t.Errorf("plain compiler error: got %q", s)
	}

	// Non-zero exit with an empty log means nsjail never produced diagnostics —
	// it never ran the build, so this is infrastructure failure, not a compile error.
	if s := sandbox.ParseBuildStatus([]byte(""), 1); s != validate.BuildStatusInternalError {
		t.Errorf("non-zero exit with empty log: got %q, want %q", s, validate.BuildStatusInternalError)
	}
}
