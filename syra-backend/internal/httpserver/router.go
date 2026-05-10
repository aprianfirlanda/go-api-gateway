package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"syra-backend/internal/auth"
	"syra-backend/internal/gateway/route"
	"syra-backend/internal/gateway/upstream"
	"syra-backend/internal/ports/input"
	"syra-backend/internal/protocol"
	restprotocol "syra-backend/internal/protocol/rest"
	"syra-backend/internal/transform"
)

type Dependencies struct {
	Logger          *slog.Logger
	HealthService   input.HealthService
	CredentialStore auth.CredentialStore
	RouteRegistry   route.Registry
	UpstreamStore   upstream.Store
	AdapterRegistry *protocol.Registry
	TemplateStore   transform.Store
	TransformEngine *transform.Engine
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

	if deps.CredentialStore != nil && deps.RouteRegistry != nil && deps.UpstreamStore != nil {
		adapterRegistry := deps.AdapterRegistry
		if adapterRegistry == nil {
			adapterRegistry = protocol.NewRegistry()
			restAdapter := restprotocol.NewAdapter(nil)
			_ = adapterRegistry.RegisterProtocol(restAdapter)
			_ = adapterRegistry.RegisterUpstream(restAdapter)
		}
		transformEngine := deps.TransformEngine
		if transformEngine == nil {
			transformEngine = transform.NewEngine()
		}
		gatewayHandler := NewGatewayHandler(deps.RouteRegistry, deps.UpstreamStore, adapterRegistry, deps.TemplateStore, transformEngine)
		r.Group(func(protected chi.Router) {
			protected.Use(APIKeyAuth(deps.CredentialStore))
			protected.Handle("/*", gatewayHandler)
		})
	}

	return r
}
