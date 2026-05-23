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
	Tests            []TestCase     `json:"tests"`
}

// PhaseOverride lets the client override limits and supply extra flags.
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
	Status string      `json:"status"`
	Build  BuildResult `json:"build"`
	Tests  []TestResult `json:"tests"`
}

// BuildResult is the build phase result in the response.
type BuildResult struct {
	Status     string `json:"status"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	DurationMs int64  `json:"duration_ms"`
}

// TestResult is one test case result in the response.
type TestResult struct {
	Status       string `json:"status"`
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr"`
	DurationMs   int64  `json:"duration_ms"`
	MemoryPeakKB int64  `json:"memory_peak_kb,omitempty"`
}

// ErrorResponse is returned for 4xx errors.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail carries the error code and human-readable message.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
