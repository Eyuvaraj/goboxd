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
	OrphanMaxAge      int // minutes
	MaxQueueDepth     int // waiting requests before shedding with 503; 0 = unbounded queue
}

func Load() Server {
	return Server{
		Port:              env("PORT", "8080"),
		NsjailPath:        env("NSJAIL_PATH", "/usr/local/bin/nsjail"),
		JailDir:           env("JAIL_DIR", "/tmp/goboxd"),
		LanguageFile:      env("LANGUAGE_FILE", "/etc/goboxd/languages.yaml"),
		MaxSourceBytes:    envInt("MAX_SOURCE_BYTES", 256*1024),
		MaxTests:          envInt("MAX_TESTS", 50),
		MaxConcurrentJobs: envInt("MAX_CONCURRENT_JOBS", AvailableCPUs()),
		MaxOutputBytes:    envInt("MAX_OUTPUT_BYTES", 256*1024),
		MaxStdinBytes:     envInt("MAX_STDIN_BYTES", 64*1024),
		OrphanMaxAge:      envInt("ORPHAN_MAX_AGE_MINUTES", 30),
		MaxQueueDepth:     envInt("MAX_QUEUE_DEPTH", 0),
	}
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
	cpus := (quota + period - 1) / period // round up
	if cpus < 1 {
		cpus = 1
	}
	return cpus
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
