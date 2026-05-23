package runner_test

import (
	"testing"

	"github.com/thesouldev/goboxd/internal/runner"
	"github.com/thesouldev/goboxd/internal/validate"
)

func makeTests(statuses ...string) []runner.TestResult {
	out := make([]runner.TestResult, len(statuses))
	for i, s := range statuses {
		out[i] = runner.TestResult{Status: s}
	}
	return out
}

func TestTopLevelStatus(t *testing.T) {
	tests := []struct {
		name        string
		buildStatus string
		testStatuses []string
		want        string
	}{
		{
			name:        "build failed → build_failed",
			buildStatus: validate.BuildStatusFailed,
			testStatuses: []string{validate.StatusNotExecuted},
			want:        validate.StatusBuildFailed,
		},
		{
			name:        "all accepted",
			buildStatus: validate.BuildStatusOK,
			testStatuses: []string{validate.StatusAccepted, validate.StatusAccepted},
			want:        validate.StatusAccepted,
		},
		{
			name:        "first non-accepted wins",
			buildStatus: validate.BuildStatusOK,
			testStatuses: []string{validate.StatusAccepted, validate.StatusWrongOutput, validate.StatusTimeExceeded},
			want:        validate.StatusWrongOutput,
		},
		{
			name:        "single time exceeded",
			buildStatus: validate.BuildStatusOK,
			testStatuses: []string{validate.StatusTimeExceeded},
			want:        validate.StatusTimeExceeded,
		},
		{
			name:        "whitespace mismatch",
			buildStatus: validate.BuildStatusOK,
			testStatuses: []string{validate.StatusWhitespaceMismatch},
			want:        validate.StatusWhitespaceMismatch,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := runner.TopLevelStatus(tc.buildStatus, makeTests(tc.testStatuses...))
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
