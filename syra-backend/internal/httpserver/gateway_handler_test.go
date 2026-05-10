package httpserver

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"syra-backend/internal/auth"
	"syra-backend/internal/gateway/route"
	"syra-backend/internal/gateway/upstream"
	"syra-backend/internal/health"
	"syra-backend/internal/protocol"
	restprotocol "syra-backend/internal/protocol/rest"
)

func TestGatewayRouteRequiresAPIKey(t *testing.T) {
	var upstreamHits atomic.Int64
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstreamServer.Close)
	router := newGatewayTestRouter(t, gatewayTestConfig{upstreamBaseURL: upstreamServer.URL})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://api.example.test/accounts", nil)

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.JSONEq(t, `{"error":{"code":"unauthorized","message":"Missing API key"}}`, rec.Body.String())
	require.Equal(t, int64(0), upstreamHits.Load())
}

func TestGatewayRouteRejectsInvalidAPIKey(t *testing.T) {
	var upstreamHits atomic.Int64
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstreamServer.Close)
	router := newGatewayTestRouter(t, gatewayTestConfig{upstreamBaseURL: upstreamServer.URL})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://api.example.test/accounts", nil)
	req.Header.Set("Authorization", "ApiKey gw_live_tenant_1.wrong")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.JSONEq(t, `{"error":{"code":"unauthorized","message":"Invalid API key"}}`, rec.Body.String())
	require.Equal(t, int64(0), upstreamHits.Load())
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

func TestGatewayRouteProxiesAuthenticatedRequestToUpstream(t *testing.T) {
	var upstreamMethod string
	var upstreamPath string
	var upstreamQuery string
	var upstreamAllowedHeader string
	var upstreamAuthHeader string
	var upstreamAPIKeyHeader string
	var upstreamConnectionHeader string

	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamMethod = r.Method
		upstreamPath = r.URL.Path
		upstreamQuery = r.URL.RawQuery
		upstreamAllowedHeader = r.Header.Get("X-Partner-Trace")
		upstreamAuthHeader = r.Header.Get("Authorization")
		upstreamAPIKeyHeader = r.Header.Get("X-API-Key")
		upstreamConnectionHeader = r.Header.Get("Connection")

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Connection", "close")
		w.Header().Set("X-Upstream-Trace", "trace-1")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(upstreamServer.Close)

	router := newGatewayTestRouter(t, gatewayTestConfig{upstreamBaseURL: upstreamServer.URL})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://api.example.test/accounts?limit=10", nil)
	req.Header.Set("Authorization", "ApiKey gw_live_tenant_1.secret")
	req.Header.Set("X-Partner-Trace", "trace-1")
	req.Header.Set("Connection", "upgrade")
	req.Header.Set("Upgrade", "websocket")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
	require.JSONEq(t, `{"ok":true}`, rec.Body.String())
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	require.Equal(t, "trace-1", rec.Header().Get("X-Upstream-Trace"))
	require.Empty(t, rec.Header().Get("Connection"))

	require.Equal(t, http.MethodGet, upstreamMethod)
	require.Equal(t, "/accounts", upstreamPath)
	require.Equal(t, "limit=10", upstreamQuery)
	require.Equal(t, "trace-1", upstreamAllowedHeader)
	require.Empty(t, upstreamAuthHeader)
	require.Empty(t, upstreamAPIKeyHeader)
	require.Empty(t, upstreamConnectionHeader)
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

func TestGatewayRouteTimeout(t *testing.T) {
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(75 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstreamServer.Close)

	router := newGatewayTestRouter(t, gatewayTestConfig{
		upstreamBaseURL: upstreamServer.URL,
		timeoutMs:       10,
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://api.example.test/accounts", nil)
	req.Header.Set("Authorization", "ApiKey gw_live_tenant_1.secret")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusGatewayTimeout, rec.Code)
	require.JSONEq(t, `{"error":{"code":"upstream_timeout","message":"Upstream request timed out"}}`, rec.Body.String())
}

type gatewayTestConfig struct {
	credentialStatus   string
	credentialTenantID string
	consumerID         string
	credentialID       string
	keyPrefix          string
	upstreamBaseURL    string
	timeoutMs          int
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
	if cfg.upstreamBaseURL == "" {
		cfg.upstreamBaseURL = "http://upstream.example.test"
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
			ID:               "route_1",
			TenantID:         "tenant_1",
			APIProductID:     "product_1",
			InboundProtocol:  restprotocol.Name,
			OutboundProtocol: restprotocol.Name,
			UpstreamRef:      "upstream_1",
			Host:             "api.example.test",
			Method:           http.MethodGet,
			Path:             "/accounts",
			TimeoutMs:        cfg.timeoutMs,
			Status:           route.StatusActive,
		}),
		UpstreamStore: upstream.NewInMemoryStore(upstream.Upstream{
			ID:       "upstream_1",
			TenantID: "tenant_1",
			Protocol: upstream.ProtocolREST,
			BaseURL:  cfg.upstreamBaseURL,
		}),
		AdapterRegistry: newTestAdapterRegistry(t),
	})
}

func TestGatewayRouteReturnsBadGatewayWhenUpstreamMissing(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:        slog.New(slog.NewTextHandler(discardWriter{}, nil)),
		HealthService: health.NewService(nil),
		CredentialStore: auth.NewInMemoryCredentialStore(mustCredential(t, auth.APIKeyCredential{
			ID:         "credential_1",
			TenantID:   "tenant_1",
			ConsumerID: "consumer_1",
			KeyPrefix:  "gw_live_tenant_1",
			Status:     auth.StatusActive,
		})),
		RouteRegistry: route.NewInMemoryRegistry(route.Route{
			ID:               "route_1",
			TenantID:         "tenant_1",
			InboundProtocol:  restprotocol.Name,
			OutboundProtocol: restprotocol.Name,
			UpstreamRef:      "missing_upstream",
			Host:             "api.example.test",
			Method:           http.MethodGet,
			Path:             "/accounts",
			Status:           route.StatusActive,
		}),
		UpstreamStore:   upstream.NewInMemoryStore(),
		AdapterRegistry: newTestAdapterRegistry(t),
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://api.example.test/accounts", nil)
	req.Header.Set("Authorization", "ApiKey gw_live_tenant_1.secret")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.JSONEq(t, `{"error":{"code":"upstream_not_found","message":"Upstream not found"}}`, rec.Body.String())
}

func mustCredential(t *testing.T, credential auth.APIKeyCredential) auth.APIKeyCredential {
	t.Helper()

	secretHash, err := auth.HashSecretWithParams("secret", auth.HashParams{
		Memory:      32,
		Iterations:  1,
		Parallelism: 1,
		SaltLength:  8,
		KeyLength:   16,
	})
	require.NoError(t, err)

	credential.SecretHash = secretHash
	return credential
}

func newTestAdapterRegistry(t *testing.T) *protocol.Registry {
	t.Helper()

	registry := protocol.NewRegistry()
	adapter := restprotocol.NewAdapter(http.DefaultClient)
	require.NoError(t, registry.RegisterProtocol(adapter))
	require.NoError(t, registry.RegisterUpstream(adapter))
	return registry
}
