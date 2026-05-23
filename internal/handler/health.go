package handler

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"

	"github.com/thesouldev/goboxd/internal/config"
	"github.com/thesouldev/goboxd/internal/registry"
	"github.com/thesouldev/goboxd/internal/stats"
)

// Build-time variables injected via -ldflags.
var (
	Version   = "dev"
	Commit    = "unknown"
	GoVersion = runtime.Version()
)

// HealthHandler returns a HealthzHandler and ReadyzHandler pair.
type HealthHandler struct {
	reg        *registry.Registry
	nsjailPath string
	cfg        config.Server
	counters   *stats.Counters
}

func NewHealthHandler(reg *registry.Registry, cfg config.Server, counters *stats.Counters) *HealthHandler {
	return &HealthHandler{reg: reg, nsjailPath: cfg.NsjailPath, cfg: cfg, counters: counters}
}

// Healthz is a cheap liveness check — just confirms the process is up.
func (h *HealthHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Readyz checks nsjail and every language binary. Returns 503 if anything is broken.
func (h *HealthHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	nsjailResult := registry.ProbeNsjail(h.nsjailPath)

	langResults := map[string]registry.ProbeResult{}
	allOK := nsjailResult.OK
	for _, lang := range h.reg.All() {
		res := registry.ProbeLanguage(lang)
		langResults[lang.ID] = res
		if !res.OK {
			allOK = false
		}
	}

	status := "ok"
	code := http.StatusOK
	if !allOK {
		status = "degraded"
		code = http.StatusServiceUnavailable
	}

	writeJSON(w, code, map[string]any{
		"status":    status,
		"nsjail":    nsjailResult,
		"languages": langResults,
	})
}

// Info returns build metadata, nsjail version, language list, limits, and stats.
func (h *HealthHandler) Info(w http.ResponseWriter, r *http.Request) {
	nsjailResult := registry.ProbeNsjail(h.nsjailPath)

	langs := make([]map[string]any, 0)
	for _, lang := range h.reg.All() {
		probe := registry.ProbeLanguage(lang)
		entry := map[string]any{
			"id":      lang.ID,
			"name":    lang.Name,
			"version": probe.Version,
			"default_run_limits": map[string]any{
				"wall_time_s":   lang.Run.Limits.WallTimeS,
				"memory_kb":     lang.Run.Limits.MemoryKB,
				"max_processes": lang.Run.Limits.MaxProcesses,
			},
		}
		langs = append(langs, entry)
	}

	errAt := h.counters.LastInternalErrorAt()
	var errAtStr *string
	if !errAt.IsZero() {
		s := errAt.UTC().Format(time.RFC3339)
		errAtStr = &s
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"build_info": map[string]string{
			"version":    Version,
			"commit":     Commit,
			"go_version": GoVersion,
		},
		"nsjail": map[string]any{
			"path":    h.nsjailPath,
			"version": nsjailResult.Version,
		},
		"languages": langs,
		"limits": map[string]any{
			"max_source_bytes":    h.cfg.MaxSourceBytes,
			"max_tests":           h.cfg.MaxTests,
			"max_concurrent_jobs": h.cfg.MaxConcurrentJobs,
		},
		"stats": map[string]any{
			"in_flight_jobs":          h.counters.InFlight(),
			"jobs_total":              h.counters.JobsTotal(),
			"jobs_failed_internal":    h.counters.JobsFailed(),
			"last_internal_error_at":  errAtStr,
			"disk_free_bytes_jail_dir": stats.DiskFreeBytes(h.cfg.JailDir),
		},
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
