package sandbox

import (
	"bytes"

	"github.com/thesouldev/goboxd/internal/validate"
)

// ParseRunStatus interprets nsjail's stderr output to determine the test
// outcome. nsjail writes status lines like:
//   [E][...] run time >= time limit
//   [W][...] prctl(PR_SET_DUMPABLE, 1): Operation not permitted
//   [I][...] PID 12345 exited with status: 0
//   [I][...] killed by signal 11
func ParseRunStatus(stderr []byte, exitCode int) string {
	switch {
	case bytes.Contains(stderr, []byte("run time >= time limit")):
		return validate.StatusTimeExceeded
	case bytes.Contains(stderr, []byte("memory limit exceeded")):
		return validate.StatusMemoryExceeded
	case bytes.Contains(stderr, []byte("out of memory")):
		return validate.StatusMemoryExceeded
	case bytes.Contains(stderr, []byte("killed by signal")):
		return validate.StatusRuntimeError
	case exitCode == 0:
		return validate.StatusAccepted
	default:
		return validate.StatusRuntimeError
	}
}

// ParseBuildStatus maps build-phase exit codes and stderr to build status.
func ParseBuildStatus(stderr []byte, exitCode int) string {
	switch {
	case exitCode == 0:
		return validate.BuildStatusOK
	case bytes.Contains(stderr, []byte("nsjail")):
		// nsjail itself failed (not the compiler) — internal error
		return validate.BuildStatusInternalError
	default:
		return validate.BuildStatusFailed
	}
}
