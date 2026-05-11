package gateway

import (
	"context"
	"log/slog"
	"net/http"

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
	"syra-backend/internal/protocol"
	"syra-backend/internal/protocol/iso8583"
	restprotocol "syra-backend/internal/protocol/rest"
	"syra-backend/internal/protocol/soapxml"
	"syra-backend/internal/transform"
)

type App struct {
	Server       *http.Server
	ConfigReload *runtimeconfig.Manager
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
	reloadManager := runtimeconfig.NewManager(runtimeconfig.StaticSource{
		Snapshot: runtimeconfig.Snapshot{Version: 1, Status: "active"},
	}, runtimeconfig.Applier{
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
	}, nil
}
