package gateway

import (
	"log/slog"
	"net/http"

	"syra-backend/internal/auth"
	"syra-backend/internal/config"
	"syra-backend/internal/gateway/route"
	"syra-backend/internal/gateway/upstream"
	"syra-backend/internal/health"
	"syra-backend/internal/httpserver"
	"syra-backend/internal/protocol"
	"syra-backend/internal/protocol/iso8583"
	restprotocol "syra-backend/internal/protocol/rest"
	"syra-backend/internal/transform"
)

type App struct {
	Server *http.Server
}

func New(cfg config.Config, logger *slog.Logger) (*App, error) {
	healthService := health.NewService(nil)
	adapterRegistry := protocol.NewRegistry()
	restAdapter := restprotocol.NewAdapter(nil)
	if err := adapterRegistry.RegisterProtocol(restAdapter); err != nil {
		return nil, err
	}
	if err := adapterRegistry.RegisterUpstream(restAdapter); err != nil {
		return nil, err
	}
	isoAdapter := iso8583.NewAdapter(nil, iso8583.NewInMemoryProfileStore(), nil)
	if err := adapterRegistry.RegisterUpstream(isoAdapter); err != nil {
		return nil, err
	}

	router := httpserver.NewRouter(httpserver.Dependencies{
		Logger:          logger,
		HealthService:   healthService,
		CredentialStore: auth.NewInMemoryCredentialStore(),
		RouteRegistry:   route.NewInMemoryRegistry(),
		UpstreamStore:   upstream.NewInMemoryStore(),
		AdapterRegistry: adapterRegistry,
		TemplateStore:   transform.NewInMemoryStore(),
		TransformEngine: transform.NewEngine(),
		BodyLimit:       cfg.RequestBodyLimit,
	})

	return &App{
		Server: &http.Server{
			Addr:         cfg.GatewayAddr,
			Handler:      router,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
		},
	}, nil
}
