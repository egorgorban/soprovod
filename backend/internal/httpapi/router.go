// Package httpapi wires up the HTTP server: routing, middleware and JSON
// helpers. Route handlers for /api/* are added by the pipeline-integration
// agent (see Router's TODO below); this package only sets up the skeleton so
// that wiring can happen without churn later.
package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Deps holds everything a Router needs to build request handlers.
type Deps struct {
	Logger *slog.Logger
	// Pipeline runs the fetch->filter->generate flow for POST /api/applications.
	Pipeline Pipeline
	// Repo backs the list/get/patch application endpoints.
	Repo ApplicationRepo
	// StaticDir, if non-empty, is served for any path not matched by an API
	// route (SPA fallback to index.html).
	StaticDir string
}

// requestTimeout bounds every request. POST /api/applications does a
// synchronous hh.ru fetch + LLM call, which can take up to ~30-40s, so this
// must be comfortably above that (PLAN.md budgets ~60s for the whole
// pipeline call).
const requestTimeout = 90 * time.Second

// NewRouter builds the chi router: middleware, /healthz, and the /api routes.
func NewRouter(deps Deps) http.Handler {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(requestLogger(logger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(requestTimeout))

	r.Get("/healthz", handleHealthz)

	if deps.Pipeline != nil && deps.Repo != nil {
		r.Route("/api/applications", func(r chi.Router) {
			r.Post("/", handleCreateApplication(deps.Pipeline))
			r.Get("/", handleListApplications(deps.Repo))
			r.Get("/{id}", handleGetApplication(deps.Repo))
			r.Patch("/{id}", handleUpdateApplication(deps.Repo))
		})
	}

	if deps.StaticDir != "" {
		mountStatic(r, deps.StaticDir)
	}

	return r
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// requestLogger logs each request's method, path, status and duration via slog.
func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			logger.Info("http_request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", middleware.GetReqID(r.Context()),
			)
		})
	}
}
