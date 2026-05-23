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
		{"memory exceeded", "memory limit exceeded", 1, validate.StatusMemoryExceeded},
		{"oom", "out of memory", 137, validate.StatusMemoryExceeded},
		{"signal", "killed by signal 11", 139, validate.StatusRuntimeError},
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
	if s := sandbox.ParseBuildStatus([]byte(""), 0); s != validate.BuildStatusOK {
		t.Errorf("exit 0: got %q", s)
	}
	if s := sandbox.ParseBuildStatus([]byte("error: syntax"), 1); s != validate.BuildStatusFailed {
		t.Errorf("compiler error: got %q", s)
	}
	if s := sandbox.ParseBuildStatus([]byte("[E][nsjail] some error"), 255); s != validate.BuildStatusInternalError {
		t.Errorf("nsjail error: got %q", s)
	}
}
