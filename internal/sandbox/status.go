package sandbox

import (
	"bytes"

	"github.com/thesouldev/goboxd/internal/validate"
)

// ParseRunStatus interprets nsjail's log output to determine the test outcome.
// nsjail writes status lines like:
//
//	[E][...] run time >= time limit
//	[W][...] prctl(PR_SET_DUMPABLE, 1): Operation not permitted
//	[I][...] PID 12345 exited with status: 0
//	[I][...] killed by signal 11
//
// Cgroup v2 OOM kills arrive as SIGKILL; nsjail additionally logs the OOM event.
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
	case bytes.Contains(log, []byte("killed by signal")):
		return validate.StatusRuntimeError
	case exitCode == 0:
		return validate.StatusAccepted
	default:
		return validate.StatusRuntimeError
	}
}

// ParseBuildStatus maps build-phase nsjail log output and exit code to build status.
// log is the nsjail diagnostic log from --log_fd 3, not the compiler's stderr.
// [E][ prefix lines mean nsjail itself failed to exec/mount (internal_error).
// Any other non-zero exit means the compiler ran and failed (failed).
func ParseBuildStatus(log []byte, exitCode int) string {
	switch {
	case exitCode == 0:
		return validate.BuildStatusOK
	case bytes.Contains(log, []byte("[E][")):
		return validate.BuildStatusInternalError
	default:
		return validate.BuildStatusFailed
	}
}
