package validate

// Status vocabulary — top-level and per-entity values from the spec.
const (
	StatusAccepted           = "accepted"
	StatusBuildFailed        = "build_failed"
	StatusWrongOutput        = "wrong_output"
	StatusWhitespaceMismatch = "output_whitespace_mismatch"
	StatusTimeExceeded       = "time_exceeded"
	StatusMemoryExceeded     = "memory_exceeded"
	StatusRuntimeError       = "runtime_error"
	StatusNotExecuted        = "not_executed"
	StatusInternalError      = "internal_error"

	BuildStatusOK            = "ok"
	BuildStatusFailed        = "failed"
	BuildStatusInternalError = "internal_error"
)
