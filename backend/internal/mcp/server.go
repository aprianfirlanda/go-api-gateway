package mcp

import (
	"encoding/json"
	"net/http"
	"time"

	"backend/internal/billing"
	"backend/internal/controlplane"

	"github.com/go-chi/chi/v5"
)

type server struct {
	store controlplane.Repository
	usage billing.UsageEventStore
	now   func() time.Time
}

func NewHandler(cfg Config) http.Handler {
	if cfg.Store == nil {
		cfg.Store = controlplane.NewStore()
	}
	now := cfg.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	s := &server{store: cfg.Store, usage: cfg.Usage, now: now}

	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, HealthResponse{
			Service:   "mcp-server",
			Status:    "ok",
			Timestamp: now(),
		})
	})

	r.Group(func(protected chi.Router) {
		protected.Use(AuthMiddleware(cfg.AuthToken))
		protected.Get("/mcp", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]any{
				"service":       "mcp-server",
				"authenticated": IsAuthenticated(r.Context()),
				"status":        "ready",
			})
		})
		protected.Get("/mcp/tools/list-tenants", s.listTenants)
		protected.Post("/mcp/tools/create-tenant", s.createTenant)
		protected.Get("/mcp/tools/list-routes", s.listRoutes)
		protected.Post("/mcp/tools/create-route", s.createRoute)
		protected.Post("/mcp/tools/rotate-api-key", s.rotateAPIKey)
		protected.Get("/mcp/tools/usage-report", s.usageReport)
		protected.Get("/mcp/tools/audit-logs", s.auditLogs)
	})

	return r
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, ErrorResponse{Code: code, Message: message})
}
