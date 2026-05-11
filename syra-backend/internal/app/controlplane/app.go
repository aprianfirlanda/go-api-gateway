package controlplane

import (
	"log/slog"
	"net/http"

	"syra-backend/internal/config"
	cp "syra-backend/internal/controlplane"
)

type App struct {
	Server *http.Server
	Store  *cp.Store
}

func New(cfg config.Config, logger *slog.Logger) (*App, error) {
	store := cp.NewStore()
	router := cp.NewRouter(cp.RouterConfig{
		AdminToken: cfg.ControlPlaneAdminToken,
		Store:      store,
	})

	return &App{
		Server: &http.Server{
			Addr:         cfg.ControlPlaneAddr,
			Handler:      router,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
		},
		Store: store,
	}, nil
}
