package httpserver

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"syra-backend/internal/auth"
	"syra-backend/internal/gateway/route"
	"syra-backend/internal/health"
)

func TestGatewayRouteRequiresAPIKey(t *testing.T) {
	router := newGatewayTestRouter(t, gatewayTestConfig{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://api.example.test/accounts", nil)

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.JSONEq(t, `{"error":{"code":"unauthorized","message":"Missing API key"}}`, rec.Body.String())
}

func TestGatewayRouteRejectsInvalidAPIKey(t *testing.T) {
	router := newGatewayTestRouter(t, gatewayTestConfig{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://api.example.test/accounts", nil)
	req.Header.Set("Authorization", "ApiKey gw_live_tenant_1.wrong")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.JSONEq(t, `{"error":{"code":"unauthorized","message":"Invalid API key"}}`, rec.Body.String())
}

func TestGatewayRouteRejectsSuspendedCredential(t *testing.T) {
	router := newGatewayTestRouter(t, gatewayTestConfig{
		credentialStatus: auth.StatusSuspended,
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://api.example.test/accounts", nil)
	req.Header.Set("Authorization", "ApiKey gw_live_tenant_1.secret")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.JSONEq(t, `{"error":{"code":"forbidden","message":"Credential is not allowed"}}`, rec.Body.String())
}

func TestGatewayRouteMatchesAuthenticatedTenantRoute(t *testing.T) {
	router := newGatewayTestRouter(t, gatewayTestConfig{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://api.example.test/accounts", nil)
	req.Header.Set("Authorization", "ApiKey gw_live_tenant_1.secret")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{
		"status": "matched",
		"tenantId": "tenant_1",
		"consumerId": "consumer_1",
		"credentialId": "credential_1",
		"routeId": "route_1"
	}`, rec.Body.String())
}

func TestGatewayRouteDoesNotCrossTenantMatch(t *testing.T) {
	router := newGatewayTestRouter(t, gatewayTestConfig{
		credentialTenantID: "tenant_2",
		consumerID:         "consumer_2",
		credentialID:       "credential_2",
		keyPrefix:          "gw_live_tenant_2",
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://api.example.test/accounts", nil)
	req.Header.Set("Authorization", "ApiKey gw_live_tenant_2.secret")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.JSONEq(t, `{"error":{"code":"route_not_found","message":"Route not found"}}`, rec.Body.String())
}

type gatewayTestConfig struct {
	credentialStatus   string
	credentialTenantID string
	consumerID         string
	credentialID       string
	keyPrefix          string
}

func newGatewayTestRouter(t *testing.T, cfg gatewayTestConfig) http.Handler {
	t.Helper()

	if cfg.credentialStatus == "" {
		cfg.credentialStatus = auth.StatusActive
	}
	if cfg.credentialTenantID == "" {
		cfg.credentialTenantID = "tenant_1"
	}
	if cfg.consumerID == "" {
		cfg.consumerID = "consumer_1"
	}
	if cfg.credentialID == "" {
		cfg.credentialID = "credential_1"
	}
	if cfg.keyPrefix == "" {
		cfg.keyPrefix = "gw_live_tenant_1"
	}

	secretHash, err := auth.HashSecretWithParams("secret", auth.HashParams{
		Memory:      32,
		Iterations:  1,
		Parallelism: 1,
		SaltLength:  8,
		KeyLength:   16,
	})
	require.NoError(t, err)

	return NewRouter(Dependencies{
		Logger:        slog.New(slog.NewTextHandler(discardWriter{}, nil)),
		HealthService: health.NewService(nil),
		CredentialStore: auth.NewInMemoryCredentialStore(auth.APIKeyCredential{
			ID:         cfg.credentialID,
			TenantID:   cfg.credentialTenantID,
			ConsumerID: cfg.consumerID,
			KeyPrefix:  cfg.keyPrefix,
			SecretHash: secretHash,
			Status:     cfg.credentialStatus,
		}),
		RouteRegistry: route.NewInMemoryRegistry(route.Route{
			ID:           "route_1",
			TenantID:     "tenant_1",
			APIProductID: "product_1",
			Host:         "api.example.test",
			Method:       http.MethodGet,
			Path:         "/accounts",
			Status:       route.StatusActive,
		}),
	})
}
