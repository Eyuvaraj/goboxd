package runner

import "github.com/thesouldev/goboxd/internal/validate"

// TopLevelStatus derives the overall request status from build and test results.
// Returns build_failed if build not ok. Otherwise, if all tests pass it returns accepted.
// If any test fails, the status of the first non-accepted test wins and is returned.
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
