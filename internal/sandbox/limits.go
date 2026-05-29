package sandbox

import "github.com/thesouldev/goboxd/internal/config"

// MergeLimits returns base with any non-zero override fields applied.
func MergeLimits(base, override config.LimitsDef) config.LimitsDef {
	result := base
	if override.WallTimeS > 0 {
		result.WallTimeS = override.WallTimeS
	}
	if override.MemoryKB > 0 {
		result.MemoryKB = override.MemoryKB
	}
	if override.MaxProcesses > 0 {
		result.MaxProcesses = override.MaxProcesses
	}
	return result
}
