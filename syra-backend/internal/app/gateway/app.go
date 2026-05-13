package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"syra-backend/internal/auth"
	"syra-backend/internal/billing"
	"syra-backend/internal/config"
	"syra-backend/internal/gateway/policy"
	"syra-backend/internal/gateway/route"
	"syra-backend/internal/gateway/runtimeconfig"
	"syra-backend/internal/gateway/upstream"
	"syra-backend/internal/health"
	"syra-backend/internal/httpserver"
	"syra-backend/internal/observability"
	"syra-backend/internal/ports/output"
	"syra-backend/internal/protocol"
	"syra-backend/internal/protocol/iso8583"
	restprotocol "syra-backend/internal/protocol/rest"
	"syra-backend/internal/protocol/soapxml"
	"syra-backend/internal/runtime/state"
	"syra-backend/internal/storage/postgres"
	storageredis "syra-backend/internal/storage/redis"
	"syra-backend/internal/transform"
)

type App struct {
	Server       *http.Server
	ConfigReload *runtimeconfig.Manager
	pool         *pgxpool.Pool
	redisClient  *goredis.Client
	RuntimeState state.Store
}

func New(cfg config.Config, logger *slog.Logger) (*App, error) {
	adapterRegistry := protocol.NewRegistry()
	restAdapter := restprotocol.NewAdapter(nil)
	if err := adapterRegistry.RegisterProtocol(restAdapter); err != nil {
		return nil, err
	}
	if err := adapterRegistry.RegisterUpstream(restAdapter); err != nil {
		return nil, err
	}
	profileStore := iso8583.NewInMemoryProfileStore()
	isoAdapter := iso8583.NewAdapter(nil, profileStore, nil)
	if err := adapterRegistry.RegisterUpstream(isoAdapter); err != nil {
		return nil, err
	}
	soapAdapter := soapxml.NewAdapter(nil)
	if err := adapterRegistry.RegisterUpstream(soapAdapter); err != nil {
		return nil, err
	}
	credentialStore := auth.NewInMemoryCredentialStore()
	routeRegistry := route.NewInMemoryRegistry()
	upstreamStore := upstream.NewInMemoryStore()
	templateStore := transform.NewInMemoryStore()
	usageEvents := billing.NewInMemoryUsageEventStore()
	metrics := observability.NewMetrics()
	source := runtimeconfig.SnapshotSource(runtimeconfig.StaticSource{
		Snapshot: runtimeconfig.Snapshot{Version: 1, Status: "active"},
	})
	var pool *pgxpool.Pool
	pingers := []output.DBPinger{}
	if cfg.DatabaseURL != "" {
		if err := postgres.Migrate(context.Background(), cfg.DatabaseURL, "migrations"); err != nil {
			return nil, fmt.Errorf("migrate gateway database: %w", err)
		}
		var err error
		pool, err = postgres.Open(context.Background(), cfg.DatabaseURL)
		if err != nil {
			return nil, err
		}
		source = postgres.NewRuntimeConfigSource(pool)
		pingers = append(pingers, postgres.NewPinger(pool))
	}
	var redisClient *goredis.Client
	var runtimeStateStore state.Store
	namespacer := state.Namespacer{Environment: cfg.RuntimeStateEnv, Version: cfg.RuntimeStateVersion}
	if strings.EqualFold(cfg.RuntimeStateBackend, "redis") {
		client, err := storageredis.Open(context.Background(), storageredis.Config{
			Addr:    cfg.RedisAddr,
			Timeout: cfg.RedisTimeout,
		}, logger)
		if err != nil {
			return nil, err
		}
		redisClient = client
		runtimeStateStore = state.NewRedisStore(redisClient, namespacer, metrics, logger)
		pingers = append(pingers, storageredis.NewPinger(redisClient))
	} else {
		runtimeStateStore = state.NewInMemoryStore(namespacer)
	}
	healthService := health.NewService(health.NewMultiPinger(pingers...))
	reloadManager := runtimeconfig.NewManager(source, runtimeconfig.Applier{
		Routes:      routeRegistry,
		Upstreams:   upstreamStore,
		Credentials: credentialStore,
		Templates:   templateStore,
		Profiles:    profileStore,
	}, logger)
	_ = reloadManager.Reload(context.Background())

	router := httpserver.NewRouter(httpserver.Dependencies{
		Logger:          logger,
		HealthService:   healthService,
		CredentialStore: credentialStore,
		RouteRegistry:   routeRegistry,
		UpstreamStore:   upstreamStore,
		AdapterRegistry: adapterRegistry,
		TemplateStore:   templateStore,
		TransformEngine: transform.NewEngine(),
		PolicyPipeline:  policy.NewPipeline(),
		UsageEventStore: usageEvents,
		Metrics:         metrics,
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
		ConfigReload: reloadManager,
		pool:         pool,
		redisClient:  redisClient,
		RuntimeState: runtimeStateStore,
	}, nil
}

func (a *App) Close() {
	if a != nil && a.pool != nil {
		a.pool.Close()
	}
	if a != nil && a.redisClient != nil {
		_ = a.redisClient.Close()
	}
}
