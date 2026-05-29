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
	writeJSON(w, http.StatusOK, HealthzResponse{Status: "ok"})
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
	probes := h.probes.Languages()
	languages := make(map[string]ProbeInfo, len(probes))
	for id, p := range probes {
		languages[id] = toProbeInfo(p)
	}

	status, code := "ok", http.StatusOK
	if !h.probes.AllOK() {
		status, code = "degraded", http.StatusServiceUnavailable
	}

	writeJSON(w, code, ReadyzResponse{
		Status:    status,
		Nsjail:    toProbeInfo(h.probes.Nsjail()),
		Languages: languages,
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
	langProbes := h.probes.Languages()

	languages := make([]LanguageInfo, 0, h.reg.Len())
	for _, lang := range h.reg.All() {
		languages = append(languages, LanguageInfo{
			ID:      lang.ID,
			Name:    lang.Name,
			Version: langProbes[lang.ID].Version,
			DefaultRunLimits: LanguageRunLimits{
				WallTimeS:    lang.Run.Limits.WallTimeS,
				MemoryKB:     lang.Run.Limits.MemoryKB,
				MaxProcesses: lang.Run.Limits.MaxProcesses,
			},
		})
	}

	var lastErrorAt *string
	if at := h.counters.LastInternalErrorAt(); !at.IsZero() {
		s := at.UTC().Format(time.RFC3339)
		lastErrorAt = &s
	}

	writeJSON(w, http.StatusOK, InfoResponse{
		BuildInfo: BuildInfo{Version: Version, Commit: Commit, GoVersion: GoVersion},
		Nsjail:    NsjailInfo{Path: h.nsjailPath, Version: h.probes.Nsjail().Version},
		Languages: languages,
		Limits: ServiceLimits{
			MaxSourceBytes:    h.cfg.MaxSourceBytes,
			MaxTests:          h.cfg.MaxTests,
			MaxConcurrentJobs: h.cfg.MaxConcurrentJobs,
		},
		Stats: ServiceStats{
			InFlightJobs:        h.counters.InFlight(),
			QueueSize:           h.counters.QueueSize(),
			JobsTotal:           h.counters.JobsTotal(),
			JobsFailedInternal:  h.counters.JobsFailed(),
			LastInternalErrorAt: lastErrorAt,
			DiskFreeByteJailDir: stats.DiskFreeBytes(h.cfg.JailDir),
		},
	})
}

// toProbeInfo converts a registry probe result into the API response shape.
func toProbeInfo(r registry.ProbeResult) ProbeInfo {
	return ProbeInfo{OK: r.OK, Version: r.Version, Error: r.Error}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
