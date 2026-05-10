package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"syra-backend/internal/auth"
	"syra-backend/internal/gateway/route"
	"syra-backend/internal/ports/input"
)

type Dependencies struct {
	Logger          *slog.Logger
	HealthService   input.HealthService
	CredentialStore auth.CredentialStore
	RouteRegistry   route.Registry
	BodyLimit       int64
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

	if deps.CredentialStore != nil && deps.RouteRegistry != nil {
		gatewayHandler := NewGatewayHandler(deps.RouteRegistry)
		r.Group(func(protected chi.Router) {
			protected.Use(APIKeyAuth(deps.CredentialStore))
			protected.Handle("/*", gatewayHandler)
		})
	}

	return r
}
