package config

import (
	"os"
	"runtime"
	"strconv"
	"strings"
)

type Server struct {
	Port              string
	NsjailPath        string
	JailDir           string
	LanguageFile      string
	MaxSourceBytes    int
	MaxTests          int
	MaxConcurrentJobs int
	MaxOutputBytes    int
	MaxStdinBytes     int
	OrphanMaxAge      int   // minutes
	MaxQueueDepth     int   // max waiters before 503 load-shedding; 0 = unbounded (default); set MAX_QUEUE_DEPTH to enable
	MemBudgetKB       int64 // total memory admission budget in KiB; 0 disables memory-based admission
}

func Load() Server {
	maxConcurrent := envInt("MAX_CONCURRENT_JOBS", AvailableCPUs())
	return Server{
		Port:              env("PORT", "8080"),
		NsjailPath:        env("NSJAIL_PATH", "/usr/local/bin/nsjail"),
		JailDir:           env("JAIL_DIR", "/tmp/goboxd"),
		LanguageFile:      env("LANGUAGE_FILE", "/etc/goboxd/languages.yaml"),
		MaxSourceBytes:    envInt("MAX_SOURCE_BYTES", 256*1024),
		MaxTests:          envInt("MAX_TESTS", 50),
		MaxConcurrentJobs: maxConcurrent,
		MaxOutputBytes:    envInt("MAX_OUTPUT_BYTES", 256*1024),
		MaxStdinBytes:     envInt("MAX_STDIN_BYTES", 64*1024),
		OrphanMaxAge:      envInt("ORPHAN_MAX_AGE_MINUTES", 30),
		MaxQueueDepth:     queueDepth(0),
		MemBudgetKB:       envInt64("MEM_BUDGET_KB", defaultMemBudgetKB()),
	}
}

// queueDepth parses MAX_QUEUE_DEPTH. Default is 0 (unbounded queue, no shedding).
// A positive integer enables load-shedding at that depth. Invalid/negative falls
// back to the default. Distinct from envInt: can't tell explicit 0 from unset.
func queueDepth(fallback int) int {
	if v := os.Getenv("MAX_QUEUE_DEPTH"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return fallback
}

// defaultMemBudgetKB reserves 80% of detected RAM for jobs, leaving headroom for
// the Go service and page cache. Returns 0 when RAM can't be detected (e.g.
// macOS dev box), which disables memory-based admission.
func defaultMemBudgetKB() int64 {
	return AvailableMemoryKB() * 8 / 10
}

// AvailableMemoryKB returns the memory ceiling the process must live within: the
// cgroup v2 memory.max when set (a memory-limited container), else /proc/meminfo
// MemTotal. Returns 0 when undetectable (non-Linux, cgroup v1, or unparseable).
func AvailableMemoryKB() int64 {
	if data, err := os.ReadFile("/sys/fs/cgroup/memory.max"); err == nil {
		s := strings.TrimSpace(string(data))
		if s != "max" {
			if v, err := strconv.ParseInt(s, 10, 64); err == nil && v > 0 {
				return v / 1024
			}
		}
	}
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		for line := range strings.SplitSeq(string(data), "\n") {
			if rest, ok := strings.CutPrefix(line, "MemTotal:"); ok {
				fields := strings.Fields(rest)
				if len(fields) >= 1 {
					if v, err := strconv.ParseInt(fields[0], 10, 64); err == nil && v > 0 {
						return v
					}
				}
			}
		}
	}
	return 0
}

// AvailableCPUs returns usable CPUs: the cgroup v2 quota when set and lower than
// NumCPU (a CPU-limited container), else runtime.NumCPU(). Always at least 1.
func AvailableCPUs() int {
	n := runtime.NumCPU()
	if q := cgroupCPUQuota(); q > 0 && q < n {
		return q
	}
	return n
}

// cgroupCPUQuota reads the cgroup v2 CPU quota in whole CPUs (rounded up), or 0
// when unset ("max"), absent (non-Linux/cgroup v1), or unparseable.
func cgroupCPUQuota() int {
	data, err := os.ReadFile("/sys/fs/cgroup/cpu.max")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) != 2 || fields[0] == "max" {
		return 0
	}
	quota, err := strconv.Atoi(fields[0])
	if err != nil || quota <= 0 {
		return 0
	}
	period, err := strconv.Atoi(fields[1])
	if err != nil || period <= 0 {
		return 0
	}
	return max((quota+period-1)/period, 1)
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func envInt64(key string, fallback int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}
