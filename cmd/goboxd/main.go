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
	httpSwagger "github.com/swaggo/http-swagger/v2"

	_ "github.com/thesouldev/goboxd/docs" // generated swagger spec
	"github.com/thesouldev/goboxd/internal/config"
	"github.com/thesouldev/goboxd/internal/handler"
	"github.com/thesouldev/goboxd/internal/registry"
	"github.com/thesouldev/goboxd/internal/runner"
	"github.com/thesouldev/goboxd/internal/sandbox"
	"github.com/thesouldev/goboxd/internal/stats"
)

func main() {
	// Structured JSON logging from the start.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := config.Load()

	// Sweep orphaned jail directories from any previous unclean shutdown.
	sandbox.SweepOrphans(cfg.JailDir, time.Duration(cfg.OrphanMaxAge)*time.Minute)

	// Load and validate language registry — fail loudly at startup.
	reg, err := registry.Load(cfg.LanguageFile)
	if err != nil {
		slog.Error("failed to load language registry", "error", err)
		os.Exit(1)
	}
	slog.Info("language registry loaded", "count", reg.Len())

	// ProbeCache runs the initial probe synchronously (fails loudly if nsjail or
	// any language binary is missing) and refreshes in the background every 30s.
	probes := registry.NewProbeCache(reg, cfg.NsjailPath)

	counters := &stats.Counters{}
	jobRunner := runner.New(cfg.MaxConcurrentJobs, reg, cfg, counters)

	healthH := handler.NewHealthHandler(reg, probes, cfg, counters)
	runH := handler.NewRunHandler(jobRunner, reg, cfg)

	// Maximum valid body: source + (tests × stdin) + (tests × expected_stdout) + JSON framing.
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

	// Swagger UI — served at /docs/  (e.g. http://localhost:8080/docs/index.html)
	r.Get("/docs/*", httpSwagger.WrapHandler)

	// WriteTimeout must cover the worst-case job: longest build + MaxTests×longest run + buffer.
	// Derived from the loaded registry so it stays correct when languages are added.
	writeTimeout := reg.MaxJobDuration(cfg.MaxTests)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: writeTimeout,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown on SIGINT/SIGTERM.
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
	shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
}
