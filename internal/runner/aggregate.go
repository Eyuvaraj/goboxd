package runner

import "github.com/thesouldev/goboxd/internal/validate"

// TopLevelStatus derives the overall request status from build and test results.
// Rules from the spec:
//   - If build failed → "build_failed" (all tests will be "not_executed")
//   - Accepted only if every test is accepted
//   - Otherwise the first non-accepted test status wins
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
