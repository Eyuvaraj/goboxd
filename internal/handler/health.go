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
	probes     *registry.ProbeCache
	nsjailPath string
	cfg        config.Server
	counters   *stats.Counters
}

func NewHealthHandler(reg *registry.Registry, probes *registry.ProbeCache, cfg config.Server, counters *stats.Counters) *HealthHandler {
	return &HealthHandler{reg: reg, probes: probes, nsjailPath: cfg.NsjailPath, cfg: cfg, counters: counters}
}

// Healthz godoc
//
//	@Summary		Liveness check
//	@Description	Returns 200 as long as the server process is running. Intended for load-balancer liveness probes.
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	HealthzResponse
//	@Router			/healthz [get]
func (h *HealthHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Readyz godoc
//
//	@Summary		Readiness check
//	@Description	Probes nsjail and every registered language binary. Returns 200 when all probes pass; 503 when any probe fails. Use this for load-balancer readiness probes.
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	ReadyzResponse
//	@Failure		503	{object}	ReadyzResponse	"One or more probes failed — service is degraded"
//	@Router			/readyz [get]
func (h *HealthHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	nsjailResult := h.probes.Nsjail()
	langResults := h.probes.Languages()

	status := "ok"
	code := http.StatusOK
	if !h.probes.AllOK() {
		status = "degraded"
		code = http.StatusServiceUnavailable
	}

	writeJSON(w, code, map[string]any{
		"status":    status,
		"nsjail":    nsjailResult,
		"languages": langResults,
	})
}

// Info godoc
//
//	@Summary		Server info and stats
//	@Description	Returns build metadata (version, commit, Go version), nsjail info, registered language list with default limits, server-wide enforcement limits, and live runtime statistics (in-flight jobs, totals, disk space).
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	InfoResponse
//	@Router			/info [get]
func (h *HealthHandler) Info(w http.ResponseWriter, r *http.Request) {
	nsjailResult := h.probes.Nsjail()
	langProbes := h.probes.Languages()

	langs := make([]map[string]any, 0, h.reg.Len())
	for _, lang := range h.reg.All() {
		probe := langProbes[lang.ID]
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
			"in_flight_jobs":           h.counters.InFlight(),
			"queue_size":               h.counters.QueueSize(),
			"jobs_total":               h.counters.JobsTotal(),
			"jobs_failed_internal":     h.counters.JobsFailed(),
			"last_internal_error_at":   errAtStr,
			"disk_free_bytes_jail_dir": stats.DiskFreeBytes(h.cfg.JailDir),
		},
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
