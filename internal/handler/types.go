package handler

import "github.com/thesouldev/goboxd/internal/config"

// ── Run endpoint ─────────────────────────────────────────────────────────────

// RunRequest is the JSON body for POST /run.
type RunRequest struct {
	Language         string         `json:"language"          example:"py3"`
	Source           string         `json:"source"            example:"print(input())"`
	SourceFilename   string         `json:"source_filename"   example:"Main.java"`
	ArtifactFilename string         `json:"artifact_filename" example:"Main"`
	Build            *PhaseOverride `json:"build"`
	Run              *PhaseOverride `json:"run"`
	Tests            []TestCase     `json:"tests"`
}

// PhaseOverride lets the client override limits and supply extra flags for a phase.
type PhaseOverride struct {
	Limits config.LimitsDef `json:"limits"`
	Flags  []string         `json:"flags" example:"-O2"`
}

// TestCase is one stdin/expected_stdout pair in the request.
type TestCase struct {
	Stdin          string `json:"stdin"           example:"hello\n"`
	ExpectedStdout string `json:"expected_stdout" example:"hello\n"`
}

// RunResponse is the JSON body returned by POST /run.
type RunResponse struct {
	// Status summarises the overall result. "accepted" means build succeeded and all tests passed.
	Status string       `json:"status" enums:"accepted,build_failed,wrong_output,output_whitespace_mismatch,time_exceeded,memory_exceeded,runtime_error,internal_error" example:"accepted"`
	Build  BuildResult  `json:"build"`
	Tests  []TestResult `json:"tests"`
}

// BuildResult is the build phase result embedded in RunResponse.
type BuildResult struct {
	Status     string `json:"status"      enums:"ok,failed,internal_error" example:"ok"`
	Stdout     string `json:"stdout"      example:""`
	Stderr     string `json:"stderr"      example:""`
	DurationMs int64  `json:"duration_ms" example:"412"`
}

// TestResult is one test-case result embedded in RunResponse.
type TestResult struct {
	Status       string `json:"status"        enums:"accepted,wrong_output,output_whitespace_mismatch,time_exceeded,memory_exceeded,runtime_error,not_executed,internal_error" example:"accepted"`
	Stdout       string `json:"stdout"        example:"hello"`
	Stderr       string `json:"stderr"        example:""`
	DurationMs   int64  `json:"duration_ms"   example:"38"`
	MemoryPeakKB int64  `json:"memory_peak_kb,omitempty" example:"1024"`
}

// ErrorResponse is returned for 4xx/5xx errors.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail carries the machine-readable error code and human-readable message.
type ErrorDetail struct {
	// Code is a stable machine-readable identifier.
	Code    string `json:"code"    enums:"invalid_json,unknown_language,missing_source,source_too_large,missing_source_filename,missing_artifact_filename,invalid_filename,invalid_flag,invalid_limits,invalid_test_count,stdin_too_large,expected_too_large,internal_error" example:"unknown_language"`
	Message string `json:"message" example:"language \"cobol\" is not registered"`
}

// ── Health endpoints ──────────────────────────────────────────────────────────

// HealthzResponse is returned by GET /healthz.
type HealthzResponse struct {
	Status string `json:"status" example:"ok"`
}

// ProbeInfo is a runtime probe result for one binary (nsjail or a language runtime).
type ProbeInfo struct {
	OK      bool   `json:"ok"               example:"true"`
	Version string `json:"version,omitempty" example:"Python 3.11.9"`
	Error   string `json:"error,omitempty"   example:""`
}

// ReadyzResponse is returned by GET /readyz.
type ReadyzResponse struct {
	// Status is "ok" when all probes pass; "degraded" when any probe fails.
	Status    string               `json:"status"    enums:"ok,degraded" example:"ok"`
	Nsjail    ProbeInfo            `json:"nsjail"`
	Languages map[string]ProbeInfo `json:"languages"`
}

// BuildInfo holds version metadata returned by GET /info.
type BuildInfo struct {
	Version   string `json:"version"    example:"0.1.0"`
	Commit    string `json:"commit"     example:"abc1234"`
	GoVersion string `json:"go_version" example:"go1.23.0"`
}

// NsjailInfo holds nsjail path and version for GET /info.
type NsjailInfo struct {
	Path    string `json:"path"    example:"/usr/local/bin/nsjail"`
	Version string `json:"version" example:"nsjail version: 3.6"`
}

// LanguageRunLimits holds the default run-phase limits for one language in GET /info.
type LanguageRunLimits struct {
	WallTimeS    int `json:"wall_time_s"   example:"10"`
	MemoryKB     int `json:"memory_kb"     example:"102400"`
	MaxProcesses int `json:"max_processes" example:"100"`
}

// LanguageInfo is one language entry in InfoResponse.
type LanguageInfo struct {
	ID               string            `json:"id"                 example:"py3"`
	Name             string            `json:"name"               example:"Python 3"`
	Version          string            `json:"version"            example:"Python 3.11.9"`
	DefaultRunLimits LanguageRunLimits `json:"default_run_limits"`
}

// ServiceLimits holds server-wide enforcement limits for GET /info.
type ServiceLimits struct {
	MaxSourceBytes    int `json:"max_source_bytes"    example:"262144"`
	MaxTests          int `json:"max_tests"           example:"50"`
	MaxConcurrentJobs int `json:"max_concurrent_jobs" example:"8"`
}

// ServiceStats holds runtime counters for GET /info.
type ServiceStats struct {
	InFlightJobs           int64   `json:"in_flight_jobs"           example:"2"`
	JobsTotal              int64   `json:"jobs_total"               example:"1042"`
	JobsFailedInternal     int64   `json:"jobs_failed_internal"     example:"0"`
	LastInternalErrorAt    *string `json:"last_internal_error_at"   example:"2024-01-15T10:30:00Z"`
	DiskFreeByteJailDir    int64   `json:"disk_free_bytes_jail_dir" example:"10737418240"`
}

// InfoResponse is returned by GET /info.
type InfoResponse struct {
	BuildInfo BuildInfo      `json:"build_info"`
	Nsjail    NsjailInfo     `json:"nsjail"`
	Languages []LanguageInfo `json:"languages"`
	Limits    ServiceLimits  `json:"limits"`
	Stats     ServiceStats   `json:"stats"`
}
