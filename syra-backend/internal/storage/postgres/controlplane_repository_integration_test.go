package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"syra-backend/internal/billing"
	"syra-backend/internal/controlplane"
	"syra-backend/pkg/ids"
)

func TestControlPlaneRepositoryIntegration(t *testing.T) {
	ctx := context.Background()
	pool := newTestPostgresPool(t, ctx)
	repo := NewControlPlaneRepository(pool)
	now := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)

	tenantA := controlplane.Tenant{ID: ids.New(), Name: "Bank A", Slug: "bank-a", Status: controlplane.StatusActive, Metadata: map[string]any{"industry": "banking"}, CreatedAt: now, UpdatedAt: now}
	tenantB := controlplane.Tenant{ID: ids.New(), Name: "Bank B", Slug: "bank-b", Status: controlplane.StatusActive, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, repo.CreateTenant(ctx, tenantA))
	require.NoError(t, repo.CreateTenant(ctx, tenantB))

	plan := billing.BillingPlan{
		ID:               ids.New(),
		Name:             "Enterprise",
		Slug:             "enterprise",
		MonthlyFee:       5000,
		IncludedRequests: 100,
		OveragePrice:     0.1,
		Currency:         "USD",
		Status:           billing.PlanStatusActive,
	}
	require.NoError(t, repo.CreateBillingPlan(ctx, plan))
	loadedPlan, err := repo.GetBillingPlan(ctx, plan.ID)
	require.NoError(t, err)
	require.Equal(t, plan.Slug, loadedPlan.Slug)

	product := controlplane.APIProduct{ID: ids.New(), TenantID: tenantA.ID, Name: "Card Authorization", Slug: "card-authorization", Status: controlplane.StatusActive, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, repo.CreateAPIProduct(ctx, product))
	otherProduct := controlplane.APIProduct{ID: ids.New(), TenantID: tenantB.ID, Name: "Accounts", Slug: "accounts", Status: controlplane.StatusActive, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, repo.CreateAPIProduct(ctx, otherProduct))

	upstream := controlplane.Upstream{ID: ids.New(), TenantID: tenantA.ID, Name: "switch-primary", Protocol: "iso8583", Config: json.RawMessage(`{"host":"10.10.10.20","port":5000}`), Status: controlplane.StatusActive, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, repo.CreateUpstream(ctx, upstream))

	route := controlplane.Route{
		ID:               ids.New(),
		TenantID:         tenantA.ID,
		APIProductID:     product.ID,
		Name:             "Purchase Authorization",
		InboundProtocol:  "rest",
		OutboundProtocol: "iso8583",
		Host:             "api.gateway.example.com",
		Method:           "POST",
		Path:             "/cards/authorization",
		UpstreamID:       upstream.ID,
		Priority:         100,
		TimeoutMs:        5000,
		Status:           controlplane.StatusDraft,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	require.NoError(t, repo.CreateRoute(ctx, route))

	tenantAProducts, err := repo.ListAPIProducts(ctx, tenantA.ID)
	require.NoError(t, err)
	require.Len(t, tenantAProducts, 1)
	require.Equal(t, product.ID, tenantAProducts[0].ID)

	_, err = repo.GetAPIProduct(ctx, tenantB.ID, product.ID)
	require.ErrorIs(t, err, controlplane.ErrNotFound)

	route.Status = controlplane.StatusActive
	route.UpdatedAt = now.Add(time.Minute)
	require.NoError(t, repo.UpdateRoute(ctx, route))
	loadedRoute, err := repo.GetRoute(ctx, tenantA.ID, route.ID)
	require.NoError(t, err)
	require.Equal(t, controlplane.StatusActive, loadedRoute.Status)
	require.Equal(t, "/cards/authorization", loadedRoute.Path)

	consumer := controlplane.Consumer{ID: ids.New(), TenantID: tenantA.ID, Name: "Mobile App", Slug: "mobile-app", Status: controlplane.StatusActive, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, repo.CreateConsumer(ctx, consumer))
	credential := controlplane.Credential{ID: ids.New(), TenantID: tenantA.ID, ConsumerID: consumer.ID, Type: "api_key", KeyPrefix: "gw_live_test", SecretHash: "$argon2id$hash", Scopes: []string{"api:card-authorization:invoke"}, Status: controlplane.StatusActive, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, repo.CreateCredential(ctx, credential))
	loadedCredential, err := repo.GetCredential(ctx, tenantA.ID, credential.ID)
	require.NoError(t, err)
	require.Equal(t, credential.SecretHash, loadedCredential.SecretHash)
	require.Equal(t, credential.Scopes, loadedCredential.Scopes)
	authCredential, err := repo.FindByPrefix(ctx, credential.KeyPrefix)
	require.NoError(t, err)
	require.Equal(t, credential.TenantID, authCredential.TenantID)
	require.Equal(t, credential.ConsumerID, authCredential.ConsumerID)
	require.Equal(t, credential.SecretHash, authCredential.SecretHash)

	template := controlplane.TransformationTemplate{ID: ids.New(), TenantID: tenantA.ID, APIProductID: product.ID, Name: "card-auth", SourceProtocol: "rest", TargetProtocol: "iso8583", TemplateBody: json.RawMessage(`{"request":{"fields":{"2":"$.fields.pan"}}}`), Version: 1, Status: controlplane.StatusDraft, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, repo.CreateTemplate(ctx, template))
	template.Status = "published"
	publishedAt := now.Add(time.Minute)
	template.PublishedAt = &publishedAt
	require.NoError(t, repo.UpdateTemplate(ctx, template))
	loadedTemplate, err := repo.GetTemplate(ctx, tenantA.ID, template.ID)
	require.NoError(t, err)
	require.Equal(t, "published", loadedTemplate.Status)
	require.JSONEq(t, string(template.TemplateBody), string(loadedTemplate.TemplateBody))

	audit := controlplane.AuditEvent{ID: ids.New(), ActorID: "platform_admin", TenantID: tenantA.ID, Action: "route.publish", Resource: "route", ResourceID: route.ID, OccurredAt: now}
	require.NoError(t, repo.AppendAudit(ctx, audit))
	audits, err := repo.ListAuditEvents(ctx)
	require.NoError(t, err)
	require.Len(t, audits, 1)
	require.Equal(t, "route.publish", audits[0].Action)
}

func TestPostgresUsageEventStoreIntegration(t *testing.T) {
	ctx := context.Background()
	pool := newTestPostgresPool(t, ctx)
	repo := NewControlPlaneRepository(pool)
	usageStore := NewUsageEventStore(pool)
	now := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)

	tenant := controlplane.Tenant{ID: ids.New(), Name: "Bank A", Slug: "bank-a", Status: controlplane.StatusActive, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, repo.CreateTenant(ctx, tenant))
	product := controlplane.APIProduct{ID: ids.New(), TenantID: tenant.ID, Name: "Card Authorization", Slug: "card-authorization", Status: controlplane.StatusActive, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, repo.CreateAPIProduct(ctx, product))
	upstream := controlplane.Upstream{ID: ids.New(), TenantID: tenant.ID, Name: "core", Protocol: "rest", Config: json.RawMessage(`{"baseUrl":"https://core.example.test"}`), Status: controlplane.StatusActive, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, repo.CreateUpstream(ctx, upstream))
	route := controlplane.Route{ID: ids.New(), TenantID: tenant.ID, APIProductID: product.ID, Name: "Accounts", InboundProtocol: "rest", OutboundProtocol: "rest", Host: "api.example.test", Method: "GET", Path: "/accounts", UpstreamID: upstream.ID, Priority: 100, TimeoutMs: 5000, Status: controlplane.StatusActive, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, repo.CreateRoute(ctx, route))

	event := billing.UsageEvent{
		EventID:        "evt_test_1",
		TenantID:       tenant.ID,
		APIProductID:   product.ID,
		RouteID:        route.ID,
		SourceProtocol: "rest",
		TargetProtocol: "rest",
		Status:         billing.StatusSuccess,
		HTTPStatus:     200,
		UpstreamStatus: "200",
		LatencyMs:      42,
		Billable:       true,
		OccurredAt:     now,
	}
	require.NoError(t, usageStore.Save(ctx, event))

	events, err := usageStore.List(ctx, billing.UsageEventFilter{TenantID: tenant.ID})
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, event.EventID, events[0].EventID)
	require.Equal(t, event.RouteID, events[0].RouteID)
	require.True(t, events[0].Billable)

	page, err := usageStore.ListPage(ctx, billing.UsageEventFilter{TenantID: tenant.ID}, 1, "")
	require.NoError(t, err)
	require.Len(t, page.Data, 1)
	require.Equal(t, event.EventID, page.Data[0].EventID)

	summary := billing.BillingSummary{
		TenantID:         tenant.ID,
		BillingPeriod:    "2026-05",
		PlanID:           "",
		TotalRequests:    1,
		BillableRequests: 1,
		IncludedRequests: 0,
		OverageRequests:  1,
		FailedRequests:   0,
		RejectedRequests: 0,
		TimeoutRequests:  0,
		MonthlyFee:       0,
		OverageAmount:    0.1,
		EstimatedAmount:  0.1,
		Currency:         "USD",
		Status:           "draft",
		CalculatedAt:     now,
	}
	require.NoError(t, repo.UpsertBillingSummary(ctx, summary))
	loadedSummary, err := repo.GetBillingSummary(ctx, tenant.ID, "2026-05")
	require.NoError(t, err)
	require.Equal(t, int64(1), loadedSummary.TotalRequests)
}

func newTestPostgresPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Skipf("postgres testcontainer unavailable: %v", recovered)
		}
	}()

	container, err := tcpostgres.Run(
		ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("syra"),
		tcpostgres.WithUsername("syra"),
		tcpostgres.WithPassword("secret"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp")),
	)
	if err != nil {
		t.Skipf("postgres testcontainer unavailable: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil && !errors.Is(err, context.Canceled) {
			t.Logf("terminate postgres container: %v", err)
		}
	})

	databaseURL, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, Migrate(ctx, databaseURL, filepath.Join("..", "..", "..", "migrations")))

	pool, err := Open(ctx, databaseURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}
