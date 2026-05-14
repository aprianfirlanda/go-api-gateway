package controlplane

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"backend/internal/auth"
	"backend/internal/billing"
	"backend/internal/config"
	cp "backend/internal/controlplane"
	"backend/internal/health"
	"backend/internal/httpserver"
	"backend/internal/ports/output"
	"backend/internal/storage/postgres"
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
		AdminToken:         cfg.ControlPlaneAdminToken,
		AdminAuthenticator: buildAdminAuthenticator(cfg),
		Store:              store,
		UsageEvents:        usageEvents,
	})
	pingers := []output.DBPinger{}
	if pool != nil {
		pingers = append(pingers, postgres.NewPinger(pool))
	}
	healthHandler := httpserver.NewHealthHandler(health.NewService(health.NewMultiPinger(pingers...)))
	root := http.NewServeMux()
	root.HandleFunc("/healthz", healthHandler.Liveness)
	root.HandleFunc("/readyz", healthHandler.Readiness)
	root.Handle("/", router)

	return &App{
		Server: &http.Server{
			Addr:         cfg.ControlPlaneAddr,
			Handler:      root,
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

func buildAdminAuthenticator(cfg config.Config) cp.AdminAuthenticator {
	authenticator := cp.StaticAdminAuthenticator{BootstrapToken: cfg.ControlPlaneAdminToken}
	raw := strings.TrimSpace(cfg.ControlPlaneAdminAPIKeys)
	if raw == "" {
		return authenticator
	}
	records := strings.Split(raw, ",")
	for _, record := range records {
		parts := strings.Split(strings.TrimSpace(record), "|")
		if len(parts) < 3 {
			continue
		}
		prefix, secret, err := auth.ParseAPIKey(parts[0])
		if err != nil {
			continue
		}
		hash, err := auth.HashSecret(secret)
		if err != nil {
			continue
		}
		authenticator.APIKeys = append(authenticator.APIKeys, cp.AdminAPIKey{
			ActorID:    "admin_api_key_" + prefix,
			Role:       strings.TrimSpace(parts[1]),
			TenantID:   strings.TrimSpace(parts[2]),
			KeyPrefix:  prefix,
			SecretHash: hash,
		})
	}
	return authenticator
}
