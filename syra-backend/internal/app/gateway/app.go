package gateway

import (
	"log/slog"
	"net/http"

	"syra-backend/internal/auth"
	"syra-backend/internal/config"
	"syra-backend/internal/gateway/route"
	"syra-backend/internal/health"
	"syra-backend/internal/httpserver"
)

type App struct {
	Server *http.Server
}

func New(cfg config.Config, logger *slog.Logger) (*App, error) {
	healthService := health.NewService(nil)
	router := httpserver.NewRouter(httpserver.Dependencies{
		Logger:          logger,
		HealthService:   healthService,
		CredentialStore: auth.NewInMemoryCredentialStore(),
		RouteRegistry:   route.NewInMemoryRegistry(),
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
