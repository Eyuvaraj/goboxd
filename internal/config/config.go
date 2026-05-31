package config

import (
	"os"
	"runtime"
	"strconv"
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
		MaxConcurrentJobs: envInt("MAX_CONCURRENT_JOBS", runtime.NumCPU()),
		MaxOutputBytes:    envInt("MAX_OUTPUT_BYTES", 256*1024),
		MaxStdinBytes:     envInt("MAX_STDIN_BYTES", 64*1024),
		OrphanMaxAge:      envInt("ORPHAN_MAX_AGE_MINUTES", 30),
		MaxQueueDepth:     envInt("MAX_QUEUE_DEPTH", 0),
	}
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
