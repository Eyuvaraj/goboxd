package sandbox_test

import (
	"testing"

	"github.com/thesouldev/goboxd/internal/config"
	"github.com/thesouldev/goboxd/internal/sandbox"
)

func TestMergeLimits(t *testing.T) {
	base := config.LimitsDef{WallTimeS: 5, MemoryKB: 102400, MaxProcesses: 100}

	// Zero-value override changes nothing.
	result := sandbox.MergeLimits(base, config.LimitsDef{})
	if result != base {
		t.Fatalf("expected %+v, got %+v", base, result)
	}

	// Partial override: only WallTimeS.
	result = sandbox.MergeLimits(base, config.LimitsDef{WallTimeS: 10})
	if result.WallTimeS != 10 || result.MemoryKB != 102400 || result.MaxProcesses != 100 {
		t.Fatalf("unexpected result: %+v", result)
	}

	// Full override.
	override := config.LimitsDef{WallTimeS: 1, MemoryKB: 1024, MaxProcesses: 10}
	result = sandbox.MergeLimits(base, override)
	if result != override {
		t.Fatalf("expected %+v, got %+v", override, result)
	}
}

func TestExpandArgs(t *testing.T) {
	tests := []struct {
		name     string
		tmpl     []string
		source   string
		artifact string
		flags    []string
		want     []string
	}{
		{
			name:     "flags expanded in place",
			tmpl:     []string{"{{flags}}", "-o", "{{artifact}}", "{{source}}"},
			source:   "a.c", artifact: "a",
			flags: []string{"-O2", "-Wall"},
			want:  []string{"-O2", "-Wall", "-o", "a", "a.c"},
		},
		{
			name:     "no flags",
			tmpl:     []string{"{{flags}}", "{{source}}"},
			source:   "sol.py", artifact: "",
			flags: nil,
			want:  []string{"sol.py"},
		},
		{
			name:     "inline substitution",
			tmpl:     []string{"./{{artifact}}"},
			source:   "", artifact: "solution",
			flags: nil,
			want:  []string{"./solution"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sandbox.ExpandArgs(tc.tmpl, tc.source, tc.artifact, tc.flags)
			if len(got) != len(tc.want) {
				t.Fatalf("len mismatch: got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("arg[%d]: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}
