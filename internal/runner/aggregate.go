package runner

import "github.com/thesouldev/goboxd/internal/validate"

// TopLevelStatus derives the overall request status from build and test results.
// build_failed if build not ok; accepted if all tests pass; otherwise first failing test status.
func TopLevelStatus(buildStatus string, tests []TestResult) string {
	if buildStatus != validate.BuildStatusOK {
		return validate.StatusBuildFailed
	}
	for _, t := range tests {
		if t.Status != validate.StatusAccepted {
			return t.Status
		}
	}
	return validate.StatusAccepted
}
