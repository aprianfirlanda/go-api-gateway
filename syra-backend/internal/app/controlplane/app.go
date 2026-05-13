package controlplane

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"syra-backend/internal/billing"
	"syra-backend/internal/config"
	cp "syra-backend/internal/controlplane"
	"syra-backend/internal/storage/postgres"
)

type App struct {
	Server      *http.Server
	Store       cp.Repository
	UsageEvents billing.UsageEventStore
	pool        *pgxpool.Pool
}

func New(cfg config.Config, logger *slog.Logger) (*App, error) {
	store, usageEvents, pool, err := openRepository(context.Background(), cfg)
	if err != nil {
		return nil, err
	}
	router := cp.NewRouter(cp.RouterConfig{
		AdminToken:  cfg.ControlPlaneAdminToken,
		Store:       store,
		UsageEvents: usageEvents,
	})

	return &App{
		Server: &http.Server{
			Addr:         cfg.ControlPlaneAddr,
			Handler:      router,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
		},
		Store:       store,
		UsageEvents: usageEvents,
		pool:        pool,
	}, nil
}

func (a *App) Close() {
	if a != nil && a.pool != nil {
		a.pool.Close()
	}
}

func openRepository(ctx context.Context, cfg config.Config) (cp.Repository, billing.UsageEventStore, *pgxpool.Pool, error) {
	if cfg.DatabaseURL == "" {
		return cp.NewStore(), billing.NewInMemoryUsageEventStore(), nil, nil
	}
	if err := postgres.Migrate(ctx, cfg.DatabaseURL, "migrations"); err != nil {
		return nil, nil, nil, fmt.Errorf("migrate control plane database: %w", err)
	}
	pool, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, nil, err
	}
	return postgres.NewControlPlaneRepository(pool), postgres.NewUsageEventStore(pool), pool, nil
}
