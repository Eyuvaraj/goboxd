package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
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

	healthH := handler.NewHealthHandler(reg, probes, cfg, counters)
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
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: writeTimeout,
		IdleTimeout:  60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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
}
