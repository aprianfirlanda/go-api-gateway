package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"syra-backend/internal/ports/input"
)

type Dependencies struct {
	Logger        *slog.Logger
	HealthService input.HealthService
	BodyLimit     int64
}

func NewRouter(deps Dependencies) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(ResponseRequestID)
	r.Use(middleware.RealIP)
	r.Use(Recoverer(deps.Logger))
	r.Use(RequestLogger(deps.Logger))
	r.Use(MaxBodyBytes(deps.BodyLimit))

	healthHandler := NewHealthHandler(deps.HealthService)
	r.Get("/healthz", healthHandler.Liveness)
	r.Get("/readyz", healthHandler.Readiness)

	return r
}
