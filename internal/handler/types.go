package handler

import "github.com/thesouldev/goboxd/internal/config"

// RunRequest is the JSON body for POST /run.
type RunRequest struct {
	Language         string         `json:"language"`
	Source           string         `json:"source"`
	SourceFilename   string         `json:"source_filename"`
	ArtifactFilename string         `json:"artifact_filename"`
	Build            *PhaseOverride `json:"build"`
	Run              *PhaseOverride `json:"run"`
	Stdin            string         `json:"stdin"` // raw mode: single stdin when tests is empty
	Tests            []TestCase     `json:"tests"`
}

// PhaseOverride lets the client supply per-request flags and limit overrides.
type PhaseOverride struct {
	Limits config.LimitsDef `json:"limits"`
	Flags  []string         `json:"flags"`
}

// TestCase is one stdin/expected_stdout pair in the request.
type TestCase struct {
	Stdin          string `json:"stdin"`
	ExpectedStdout string `json:"expected_stdout"`
}

// RunResponse is the JSON body returned by POST /run.
type RunResponse struct {
	Status string       `json:"status"`
	Build  BuildResult  `json:"build"`
	Tests  []TestResult `json:"tests"`
}

type BuildResult struct {
	Status     string `json:"status"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	DurationMs int64  `json:"duration_ms"`
}

type TestResult struct {
	Status       string `json:"status"`
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr"`
	ExitCode     int    `json:"exit_code"`
	DurationMs   int64  `json:"duration_ms"`
	MemoryPeakKB int64  `json:"memory_peak_kb"`
}

// ErrorResponse is returned for 4xx/5xx errors.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail carries a stable machine-readable code and a human-readable message.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type HealthzResponse struct {
	Status string `json:"status"`
}

type ProbeInfo struct {
	OK      bool   `json:"ok"`
	Version string `json:"version,omitempty"`
	Error   string `json:"error,omitempty"`
}

type ReadyzResponse struct {
	Status    string               `json:"status"`
	Nsjail    ProbeInfo            `json:"nsjail"`
	Languages map[string]ProbeInfo `json:"languages"`
}

type BuildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	GoVersion string `json:"go_version"`
}

type NsjailInfo struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}

type LanguageRunLimits struct {
	WallTimeS    int `json:"wall_time_s"`
	MemoryKB     int `json:"memory_kb"`
	MaxProcesses int `json:"max_processes"`
}

type LanguageInfo struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Version          string            `json:"version"`
	DefaultRunLimits LanguageRunLimits `json:"default_run_limits"`
}

type ServiceLimits struct {
	MaxSourceBytes    int `json:"max_source_bytes"`
	MaxTests          int `json:"max_tests"`
	MaxConcurrentJobs int `json:"max_concurrent_jobs"`
}

type ServiceStats struct {
	InFlightJobs        int64   `json:"in_flight_jobs"`
	QueueSize           int64   `json:"queue_size"`
	JobsTotal           int64   `json:"jobs_total"`
	JobsFailedInternal  int64   `json:"jobs_failed_internal"`
	LastInternalErrorAt *string `json:"last_internal_error_at"`
	DiskFreeByteJailDir int64   `json:"disk_free_bytes_jail_dir"`
}

// InfoResponse is returned by GET /info.
type InfoResponse struct {
	BuildInfo BuildInfo      `json:"build_info"`
	Nsjail    NsjailInfo     `json:"nsjail"`
	Languages []LanguageInfo `json:"languages"`
	Limits    ServiceLimits  `json:"limits"`
	Stats     ServiceStats   `json:"stats"`
}
