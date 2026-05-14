package mcp

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"backend/internal/auth"
	"backend/internal/billing"
	"backend/internal/controlplane"

	"github.com/stretchr/testify/require"
)

func TestNewHandler_Healthz(t *testing.T) {
	handler := NewHandler(Config{
		AuthToken: "test-token",
		Now: func() time.Time {
			return time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"service":"mcp-server"`)
	require.Contains(t, rec.Body.String(), `"status":"ok"`)
}

func TestNewHandler_AuthGuard(t *testing.T) {
	handler := NewHandler(Config{AuthToken: "test-token"})

	t.Run("rejects missing token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
		require.Contains(t, rec.Body.String(), `"code":"unauthorized"`)
	})

	t.Run("accepts valid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
		req.Header.Set("X-MCP-Token", "test-token")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), `"authenticated":true`)
	})
}

func TestTools_CreateAndListTenant(t *testing.T) {
	store := controlplane.NewStore()
	handler := NewHandler(Config{
		AuthToken: "test-token",
		Store:     store,
		Now: func() time.Time {
			return time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
		},
	})

	createReq := httptest.NewRequest(http.MethodPost, "/mcp/tools/create-tenant", bytes.NewBufferString(`{"name":"Tenant A","slug":"tenant-a"}`))
	createReq.Header.Set("X-MCP-Token", "test-token")
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusCreated, createRec.Code)
	require.Contains(t, createRec.Body.String(), `"tool":"createTenant"`)
	require.Contains(t, createRec.Body.String(), `"name":"Tenant A"`)

	listReq := httptest.NewRequest(http.MethodGet, "/mcp/tools/list-tenants", nil)
	listReq.Header.Set("X-MCP-Token", "test-token")
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	require.Equal(t, http.StatusOK, listRec.Code)
	require.Contains(t, listRec.Body.String(), `"tool":"listTenants"`)
	require.Contains(t, listRec.Body.String(), `"slug":"tenant-a"`)
}

func TestTools_CreateAndListRoutes(t *testing.T) {
	store := controlplane.NewStore()
	now := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	err := store.CreateTenant(t.Context(), controlplane.Tenant{
		ID:        "tenant_1",
		Name:      "Tenant 1",
		Slug:      "tenant-1",
		Status:    controlplane.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	})
	require.NoError(t, err)

	handler := NewHandler(Config{
		AuthToken: "test-token",
		Store:     store,
		Now:       func() time.Time { return now },
	})

	createReq := httptest.NewRequest(http.MethodPost, "/mcp/tools/create-route", bytes.NewBufferString(`{
		"tenantId":"tenant_1",
		"apiProductId":"product_1",
		"name":"Route A",
		"inboundProtocol":"rest",
		"outboundProtocol":"rest",
		"host":"api.local",
		"method":"GET",
		"path":"/v1/status",
		"upstreamId":"upstream_1"
	}`))
	createReq.Header.Set("X-MCP-Token", "test-token")
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusCreated, createRec.Code)
	require.Contains(t, createRec.Body.String(), `"tool":"createRoute"`)
	require.Contains(t, createRec.Body.String(), `"name":"Route A"`)

	listReq := httptest.NewRequest(http.MethodGet, "/mcp/tools/list-routes?tenantId=tenant_1", nil)
	listReq.Header.Set("X-MCP-Token", "test-token")
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	require.Equal(t, http.StatusOK, listRec.Code)
	require.Contains(t, listRec.Body.String(), `"tool":"listRoutes"`)
	require.Contains(t, listRec.Body.String(), `"path":"/v1/status"`)
}

func TestTools_RotateAPIKey(t *testing.T) {
	store := controlplane.NewStore()
	now := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	err := store.CreateTenant(t.Context(), controlplane.Tenant{
		ID:        "tenant_1",
		Name:      "Tenant 1",
		Slug:      "tenant-1",
		Status:    controlplane.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	})
	require.NoError(t, err)
	secretHash, err := auth.HashSecret("old-secret")
	require.NoError(t, err)
	err = store.CreateCredential(t.Context(), controlplane.Credential{
		ID:         "cred_1",
		TenantID:   "tenant_1",
		ConsumerID: "consumer_1",
		Type:       "api_key",
		KeyPrefix:  "gw_live_old",
		SecretHash: secretHash,
		Status:     controlplane.StatusActive,
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	require.NoError(t, err)

	handler := NewHandler(Config{AuthToken: "test-token", Store: store, Now: func() time.Time { return now }})
	req := httptest.NewRequest(http.MethodPost, "/mcp/tools/rotate-api-key", bytes.NewBufferString(`{"tenantId":"tenant_1","credentialId":"cred_1"}`))
	req.Header.Set("X-MCP-Token", "test-token")
	req.Header.Set("X-MCP-Role", "tenant_admin")
	req.Header.Set("X-MCP-Tenant-ID", "tenant_1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"tool":"rotateApiKey"`)
	require.Contains(t, rec.Body.String(), `"apiKey":"gw_live_`)
}

func TestTools_RBACEnforcedForTenantAccess(t *testing.T) {
	store := controlplane.NewStore()
	now := time.Now().UTC()
	err := store.CreateTenant(t.Context(), controlplane.Tenant{
		ID:        "tenant_1",
		Name:      "Tenant 1",
		Slug:      "tenant-1",
		Status:    controlplane.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	})
	require.NoError(t, err)
	handler := NewHandler(Config{AuthToken: "test-token", Store: store, Now: func() time.Time { return now }})

	req := httptest.NewRequest(http.MethodGet, "/mcp/tools/list-routes?tenantId=tenant_1", nil)
	req.Header.Set("X-MCP-Token", "test-token")
	req.Header.Set("X-MCP-Role", "tenant_admin")
	req.Header.Set("X-MCP-Tenant-ID", "tenant_other")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), `"code":"forbidden"`)
}

func TestTools_UsageReportAndAuditLogs(t *testing.T) {
	store := controlplane.NewStore()
	usage := billing.NewInMemoryUsageEventStore(
		billing.UsageEvent{
			EventID:        "evt_1",
			TenantID:       "tenant_1",
			ConsumerID:     "consumer_1",
			APIProductID:   "product_1",
			RouteID:        "route_1",
			SourceProtocol: "rest",
			TargetProtocol: "rest",
			Status:         billing.StatusSuccess,
			Billable:       true,
			OccurredAt:     time.Now().UTC(),
		},
	)
	now := time.Now().UTC()
	err := store.CreateTenant(t.Context(), controlplane.Tenant{
		ID:        "tenant_1",
		Name:      "Tenant 1",
		Slug:      "tenant-1",
		Status:    controlplane.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	})
	require.NoError(t, err)

	handler := NewHandler(Config{AuthToken: "test-token", Store: store, Usage: usage, Now: func() time.Time { return now }})

	usageReq := httptest.NewRequest(http.MethodGet, "/mcp/tools/usage-report?tenantId=tenant_1", nil)
	usageReq.Header.Set("X-MCP-Token", "test-token")
	usageReq.Header.Set("X-MCP-Role", "platform_admin")
	usageRec := httptest.NewRecorder()
	handler.ServeHTTP(usageRec, usageReq)
	require.Equal(t, http.StatusOK, usageRec.Code)
	require.Contains(t, usageRec.Body.String(), `"tool":"usageReport"`)
	require.Contains(t, usageRec.Body.String(), `"EventID":"evt_1"`)

	auditReq := httptest.NewRequest(http.MethodGet, "/mcp/tools/audit-logs?tenantId=tenant_1", nil)
	auditReq.Header.Set("X-MCP-Token", "test-token")
	auditReq.Header.Set("X-MCP-Role", "platform_admin")
	auditRec := httptest.NewRecorder()
	handler.ServeHTTP(auditRec, auditReq)
	require.Equal(t, http.StatusOK, auditRec.Code)
	require.Contains(t, auditRec.Body.String(), `"tool":"auditLogs"`)
	require.Contains(t, auditRec.Body.String(), `mcp.usage_report`)
}
