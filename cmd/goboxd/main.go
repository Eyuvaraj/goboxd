package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/thesouldev/goboxd/docs"
	"github.com/thesouldev/goboxd/internal/config"
	"github.com/thesouldev/goboxd/internal/handler"
	"github.com/thesouldev/goboxd/internal/playground"
	"github.com/thesouldev/goboxd/internal/registry"
	"github.com/thesouldev/goboxd/internal/runner"
	"github.com/thesouldev/goboxd/internal/sandbox"
	"github.com/thesouldev/goboxd/internal/stats"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// Match GOMAXPROCS to the cgroup CPU quota so a limited container isn't oversubscribed.
	runtime.GOMAXPROCS(config.AvailableCPUs())

	cfg := config.Load()

	sandbox.SweepOrphans(cfg.JailDir, time.Duration(cfg.OrphanMaxAge)*time.Minute)

	reg, err := registry.Load(cfg.LanguageFile)
	if err != nil {
		slog.Error("failed to load language registry", "error", err)
		os.Exit(1)
	}
	slog.Info("language registry loaded", "count", reg.Len())

	// First probe runs synchronously; background refresh every 30s.
	probes := registry.NewProbeCache(reg, cfg.NsjailPath)
	if r := probes.Nsjail(); !r.OK {
		slog.Error("nsjail probe failed at startup", "error", r.Error)
		os.Exit(1)
	}
	for id, r := range probes.Languages() {
		if !r.OK {
			slog.Warn("language probe failed at startup", "language", id, "error", r.Error)
		}
	}

	counters := &stats.Counters{}
	jobRunner := runner.New(cfg.MaxConcurrentJobs, reg, cfg, counters)

	healthH := handler.NewHealthHandler(reg, probes, cfg, counters, jobRunner)
	runH := handler.NewRunHandler(jobRunner, reg, cfg)

	maxBody := int64(cfg.MaxSourceBytes) +
		int64(cfg.MaxTests)*2*int64(cfg.MaxStdinBytes) +
		65536

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(handler.BodyLimit(maxBody))
	r.Use(handler.StructuredLogger)

	r.Get("/healthz", healthH.Healthz)
	r.Get("/readyz", healthH.Readyz)
	r.Get("/info", healthH.Info)
	r.Post("/run", runH.ServeHTTP)
	r.Post("/v1/run", runH.ServeHTTPV1)

	r.Handle("/", http.RedirectHandler("/playground/", http.StatusFound))
	r.Mount("/playground/", http.StripPrefix("/playground/", playground.Handler()))

	r.Handle("/docs", http.RedirectHandler("/docs/", http.StatusFound))
	r.Get("/docs/", docs.UIHandler)
	r.Get("/docs/swagger.json", docs.JSONHandler)

	// WriteTimeout covers the worst-case job duration across all languages.
	writeTimeout := reg.MaxJobDuration(cfg.MaxTests)

	srv := &http.Server{
		Addr:        ":" + cfg.Port,
		Handler:     r,
		ReadTimeout: 30 * time.Second,
		// ReadHeaderTimeout drops slowloris connections that dribble request
		// headers to hold a connection open. Tighter than ReadTimeout, which
		// also covers the (legitimately large) body.
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Sweep orphaned jail directories periodically, not just at startup, so a
	// long-running server cannot accumulate leaked sandboxes (e.g. from a crash
	// between MkdirTemp and cleanup) until the next restart. SweepOrphans is
	// age-gated, so it never touches an in-flight job's workspace.
	go sweepOrphansLoop(ctx, cfg.JailDir, time.Duration(cfg.OrphanMaxAge)*time.Minute)

	go func() {
		slog.Info("server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")
	// Stop the probe cache background refresh goroutine cleanly.
	probes.Stop()
	// Set shutdown timeout to max job duration to allow in-flight jobs to complete.
	shutTimeout := reg.MaxJobDuration(cfg.MaxTests) + 5*time.Second
	shutCtx, cancel := context.WithTimeout(context.Background(), shutTimeout)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
	jobRunner.Close()
}

// sweepOrphansLoop runs SweepOrphans on a ticker until ctx is cancelled. The
// interval tracks maxAge (a leaked dir is removed within roughly one maxAge of
// going stale) but is floored at one minute so a small or zero OrphanMaxAge
// cannot spin the goroutine. The startup sweep already ran before this is
// called, so the first tick is a follow-up, not the first cleanup.
func sweepOrphansLoop(ctx context.Context, jailDir string, maxAge time.Duration) {
	interval := max(maxAge, time.Minute)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sandbox.SweepOrphans(jailDir, maxAge)
		}
	}
}
