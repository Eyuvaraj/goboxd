package runner

import (
	"testing"

	"github.com/thesouldev/goboxd/internal/sandbox"
	"github.com/thesouldev/goboxd/internal/validate"
)

// compareOutput is unexported; test it from within the runner package.
func TestCompareOutput(t *testing.T) {
	okResult := func(stdout string) sandbox.RunResult {
		// exit 0, empty log → ParseRunStatus returns accepted
		return sandbox.RunResult{Stdout: []byte(stdout), ExitCode: 0}
	}

	tests := []struct {
		name     string
		actual   string
		expected string
		want     string
	}{
		{
			name:     "exact match",
			actual:   "hello\n",
			expected: "hello\n",
			want:     validate.StatusAccepted,
		},
		{
			name:     "trailing newline vs no newline",
			actual:   "hello\n",
			expected: "hello",
			want:     validate.StatusWhitespaceMismatch,
		},
		{
			name:     "trailing spaces",
			actual:   "hello   \n",
			expected: "hello",
			want:     validate.StatusWhitespaceMismatch,
		},
		{
			name:     "leading whitespace differs → wrong output not whitespace mismatch",
			actual:   "  hello\n",
			expected: "hello",
			want:     validate.StatusWrongOutput,
		},
		{
			name:     "completely different output",
			actual:   "world\n",
			expected: "hello\n",
			want:     validate.StatusWrongOutput,
		},
		{
			name:     "empty actual vs non-empty expected",
			actual:   "",
			expected: "hello\n",
			want:     validate.StatusWrongOutput,
		},
		{
			name:     "both empty",
			actual:   "",
			expected: "",
			want:     validate.StatusAccepted,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := compareOutput(okResult(tc.actual), tc.expected)
			if got != tc.want {
				t.Errorf("compareOutput(%q, %q) = %q, want %q", tc.actual, tc.expected, got, tc.want)
			}
		})
	}
}
