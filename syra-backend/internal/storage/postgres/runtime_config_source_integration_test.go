package postgres

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"syra-backend/internal/auth"
	"syra-backend/internal/billing"
	"syra-backend/internal/controlplane"
	"syra-backend/internal/gateway/policy"
	"syra-backend/internal/gateway/route"
	"syra-backend/internal/gateway/runtimeconfig"
	"syra-backend/internal/gateway/upstream"
	"syra-backend/internal/health"
	"syra-backend/internal/httpserver"
	"syra-backend/internal/observability"
	"syra-backend/internal/protocol"
	restprotocol "syra-backend/internal/protocol/rest"
	"syra-backend/internal/transform"
	"syra-backend/pkg/ids"
)

func TestRuntimeConfigSyncReloadRoutesGatewayTraffic(t *testing.T) {
	ctx := context.Background()
	pool := newTestPostgresPool(t, ctx)
	repo := NewControlPlaneRepository(pool)
	now := time.Date(2026, 5, 13, 10, 0, 0, 0, time.UTC)

	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	t.Cleanup(upstreamServer.Close)

	tenantID, productID, upstreamID := seedTenantProductUpstream(t, ctx, repo, now, upstreamServer.URL)
	activeAPIKey := createCredential(t, ctx, repo, now, tenantID, auth.StatusActive)
	disabledAPIKey := createCredential(t, ctx, repo, now, tenantID, controlplane.StatusDisabled)

	routeActive := controlplane.Route{
		ID:               ids.New(),
		TenantID:         tenantID,
		APIProductID:     productID,
		Name:             "active-route",
		InboundProtocol:  restprotocol.Name,
		OutboundProtocol: restprotocol.Name,
		Host:             "api.local.test",
		Method:           http.MethodGet,
		Path:             "/accounts",
		UpstreamID:       upstreamID,
		Priority:         100,
		TimeoutMs:        1000,
		Status:           controlplane.StatusActive,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	require.NoError(t, repo.CreateRoute(ctx, routeActive))

	routeDisabled := routeActive
	routeDisabled.ID = ids.New()
	routeDisabled.Name = "disabled-route"
	routeDisabled.Path = "/disabled"
	routeDisabled.Status = controlplane.StatusDisabled
	require.NoError(t, repo.CreateRoute(ctx, routeDisabled))

	manager, router := newReloadManagerWithRouter(pool)
	require.NoError(t, manager.Reload(ctx))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://api.local.test/accounts", nil)
	req.Header.Set("Authorization", "ApiKey "+activeAPIKey)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"ok":true}`, rec.Body.String())

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "http://api.local.test/disabled", nil)
	req.Header.Set("Authorization", "ApiKey "+activeAPIKey)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "http://api.local.test/accounts", nil)
	req.Header.Set("Authorization", "ApiKey "+disabledAPIKey)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRuntimeConfigSyncRequiresPublishedTemplate(t *testing.T) {
	ctx := context.Background()
	pool := newTestPostgresPool(t, ctx)
	repo := NewControlPlaneRepository(pool)
	now := time.Date(2026, 5, 13, 10, 0, 0, 0, time.UTC)

	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	t.Cleanup(upstreamServer.Close)

	tenantID, productID, upstreamID := seedTenantProductUpstream(t, ctx, repo, now, upstreamServer.URL)
	createCredential(t, ctx, repo, now, tenantID, auth.StatusActive)

	template := controlplane.TransformationTemplate{
		ID:             ids.New(),
		TenantID:       tenantID,
		APIProductID:   productID,
		Name:           "draft-template",
		SourceProtocol: restprotocol.Name,
		TargetProtocol: restprotocol.Name,
		Version:        1,
		Status:         controlplane.StatusDraft,
		TemplateBody:   json.RawMessage(`{"request":{"fields":{"amount":"$.amount"}},"response":{"fields":{"ok":"$.ok"}}}`),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	require.NoError(t, repo.CreateTemplate(ctx, template))
	require.NoError(t, repo.CreateRoute(ctx, controlplane.Route{
		ID:                       ids.New(),
		TenantID:                 tenantID,
		APIProductID:             productID,
		Name:                     "route-with-template",
		InboundProtocol:          restprotocol.Name,
		OutboundProtocol:         restprotocol.Name,
		Host:                     "api.local.test",
		Method:                   http.MethodPost,
		Path:                     "/payments",
		UpstreamID:               upstreamID,
		TransformationTemplateID: template.ID,
		Priority:                 100,
		TimeoutMs:                1000,
		Status:                   controlplane.StatusActive,
		CreatedAt:                now,
		UpdatedAt:                now,
	}))

	manager, _ := newReloadManagerWithRouter(pool)
	require.Error(t, manager.Reload(ctx))

	publishedAt := now.Add(time.Minute)
	template.Status = "published"
	template.PublishedAt = &publishedAt
	template.UpdatedAt = publishedAt
	require.NoError(t, repo.UpdateTemplate(ctx, template))
	require.NoError(t, manager.Reload(ctx))
}

func TestRuntimeConfigSyncKeepsLastKnownGoodOnInvalidDatabaseState(t *testing.T) {
	ctx := context.Background()
	pool := newTestPostgresPool(t, ctx)
	repo := NewControlPlaneRepository(pool)
	now := time.Date(2026, 5, 13, 10, 0, 0, 0, time.UTC)

	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	t.Cleanup(upstreamServer.Close)

	tenantID, productID, upstreamID := seedTenantProductUpstream(t, ctx, repo, now, upstreamServer.URL)
	activeAPIKey := createCredential(t, ctx, repo, now, tenantID, auth.StatusActive)
	routeID := ids.New()
	require.NoError(t, repo.CreateRoute(ctx, controlplane.Route{
		ID:               routeID,
		TenantID:         tenantID,
		APIProductID:     productID,
		Name:             "route",
		InboundProtocol:  restprotocol.Name,
		OutboundProtocol: restprotocol.Name,
		Host:             "api.local.test",
		Method:           http.MethodGet,
		Path:             "/accounts",
		UpstreamID:       upstreamID,
		Priority:         100,
		TimeoutMs:        1000,
		Status:           controlplane.StatusActive,
		CreatedAt:        now,
		UpdatedAt:        now,
	}))

	manager, router := newReloadManagerWithRouter(pool)
	require.NoError(t, manager.Reload(ctx))
	loadedVersion := manager.Current().Version

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://api.local.test/accounts", nil)
	req.Header.Set("Authorization", "ApiKey "+activeAPIKey)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	upstreamItem, err := repo.GetUpstream(ctx, tenantID, upstreamID)
	require.NoError(t, err)
	upstreamItem.Status = controlplane.StatusDisabled
	upstreamItem.UpdatedAt = now.Add(time.Minute)
	require.NoError(t, repo.UpdateUpstream(ctx, upstreamItem))

	require.Error(t, manager.Reload(ctx))
	require.Equal(t, loadedVersion, manager.Current().Version)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "http://api.local.test/accounts", nil)
	req.Header.Set("Authorization", "ApiKey "+activeAPIKey)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestRuntimeConfigSyncLoadsPersistedPolicies(t *testing.T) {
	ctx := context.Background()
	pool := newTestPostgresPool(t, ctx)
	repo := NewControlPlaneRepository(pool)
	source := NewRuntimeConfigSource(pool)
	now := time.Date(2026, 5, 13, 10, 0, 0, 0, time.UTC)
	tenantID, productID, upstreamID := seedTenantProductUpstream(t, ctx, repo, now, "http://localhost:9999")
	rate := controlplane.RateLimitPolicy{
		ID: ids.New(), TenantID: tenantID, Name: "r1", Scope: "route", LimitCount: 2, WindowSeconds: 60, BurstCount: 0, Status: controlplane.StatusActive, CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, repo.CreateRateLimitPolicy(ctx, rate))
	quota := controlplane.QuotaPolicy{
		ID: ids.New(), TenantID: tenantID, Name: "q1", Scope: "route", Period: "monthly", QuotaCount: 10, ExceededBehavior: "block", Status: controlplane.StatusActive, CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, repo.CreateQuotaPolicy(ctx, quota))
	require.NoError(t, repo.CreateRoute(ctx, controlplane.Route{
		ID: ids.New(), TenantID: tenantID, APIProductID: productID, Name: "r", InboundProtocol: restprotocol.Name, OutboundProtocol: restprotocol.Name,
		Host: "api.local.test", Method: http.MethodGet, Path: "/x", UpstreamID: upstreamID, RateLimitPolicyID: rate.ID, QuotaPolicyID: quota.ID,
		Priority: 100, TimeoutMs: 1000, Status: controlplane.StatusActive, CreatedAt: now, UpdatedAt: now,
	}))
	snapshot, err := source.Load(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, snapshot.RateLimits)
	require.NotEmpty(t, snapshot.Quotas)
	require.Equal(t, rate.ID, snapshot.Routes[0].RateLimitPolicyID)
	require.Equal(t, quota.ID, snapshot.Routes[0].QuotaPolicyID)
}

func seedTenantProductUpstream(t *testing.T, ctx context.Context, repo *ControlPlaneRepository, now time.Time, upstreamBaseURL string) (string, string, string) {
	t.Helper()
	tenant := controlplane.Tenant{
		ID:        ids.New(),
		Name:      "Bank A",
		Slug:      "bank-a-" + ids.New()[0:8],
		Status:    controlplane.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, repo.CreateTenant(ctx, tenant))
	product := controlplane.APIProduct{
		ID:        ids.New(),
		TenantID:  tenant.ID,
		Name:      "Payments",
		Slug:      "payments-" + ids.New()[0:8],
		Status:    controlplane.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, repo.CreateAPIProduct(ctx, product))
	up := controlplane.Upstream{
		ID:        ids.New(),
		TenantID:  tenant.ID,
		Name:      "core-" + ids.New()[0:8],
		Protocol:  restprotocol.Name,
		Config:    json.RawMessage(`{"baseUrl":"` + upstreamBaseURL + `"}`),
		Status:    controlplane.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, repo.CreateUpstream(ctx, up))
	return tenant.ID, product.ID, up.ID
}

func createCredential(t *testing.T, ctx context.Context, repo *ControlPlaneRepository, now time.Time, tenantID string, status string) string {
	t.Helper()
	consumer := controlplane.Consumer{
		ID:        ids.New(),
		TenantID:  tenantID,
		Name:      "app",
		Slug:      "app-" + ids.New()[0:8],
		Status:    controlplane.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, repo.CreateConsumer(ctx, consumer))
	apiKey := "gw_live_" + ids.New()[0:12] + ".secret"
	prefix, secret, err := auth.ParseAPIKey(apiKey)
	require.NoError(t, err)
	secretHash, err := auth.HashSecret(secret)
	require.NoError(t, err)
	require.NoError(t, repo.CreateCredential(ctx, controlplane.Credential{
		ID:         ids.New(),
		TenantID:   tenantID,
		ConsumerID: consumer.ID,
		Type:       "api_key",
		KeyPrefix:  prefix,
		SecretHash: secretHash,
		Status:     status,
		CreatedAt:  now,
		UpdatedAt:  now,
	}))
	return apiKey
}

func newReloadManagerWithRouter(pool *pgxpool.Pool) (*runtimeconfig.Manager, http.Handler) {
	adapterRegistry := protocol.NewRegistry()
	restAdapter := restprotocol.NewAdapter(nil)
	_ = adapterRegistry.RegisterProtocol(restAdapter)
	_ = adapterRegistry.RegisterUpstream(restAdapter)
	routeRegistry := route.NewInMemoryRegistry()
	upstreamStore := upstream.NewInMemoryStore()
	credentialStore := auth.NewInMemoryCredentialStore()
	templateStore := transform.NewInMemoryStore()
	manager := runtimeconfig.NewManager(NewRuntimeConfigSource(pool), runtimeconfig.Applier{
		Routes:      routeRegistry,
		Upstreams:   upstreamStore,
		Credentials: credentialStore,
		Templates:   templateStore,
	}, slog.Default())
	router := httpserver.NewRouter(httpserver.Dependencies{
		Logger:          slog.Default(),
		HealthService:   health.NewService(nil),
		CredentialStore: credentialStore,
		RouteRegistry:   routeRegistry,
		UpstreamStore:   upstreamStore,
		AdapterRegistry: adapterRegistry,
		TemplateStore:   templateStore,
		TransformEngine: transform.NewEngine(),
		PolicyPipeline:  policy.NewPipeline(),
		UsageEventStore: billing.NewInMemoryUsageEventStore(),
		Metrics:         observability.NewMetrics(),
		BodyLimit:       1 << 20,
	})
	return manager, router
}
