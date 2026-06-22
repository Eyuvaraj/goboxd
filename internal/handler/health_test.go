package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/thesouldev/goboxd/internal/config"
	"github.com/thesouldev/goboxd/internal/handler"
	"github.com/thesouldev/goboxd/internal/registry"
	"github.com/thesouldev/goboxd/internal/stats"
)

// healthTestLangsYAML uses nonexistent binaries so language probes fail quickly
// without waiting for a timeout (exec returns immediately when binary is missing).
const healthTestLangsYAML = `
languages:
  - id: fake
    name: Fake
    source_filename: solution.fake
    run:
      cmd: /nonexistent/fake-interpreter
      args: ["{{source}}"]
      limits:
        wall_time_s: 5
        memory_kb: 1024
        max_processes: 10
`

func newHealthRegistry(t *testing.T) *registry.Registry {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "health-langs-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(healthTestLangsYAML)
	_ = f.Close()
	reg, err := registry.Load(f.Name())
	if err != nil {
		t.Fatalf("load health registry: %v", err)
	}
	return reg
}

type stubLive struct{ inFlight, queued int64 }

func (s *stubLive) InFlight() int64  { return s.inFlight }
func (s *stubLive) QueueSize() int64 { return s.queued }

func newHealthHandler(t *testing.T) *handler.HealthHandler {
	t.Helper()
	reg := newHealthRegistry(t)
	// ProbeCache tries to exec each binary. With nonexistent paths the probe
	// fails immediately (no timeout wait), so this is fast even on macOS.
	probes := registry.NewProbeCache(reg, "/nonexistent/nsjail")
	cfg := config.Server{
		NsjailPath:        "/nonexistent/nsjail",
		JailDir:           t.TempDir(),
		MaxSourceBytes:    256 * 1024,
		MaxTests:          50,
		MaxConcurrentJobs: 1,
	}
	return handler.NewHealthHandler(reg, probes, cfg, &stats.Counters{}, &stubLive{})
}

func TestHealthz(t *testing.T) {
	h := newHealthHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	h.Healthz(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("want status=ok, got %q", body["status"])
	}
}

func TestReadyz_Structure(t *testing.T) {
	h := newHealthHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	h.Readyz(w, req)

	// We don't assert on 200 vs 503 since probes fail (no binaries in test env).
	// Assert the response body has the required fields.
	var body struct {
		Status    string                       `json:"status"`
		Nsjail    struct{ OK bool }            `json:"nsjail"`
		Languages map[string]struct{ OK bool } `json:"languages"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode /readyz body (code=%d, body=%s): %v", w.Code, w.Body.String(), err)
	}
	if body.Status == "" {
		t.Error("status field missing from /readyz response")
	}
	if body.Status != "ok" && body.Status != "degraded" {
		t.Errorf("status must be ok or degraded, got %q", body.Status)
	}
	// In CI/test environment probes fail, so languages should be present but may not be OK.
	if _, ok := body.Languages["fake"]; !ok {
		t.Error("expected languages map to contain the registered language")
	}
}

func TestReadyz_DegradedWhenProbesFail(t *testing.T) {
	h := newHealthHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	h.Readyz(w, req)

	// Probes are known to fail (nonexistent binaries) → expect 503 + "degraded".
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 when probes fail, got %d", w.Code)
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "degraded" {
		t.Fatalf("want degraded, got %q", body.Status)
	}
}

func TestInfo_Structure(t *testing.T) {
	h := newHealthHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/info", nil)
	w := httptest.NewRecorder()
	h.Info(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (body: %s)", w.Code, w.Body.String())
	}
	var body struct {
		BuildInfo struct {
			GoVersion string `json:"go_version"`
		} `json:"build_info"`
		Languages []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"languages"`
		Limits struct {
			MaxSourceBytes    int `json:"max_source_bytes"`
			MaxTests          int `json:"max_tests"`
			MaxConcurrentJobs int `json:"max_concurrent_jobs"`
		} `json:"limits"`
		Stats struct {
			InFlightJobs int64 `json:"in_flight_jobs"`
			JobsTotal    int64 `json:"jobs_total"`
		} `json:"stats"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode /info body: %v", err)
	}
	if body.BuildInfo.GoVersion == "" {
		t.Error("build_info.go_version missing")
	}
	if len(body.Languages) == 0 {
		t.Error("languages list is empty")
	}
	if body.Languages[0].ID != "fake" {
		t.Errorf("expected first language id=fake, got %q", body.Languages[0].ID)
	}
	if body.Limits.MaxSourceBytes == 0 {
		t.Error("limits.max_source_bytes missing")
	}
	if body.Limits.MaxTests == 0 {
		t.Error("limits.max_tests missing")
	}
}

func TestInfo_StatsLive(t *testing.T) {
	reg := newHealthRegistry(t)
	probes := registry.NewProbeCache(reg, "/nonexistent/nsjail")
	cfg := config.Server{
		NsjailPath:        "/nonexistent/nsjail",
		JailDir:           t.TempDir(),
		MaxSourceBytes:    256 * 1024,
		MaxTests:          50,
		MaxConcurrentJobs: 4,
	}
	counters := &stats.Counters{}
	counters.IncTotal()
	counters.IncTotal()
	h := handler.NewHealthHandler(reg, probes, cfg, counters, &stubLive{})

	req := httptest.NewRequest(http.MethodGet, "/info", nil)
	w := httptest.NewRecorder()
	h.Info(w, req)

	var body struct {
		Stats struct {
			JobsTotal         int64 `json:"jobs_total"`
			MaxConcurrentJobs int   `json:"max_concurrent_jobs"`
		} `json:"stats"`
		Limits struct {
			MaxConcurrentJobs int `json:"max_concurrent_jobs"`
		} `json:"limits"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Stats.JobsTotal != 2 {
		t.Errorf("jobs_total: want 2, got %d", body.Stats.JobsTotal)
	}
	if body.Limits.MaxConcurrentJobs != 4 {
		t.Errorf("max_concurrent_jobs: want 4, got %d", body.Limits.MaxConcurrentJobs)
	}
}
