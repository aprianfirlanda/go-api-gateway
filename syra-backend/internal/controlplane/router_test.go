package controlplane

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAdminAPIsRequireBearerToken(t *testing.T) {
	router, _ := newTestRouter()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/v1/tenants", nil)

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "unauthorized")
}

func TestTenantAPIWritesAuditEvent(t *testing.T) {
	router, store := newTestRouter()

	rec := httptest.NewRecorder()
	req := newAdminRequest(http.MethodPost, "/admin/v1/tenants", `{
		"name":"Bank A",
		"slug":"bank-a",
		"billingPlanId":"plan_1",
		"metadata":{"industry":"banking"}
	}`)

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	var tenant Tenant
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tenant))
	require.NotEmpty(t, tenant.ID)
	require.Equal(t, "Bank A", tenant.Name)
	require.Equal(t, StatusActive, tenant.Status)

	auditEvents, err := store.ListAuditEvents(context.Background())
	require.NoError(t, err)
	require.Len(t, auditEvents, 1)
	require.Equal(t, "tenant.create", auditEvents[0].Action)
	require.Equal(t, "tenant", auditEvents[0].Resource)
	require.Equal(t, tenant.ID, auditEvents[0].ResourceID)
	require.Equal(t, tenant.ID, auditEvents[0].TenantID)
}

func TestTenantScopedResourceAPIs(t *testing.T) {
	router, store := newTestRouter()
	tenantID := createTenant(t, router)

	product := postJSON[APIProduct](t, router, "/admin/v1/tenants/"+tenantID+"/api-products", `{
		"name":"Card Authorization",
		"slug":"card-authorization",
		"description":"Card purchase authorization API"
	}`)
	require.Equal(t, tenantID, product.TenantID)

	upstream := postJSON[Upstream](t, router, "/admin/v1/tenants/"+tenantID+"/upstreams", `{
		"name":"switch-primary",
		"protocol":"iso8583",
		"config":{"host":"10.10.10.20","port":5000,"profileId":"profile_1"}
	}`)
	require.Equal(t, "iso8583", upstream.Protocol)

	route := postJSON[Route](t, router, "/admin/v1/tenants/"+tenantID+"/routes", `{
		"apiProductId":"`+product.ID+`",
		"name":"Purchase Authorization",
		"inboundProtocol":"rest",
		"outboundProtocol":"iso8583",
		"host":"api.gateway.example.com",
		"method":"POST",
		"path":"/cards/authorization",
		"upstreamId":"`+upstream.ID+`",
		"transformationTemplateId":"template_1",
		"priority":100,
		"timeoutMs":5000,
		"status":"draft"
	}`)
	require.Equal(t, StatusDraft, route.Status)

	published := requestJSON[Route](t, router, http.MethodPost, "/admin/v1/tenants/"+tenantID+"/routes/"+route.ID+"/publish", `{}`, http.StatusOK)
	require.Equal(t, StatusActive, published.Status)

	rec := httptest.NewRecorder()
	req := newAdminRequest(http.MethodGet, "/admin/v1/tenants/"+tenantID+"/routes?status=active", "")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var routes listResponse[Route]
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &routes))
	require.Len(t, routes.Data, 1)
	require.Equal(t, route.ID, routes.Data[0].ID)

	auditEvents, err := store.ListAuditEvents(context.Background())
	require.NoError(t, err)
	requireAuditAction(t, auditEvents, "api_product.create")
	requireAuditAction(t, auditEvents, "upstream.create")
	requireAuditAction(t, auditEvents, "route.create")
	requireAuditAction(t, auditEvents, "route.publish")
}

func TestCredentialAPIReturnsFullKeyOnceAndAuditsWithoutSecret(t *testing.T) {
	router, store := newTestRouter()
	tenantID := createTenant(t, router)
	consumer := postJSON[Consumer](t, router, "/admin/v1/tenants/"+tenantID+"/consumers", `{
		"name":"Mobile Banking App",
		"slug":"mobile-banking-app"
	}`)

	credential := postJSON[CredentialCreateResponse](t, router, "/admin/v1/tenants/"+tenantID+"/consumers/"+consumer.ID+"/credentials", `{
		"type":"api_key",
		"scopes":["api:card-authorization:invoke"]
	}`)

	require.NotEmpty(t, credential.ID)
	require.NotEmpty(t, credential.KeyPrefix)
	require.True(t, strings.HasPrefix(credential.APIKey, credential.KeyPrefix+"."))

	stored, err := store.GetCredential(context.Background(), tenantID, credential.ID)
	require.NoError(t, err)
	require.NotEmpty(t, stored.SecretHash)
	require.NotContains(t, stored.SecretHash, credential.APIKey)
	require.NotContains(t, stored.SecretHash, strings.TrimPrefix(credential.APIKey, credential.KeyPrefix+"."))

	auditEvents, err := store.ListAuditEvents(context.Background())
	require.NoError(t, err)
	for _, event := range auditEvents {
		encoded, err := json.Marshal(event)
		require.NoError(t, err)
		require.NotContains(t, string(encoded), credential.APIKey)
	}
	requireAuditAction(t, auditEvents, "credential.create")
}

func TestTransformationTemplateAPI(t *testing.T) {
	router, store := newTestRouter()
	tenantID := createTenant(t, router)
	product := postJSON[APIProduct](t, router, "/admin/v1/tenants/"+tenantID+"/api-products", `{"name":"Card Authorization","slug":"card-authorization"}`)

	template := postJSON[TransformationTemplate](t, router, "/admin/v1/tenants/"+tenantID+"/transformation-templates", `{
		"apiProductId":"`+product.ID+`",
		"name":"card-authorization-rest-to-iso8583",
		"sourceProtocol":"rest",
		"targetProtocol":"iso8583",
		"templateBody":{"request":{"fields":{"2":"$.fields.pan"}},"response":{"fields":{"responseCode":"$.fields.39"}}}
	}`)
	require.Equal(t, StatusDraft, template.Status)

	published := requestJSON[TransformationTemplate](t, router, http.MethodPost, "/admin/v1/tenants/"+tenantID+"/transformation-templates/"+template.ID+"/publish", `{}`, http.StatusOK)
	require.Equal(t, "published", published.Status)
	require.NotNil(t, published.PublishedAt)

	auditEvents, err := store.ListAuditEvents(context.Background())
	require.NoError(t, err)
	requireAuditAction(t, auditEvents, "transformation_template.create")
	requireAuditAction(t, auditEvents, "transformation_template.publish")
}

func newTestRouter() (http.Handler, *Store) {
	store := NewStore()
	router := NewRouter(RouterConfig{
		AdminToken: "test-token",
		Store:      store,
		Now: func() time.Time {
			return time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
		},
	})
	return router, store
}

func newAdminRequest(method string, path string, body string) *http.Request {
	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	return req
}

func postJSON[T any](t *testing.T, router http.Handler, path string, body string) T {
	t.Helper()
	return requestJSON[T](t, router, http.MethodPost, path, body, http.StatusCreated)
}

func requestJSON[T any](t *testing.T, router http.Handler, method string, path string, body string, status int) T {
	t.Helper()
	rec := httptest.NewRecorder()
	req := newAdminRequest(method, path, body)
	router.ServeHTTP(rec, req)
	require.Equal(t, status, rec.Code, rec.Body.String())
	var out T
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	return out
}

func createTenant(t *testing.T, router http.Handler) string {
	t.Helper()
	tenant := postJSON[Tenant](t, router, "/admin/v1/tenants", `{"name":"Bank A","slug":"bank-a"}`)
	return tenant.ID
}

func requireAuditAction(t *testing.T, events []AuditEvent, action string) {
	t.Helper()
	for _, event := range events {
		if event.Action == action {
			return
		}
	}
	require.Failf(t, "missing audit action", "action %q not found in %#v", action, events)
}
