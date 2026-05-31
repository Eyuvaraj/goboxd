package sandbox

import (
	"bytes"

	"github.com/thesouldev/goboxd/internal/validate"
)

// ParseRunStatus maps nsjail log output, exit code, and the cgroup OOM signal to
// a test status string. A cgroup v2 OOM kill reaches us only as a SIGKILL with no
// distinguishing log line, so oomKilled (read from memory.events) is the only
// reliable way to report memory_exceeded rather than runtime_error. SIGXCPU fires
// when rlimit_cpu is hit (set 1s above --time_limit as a fallback) and still means
// time_exceeded.
func ParseRunStatus(log []byte, exitCode int, oomKilled bool) string {
	switch {
	case bytes.Contains(log, []byte("run time >= time limit")):
		return validate.StatusTimeExceeded
	case oomKilled:
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
