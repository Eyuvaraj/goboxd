package sandbox_test

import (
	"testing"

	"github.com/thesouldev/goboxd/internal/sandbox"
	"github.com/thesouldev/goboxd/internal/validate"
)

func TestParseRunStatus(t *testing.T) {
	tests := []struct {
		name     string
		stderr   string
		exitCode int
		want     string
	}{
		{"time exceeded", "run time >= time limit", 1, validate.StatusTimeExceeded},
		{"memory exceeded cgroup v1", "memory limit exceeded", 1, validate.StatusMemoryExceeded},
		{"oom", "out of memory", 137, validate.StatusMemoryExceeded},
		{"signal segfault", "killed by signal 11", 139, validate.StatusRuntimeError},
		// Cgroup v2: nsjail logs "Setting 'memory.max' to '<n>'" then SIGKILL.
		{"cgroupv2 oom", "[I] Setting 'memory.max' to '104857600'\n[I] killed by signal 9", 9, validate.StatusMemoryExceeded},
		{"signal 9 no cgroup", "killed by signal 9", 9, validate.StatusRuntimeError},
		{"exit zero", "exited with status: 0", 0, validate.StatusAccepted},
		{"exit nonzero", "exited with status: 1", 1, validate.StatusRuntimeError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sandbox.ParseRunStatus([]byte(tc.stderr), tc.exitCode)
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
}
