package sandbox

import (
	"bytes"

	"github.com/thesouldev/goboxd/internal/validate"
)

// ParseRunStatus maps nsjail log output and exit code to a test status string.
// Cgroup v2 OOM kills arrive as SIGKILL; "memory.max" in the log identifies them.
// SIGXCPU fires when rlimit_cpu is hit (set 1s above --time_limit as a fallback)
// and still means time_exceeded, not runtime_error.
func ParseRunStatus(log []byte, exitCode int) string {
	logLow := bytes.ToLower(log)
	switch {
	case bytes.Contains(log, []byte("run time >= time limit")):
		return validate.StatusTimeExceeded
	case bytes.Contains(logLow, []byte("memory limit exceeded")),
		bytes.Contains(logLow, []byte("out of memory")),
		bytes.Contains(logLow, []byte("oom kill")),
		bytes.Contains(logLow, []byte("cgroup memory")):
		return validate.StatusMemoryExceeded
	case bytes.Contains(logLow, []byte("memory.max")) &&
		bytes.Contains(log, []byte("killed by signal")):
		return validate.StatusMemoryExceeded
	case bytes.Contains(log, []byte("SIGXCPU")):
		return validate.StatusTimeExceeded
	case bytes.Contains(log, []byte("killed by signal")):
		return validate.StatusRuntimeError
	case exitCode == 0:
		return validate.StatusAccepted
	default:
		return validate.StatusRuntimeError
	}
}

// ParseBuildStatus maps build-phase exit code and nsjail log to build status.
// [E][ prefix means nsjail itself failed (internal_error); other non-zero exits
// mean the compiler ran and failed (failed).
func ParseBuildStatus(log []byte, exitCode int) string {
	switch {
	case exitCode == 0:
		return validate.BuildStatusOK
	case bytes.Contains(log, []byte("[E][")) || (exitCode != 0 && len(log) == 0):
		return validate.BuildStatusInternalError
	default:
		return validate.BuildStatusFailed
	}
}
