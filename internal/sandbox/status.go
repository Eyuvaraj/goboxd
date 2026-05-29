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
// In cgroup v2 mode (--detect_cgroupv2 + --cgroup_mem_max), OOM kills arrive as
// SIGKILL with nsjail logging "killed by signal 9". We match "memory.max" (which
// nsjail logs when setting the limit) combined with signal 9 to surface memory_exceeded.
// When signal 9 occurs without any cgroup memory context, it is runtime_error.
func ParseRunStatus(log []byte, exitCode int) string {
	logLow := bytes.ToLower(log)
	switch {
	case bytes.Contains(log, []byte("run time >= time limit")):
		return validate.StatusTimeExceeded
	// Cgroup v1 and kernel OOM patterns.
	case bytes.Contains(logLow, []byte("memory limit exceeded")),
		bytes.Contains(logLow, []byte("out of memory")),
		bytes.Contains(logLow, []byte("oom kill")),
		bytes.Contains(logLow, []byte("cgroup memory")):
		return validate.StatusMemoryExceeded
	// Cgroup v2: nsjail logs "Setting 'memory.max' to '<n>'" when the limit is
	// configured. An SIGKILL in the same log session → the cgroup limit was hit.
	case bytes.Contains(logLow, []byte("memory.max")) &&
		bytes.Contains(log, []byte("killed by signal")):
		return validate.StatusMemoryExceeded
	// SIGXCPU (CPU time limit exceeded) fires when the rlimit_cpu guard is hit.
	// rlimit_cpu is set one second above --time_limit as a fallback; if it
	// fires anyway (e.g. under heavy system load), it still means a time limit
	// was the cause — report time_exceeded, not runtime_error.
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
