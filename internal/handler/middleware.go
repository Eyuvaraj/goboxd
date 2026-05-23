package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/thesouldev/goboxd/internal/logctx"
)

// BodyLimit wraps the request body with http.MaxBytesReader.
// Requests exceeding maxBytes receive a 413 before any handler runs.
func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// StructuredLogger logs one JSON line per request via slog.
// For POST /run it also includes language, execution status, and test counts
// written into the request context by the run handler via logctx.Set.
func StructuredLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()

		next.ServeHTTP(ww, r)

		args := []any{
			"request_id", middleware.GetReqID(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"http_status", ww.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"bytes", ww.BytesWritten(),
		}

		if f := logctx.Get(r.Context()); f.Language != "" {
			args = append(args,
				"language", f.Language,
				"exec_status", f.ExecStatus,
				"build_duration_ms", f.BuildDurationMs,
				"tests_total", f.TestsTotal,
				"tests_accepted", f.TestsAccepted,
			)
		}

		slog.Info("request", args...)
	})
}
