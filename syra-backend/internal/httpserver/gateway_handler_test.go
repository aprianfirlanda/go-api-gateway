package httpserver

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"syra-backend/internal/auth"
	"syra-backend/internal/billing"
	"syra-backend/internal/controlplane"
	"syra-backend/internal/gateway/policy"
	"syra-backend/internal/gateway/route"
	"syra-backend/internal/gateway/upstream"
	"syra-backend/internal/health"
	"syra-backend/internal/observability"
	"syra-backend/internal/protocol"
	"syra-backend/internal/protocol/iso8583"
	restprotocol "syra-backend/internal/protocol/rest"
	"syra-backend/internal/protocol/soapxml"
	"syra-backend/internal/runtime/state"
	"syra-backend/internal/transform"
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

func TestGatewayRouteRejectsDisabledTenantCredential(t *testing.T) {
	router := newGatewayTestRouter(t, gatewayTestConfig{
		tenantStatus: controlplane.StatusDisabled,
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://api.example.test/accounts", nil)
	req.Header.Set("Authorization", "ApiKey gw_live_tenant_1.secret")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestGatewayRouteRejectsDisabledConsumerCredential(t *testing.T) {
	router := newGatewayTestRouter(t, gatewayTestConfig{
		consumerStatus: controlplane.StatusDisabled,
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://api.example.test/accounts", nil)
	req.Header.Set("Authorization", "ApiKey gw_live_tenant_1.secret")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestGatewayRouteRejectsExpiredCredential(t *testing.T) {
	expired := time.Now().UTC().Add(-time.Minute)
	router := newGatewayTestRouter(t, gatewayTestConfig{
		credentialExpiresAt: &expired,
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://api.example.test/accounts", nil)
	req.Header.Set("Authorization", "ApiKey gw_live_tenant_1.secret")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
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

func TestGatewayRouteRejectsMissingScope(t *testing.T) {
	router := newGatewayTestRouter(t, gatewayTestConfig{
		credentialScopes: []string{"payments:read"},
		route: route.Route{
			ID:               "route_1",
			TenantID:         "tenant_1",
			APIProductID:     "product_1",
			InboundProtocol:  restprotocol.Name,
			OutboundProtocol: restprotocol.Name,
			UpstreamRef:      "upstream_1",
			Host:             "api.example.test",
			Method:           http.MethodGet,
			Path:             "/accounts",
			RequiredScopes:   []string{"payments:write"},
			Status:           route.StatusActive,
		},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://api.example.test/accounts", nil)
	req.Header.Set("Authorization", "ApiKey gw_live_tenant_1.secret")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestGatewayRouteRejectsInvalidHMAC(t *testing.T) {
	router := newGatewayTestRouter(t, gatewayTestConfig{
		runtimeState: state.NewInMemoryStore(state.Namespacer{Environment: "test", Version: "v1"}),
		route: route.Route{
			ID:               "route_1",
			TenantID:         "tenant_1",
			APIProductID:     "product_1",
			InboundProtocol:  restprotocol.Name,
			OutboundProtocol: restprotocol.Name,
			UpstreamRef:      "upstream_1",
			Host:             "api.example.test",
			Method:           http.MethodPost,
			Path:             "/accounts",
			HMACEnabled:      true,
			HMACSecret:       "secret-key",
			ReplayWindowSec:  300,
			Status:           route.StatusActive,
		},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "http://api.example.test/accounts", strings.NewReader(`{"amount":100}`))
	req.Header.Set("Authorization", "ApiKey gw_live_tenant_1.secret")
	req.Header.Set("X-Timestamp", time.Now().UTC().Format(time.RFC3339))
	req.Header.Set("X-Nonce", "nonce-1")
	req.Header.Set("X-Signature", "hmac-sha256=bad")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestGatewayRouteRejectsReplayedNonce(t *testing.T) {
	stateStore := state.NewInMemoryStore(state.Namespacer{Environment: "test", Version: "v1"})
	routeCfg := route.Route{
		ID:               "route_1",
		TenantID:         "tenant_1",
		APIProductID:     "product_1",
		InboundProtocol:  restprotocol.Name,
		OutboundProtocol: restprotocol.Name,
		UpstreamRef:      "upstream_1",
		Host:             "api.example.test",
		Method:           http.MethodPost,
		Path:             "/accounts",
		HMACEnabled:      true,
		HMACSecret:       "secret-key",
		ReplayWindowSec:  300,
		Status:           route.StatusActive,
	}
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	t.Cleanup(upstreamServer.Close)
	router := newGatewayTestRouter(t, gatewayTestConfig{runtimeState: stateStore, route: routeCfg, upstreamBaseURL: upstreamServer.URL})
	ts := time.Now().UTC().Format(time.RFC3339)
	nonce := "nonce-1"
	body := `{"amount":100}`
	signature := signHMACRequest(t, "secret-key", http.MethodPost, "/accounts", "", ts, nonce, body)

	req1 := httptest.NewRequest(http.MethodPost, "http://api.example.test/accounts", strings.NewReader(body))
	req1.Header.Set("Authorization", "ApiKey gw_live_tenant_1.secret")
	req1.Header.Set("X-Timestamp", ts)
	req1.Header.Set("X-Nonce", nonce)
	req1.Header.Set("X-Signature", signature)
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	require.Equal(t, http.StatusOK, rec1.Code)

	req2 := httptest.NewRequest(http.MethodPost, "http://api.example.test/accounts", strings.NewReader(body))
	req2.Header.Set("Authorization", "ApiKey gw_live_tenant_1.secret")
	req2.Header.Set("X-Timestamp", ts)
	req2.Header.Set("X-Nonce", nonce)
	req2.Header.Set("X-Signature", signature)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusForbidden, rec2.Code)
}

func TestGatewayRouteIdempotencyKeyConflict(t *testing.T) {
	stateStore := state.NewInMemoryStore(state.Namespacer{Environment: "test", Version: "v1"})
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	t.Cleanup(upstreamServer.Close)
	router := newGatewayTestRouter(t, gatewayTestConfig{
		runtimeState:    stateStore,
		upstreamBaseURL: upstreamServer.URL,
		route: route.Route{
			ID:                 "route_1",
			TenantID:           "tenant_1",
			APIProductID:       "product_1",
			InboundProtocol:    restprotocol.Name,
			OutboundProtocol:   restprotocol.Name,
			UpstreamRef:        "upstream_1",
			Host:               "api.example.test",
			Method:             http.MethodPost,
			Path:               "/accounts",
			IdempotencyEnabled: true,
			IdempotencyTTLSec:  300,
			Status:             route.StatusActive,
		},
	})
	req1 := httptest.NewRequest(http.MethodPost, "http://api.example.test/accounts", strings.NewReader(`{"amount":100}`))
	req1.Header.Set("Authorization", "ApiKey gw_live_tenant_1.secret")
	req1.Header.Set("Idempotency-Key", "idem-1")
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	require.Equal(t, http.StatusOK, rec1.Code)

	req2 := httptest.NewRequest(http.MethodPost, "http://api.example.test/accounts", strings.NewReader(`{"amount":200}`))
	req2.Header.Set("Authorization", "ApiKey gw_live_tenant_1.secret")
	req2.Header.Set("Idempotency-Key", "idem-1")
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusConflict, rec2.Code)
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

func TestGatewayRouteTransformsRESTToISO8583AndBackToREST(t *testing.T) {
	profile := gatewayISOProfile()
	codec := iso8583.NewInternalCodec()
	requestCh := make(chan []byte, 1)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buffer := make([]byte, 2048)
		n, err := conn.Read(buffer)
		if err != nil {
			return
		}
		requestCh <- append([]byte(nil), buffer[:n]...)

		response, _ := codec.Pack(profile, map[string]any{
			"mti": "0110",
			"11":  "123456",
			"38":  "A12345",
			"39":  "00",
		})
		_, _ = conn.Write(response)
	}()

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
			ID:               "route_iso",
			TenantID:         "tenant_1",
			APIProductID:     "product_1",
			InboundProtocol:  restprotocol.Name,
			OutboundProtocol: iso8583.Name,
			UpstreamRef:      "switch_1",
			TemplateRef:      "template_1",
			Host:             "api.example.test",
			Method:           http.MethodPost,
			Path:             "/cards/authorization",
			TimeoutMs:        500,
			Status:           route.StatusActive,
		}),
		UpstreamStore: upstream.NewInMemoryStore(upstream.Upstream{
			ID:               "switch_1",
			TenantID:         "tenant_1",
			Protocol:         upstream.ProtocolISO8583,
			BaseURL:          listener.Addr().String(),
			ISO8583ProfileID: profile.ID,
		}),
		AdapterRegistry: newTestAdapterRegistryWithISO(t, iso8583.NewInMemoryProfileStore(profile)),
		TemplateStore:   transform.NewInMemoryStore(gatewayISOTransformTemplate()),
		TransformEngine: transform.NewEngine(
			transform.WithClock(func() time.Time {
				return time.Date(2026, 5, 10, 12, 30, 15, 0, time.UTC)
			}),
			transform.WithSTANGenerator(func() string {
				return "123456"
			}),
		),
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "http://api.example.test/cards/authorization", strings.NewReader(`{
		"pan":"4111111111111111",
		"amount":10000,
		"currency":"IDR",
		"terminalId":"ATM00101"
	}`))
	req.Header.Set("Authorization", "ApiKey gw_live_tenant_1.secret")
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"authorizationCode":"A12345","responseCode":"00","stan":"123456"}`, rec.Body.String())

	request := <-requestCh
	require.Equal(t, len(request)-2, int(binary.BigEndian.Uint16(request[:2])))
	fields, err := codec.Unpack(profile, request)
	require.NoError(t, err)
	require.Equal(t, "0100", fields["mti"])
	require.Equal(t, "4111111111111111", fields["2"])
	require.Equal(t, "000000", fields["3"])
	require.Equal(t, "000000010000", fields["4"])
	require.Equal(t, "0510123015", fields["7"])
	require.Equal(t, "123456", fields["11"])
	require.Equal(t, "ATM00101", fields["41"])
	require.Equal(t, "360", fields["49"])
}

func TestGatewayRouteTransformsRESTToSOAPXMLAndBackToREST(t *testing.T) {
	usageEvents := billing.NewInMemoryUsageEventStore()
	metrics := observability.NewMetrics()
	var upstreamBody string
	var upstreamSOAPAction string
	var upstreamHits atomic.Int64
	soapServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits.Add(1)
		body, _ := io.ReadAll(r.Body)
		upstreamBody = string(body)
		upstreamSOAPAction = r.Header.Get("SOAPAction")
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
  <soapenv:Body>
    <AuthorizeResponse>
      <responseCode>00</responseCode>
      <authorizationCode>A12345</authorizationCode>
      <rrn>654321</rrn>
    </AuthorizeResponse>
  </soapenv:Body>
</soapenv:Envelope>`))
	}))
	t.Cleanup(soapServer.Close)

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
			ID:               "route_soap",
			TenantID:         "tenant_1",
			APIProductID:     "product_1",
			InboundProtocol:  restprotocol.Name,
			OutboundProtocol: soapxml.Name,
			UpstreamRef:      "soap_1",
			TemplateRef:      "template_soap",
			Host:             "api.example.test",
			Method:           http.MethodPost,
			Path:             "/cards/authorization",
			TimeoutMs:        500,
			Status:           route.StatusActive,
		}),
		UpstreamStore: upstream.NewInMemoryStore(upstream.Upstream{
			ID:               "soap_1",
			TenantID:         "tenant_1",
			Protocol:         upstream.ProtocolSOAPXML,
			BaseURL:          soapServer.URL,
			SOAPAction:       "Authorize",
			SOAPOperation:    "AuthorizeRequest",
			SOAPNamespace:    "urn:bank",
			SOAPResponsePath: "AuthorizeResponse",
		}),
		AdapterRegistry: newTestAdapterRegistryWithSOAP(t),
		TemplateStore:   transform.NewInMemoryStore(gatewaySOAPTransformTemplate()),
		TransformEngine: transform.NewEngine(),
		PolicyPipeline:  policy.NewPipeline(policy.NewRequestSizeLimitPolicy(1024)),
		UsageEventStore: usageEvents,
		Metrics:         metrics,
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "http://api.example.test/cards/authorization", strings.NewReader(`{
		"pan":"4111111111111111",
		"amount":10000,
		"currency":"IDR",
		"terminalId":"ATM00101"
	}`))
	req.Header.Set("Authorization", "ApiKey gw_live_tenant_1.secret")
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"authorizationCode":"A12345","responseCode":"00","rrn":"654321"}`, rec.Body.String())
	require.Equal(t, int64(1), upstreamHits.Load())
	require.Equal(t, "Authorize", upstreamSOAPAction)
	require.Contains(t, upstreamBody, `<ns:AuthorizeRequest>`)
	require.Contains(t, upstreamBody, `<ns:pan>4111111111111111</ns:pan>`)
	require.Contains(t, upstreamBody, `<ns:amount>000000010000</ns:amount>`)

	events := listUsageEvents(t, usageEvents)
	require.Len(t, events, 1)
	require.Equal(t, billing.StatusSuccess, events[0].Status)
	require.Equal(t, soapxml.Name, events[0].TargetProtocol)
	require.True(t, events[0].Billable)

	metricsRec := httptest.NewRecorder()
	router.ServeHTTP(metricsRec, httptest.NewRequest(http.MethodGet, "http://api.example.test/metrics", nil))
	require.Equal(t, http.StatusOK, metricsRec.Code)
	require.Contains(t, metricsRec.Body.String(), "syra_gateway_requests_total")
}

func TestGatewayRouteRejectsUnpublishedTransformationTemplate(t *testing.T) {
	usageEvents := billing.NewInMemoryUsageEventStore()
	upstreamHits := atomic.Int64{}
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstreamServer.Close)

	template := gatewaySOAPTransformTemplate()
	template.Status = transform.StatusDraft
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
			ID:               "route_soap",
			TenantID:         "tenant_1",
			APIProductID:     "product_1",
			InboundProtocol:  restprotocol.Name,
			OutboundProtocol: restprotocol.Name,
			UpstreamRef:      "upstream_1",
			TemplateRef:      template.ID,
			Host:             "api.example.test",
			Method:           http.MethodPost,
			Path:             "/cards/authorization",
			Status:           route.StatusActive,
		}),
		UpstreamStore: upstream.NewInMemoryStore(upstream.Upstream{
			ID:       "upstream_1",
			TenantID: "tenant_1",
			Protocol: upstream.ProtocolREST,
			BaseURL:  upstreamServer.URL,
		}),
		AdapterRegistry: newTestAdapterRegistry(t),
		TemplateStore:   transform.NewInMemoryStore(template),
		TransformEngine: transform.NewEngine(),
		UsageEventStore: usageEvents,
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "http://api.example.test/cards/authorization", strings.NewReader(`{"amount":10000}`))
	req.Header.Set("Authorization", "ApiKey gw_live_tenant_1.secret")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.JSONEq(t, `{"error":{"code":"template_not_published","message":"Transformation template is not published"}}`, rec.Body.String())
	require.Equal(t, int64(0), upstreamHits.Load())
	events := listUsageEvents(t, usageEvents)
	require.Len(t, events, 1)
	require.False(t, events[0].Billable)
}

type gatewayTestConfig struct {
	credentialStatus    string
	credentialTenantID  string
	consumerID          string
	credentialID        string
	keyPrefix           string
	credentialScopes    []string
	credentialExpiresAt *time.Time
	tenantStatus        string
	consumerStatus      string
	upstreamBaseURL     string
	timeoutMs           int
	policies            *policy.Pipeline
	usageEvents         billing.UsageEventStore
	route               route.Route
	runtimeState        state.Store
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
	if cfg.tenantStatus == "" {
		cfg.tenantStatus = auth.StatusActive
	}
	if cfg.consumerStatus == "" {
		cfg.consumerStatus = auth.StatusActive
	}
	if cfg.route.ID == "" {
		cfg.route = route.Route{
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
		}
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
			ID:             cfg.credentialID,
			TenantID:       cfg.credentialTenantID,
			ConsumerID:     cfg.consumerID,
			KeyPrefix:      cfg.keyPrefix,
			SecretHash:     secretHash,
			Status:         cfg.credentialStatus,
			Scopes:         cfg.credentialScopes,
			ExpiresAt:      cfg.credentialExpiresAt,
			TenantStatus:   cfg.tenantStatus,
			ConsumerStatus: cfg.consumerStatus,
		}),
		RouteRegistry: route.NewInMemoryRegistry(cfg.route),
		UpstreamStore: upstream.NewInMemoryStore(upstream.Upstream{
			ID:       "upstream_1",
			TenantID: "tenant_1",
			Protocol: upstream.ProtocolREST,
			BaseURL:  cfg.upstreamBaseURL,
		}),
		AdapterRegistry: newTestAdapterRegistry(t),
		PolicyPipeline:  cfg.policies,
		UsageEventStore: cfg.usageEvents,
		RuntimeState:    cfg.runtimeState,
	})
}

func TestGatewayRouteEmitsSuccessfulUsageEvent(t *testing.T) {
	usageEvents := billing.NewInMemoryUsageEventStore()
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(upstreamServer.Close)
	router := newGatewayTestRouter(t, gatewayTestConfig{
		upstreamBaseURL: upstreamServer.URL,
		usageEvents:     usageEvents,
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://api.example.test/accounts", nil)
	req.Header.Set("Authorization", "ApiKey gw_live_tenant_1.secret")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	events := listUsageEvents(t, usageEvents)
	require.Len(t, events, 1)
	require.NotEmpty(t, events[0].EventID)
	require.Equal(t, "tenant_1", events[0].TenantID)
	require.Equal(t, "consumer_1", events[0].ConsumerID)
	require.Equal(t, "product_1", events[0].APIProductID)
	require.Equal(t, "route_1", events[0].RouteID)
	require.Equal(t, restprotocol.Name, events[0].SourceProtocol)
	require.Equal(t, restprotocol.Name, events[0].TargetProtocol)
	require.Equal(t, billing.StatusSuccess, events[0].Status)
	require.Equal(t, http.StatusCreated, events[0].HTTPStatus)
	require.Equal(t, "201", events[0].UpstreamStatus)
	require.True(t, events[0].Billable)
	require.NotZero(t, events[0].OccurredAt)
}

func TestGatewayRouteEmitsRejectedUsageEvent(t *testing.T) {
	usageEvents := billing.NewInMemoryUsageEventStore()
	router := newGatewayTestRouter(t, gatewayTestConfig{
		policies:    policy.NewPipeline(policy.NewRequestSizeLimitPolicy(3)),
		usageEvents: usageEvents,
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://api.example.test/accounts", strings.NewReader("abcd"))
	req.Header.Set("Authorization", "ApiKey gw_live_tenant_1.secret")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	events := listUsageEvents(t, usageEvents)
	require.Len(t, events, 1)
	require.Equal(t, billing.StatusRejected, events[0].Status)
	require.Equal(t, http.StatusRequestEntityTooLarge, events[0].HTTPStatus)
	require.False(t, events[0].Billable)
}

func TestGatewayRouteEmitsRejectedUsageEventForInvalidAPIKey(t *testing.T) {
	usageEvents := billing.NewInMemoryUsageEventStore()
	router := newGatewayTestRouter(t, gatewayTestConfig{
		usageEvents: usageEvents,
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://api.example.test/accounts", nil)
	req.Header.Set("Authorization", "ApiKey gw_live_tenant_1.wrong")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	events := listUsageEvents(t, usageEvents)
	require.Len(t, events, 1)
	require.Equal(t, "tenant_1", events[0].TenantID)
	require.Equal(t, "consumer_1", events[0].ConsumerID)
	require.Equal(t, billing.StatusRejected, events[0].Status)
	require.Equal(t, http.StatusUnauthorized, events[0].HTTPStatus)
	require.False(t, events[0].Billable)
	require.NotContains(t, eventValues(events[0]), "wrong")
}

func TestGatewayRouteEmitsFailedBillableUsageEvent(t *testing.T) {
	usageEvents := billing.NewInMemoryUsageEventStore()
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"upstream failed","pan":"4111111111111111"}`))
	}))
	t.Cleanup(upstreamServer.Close)
	router := newGatewayTestRouter(t, gatewayTestConfig{
		upstreamBaseURL: upstreamServer.URL,
		usageEvents:     usageEvents,
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://api.example.test/accounts", strings.NewReader(`{"pan":"4111111111111111","cvv":"123"}`))
	req.Header.Set("Authorization", "ApiKey gw_live_tenant_1.secret")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	events := listUsageEvents(t, usageEvents)
	require.Len(t, events, 1)
	require.Equal(t, billing.StatusFailed, events[0].Status)
	require.Equal(t, http.StatusInternalServerError, events[0].HTTPStatus)
	require.Equal(t, "500", events[0].UpstreamStatus)
	require.True(t, events[0].Billable)
	require.NotContains(t, eventValues(events[0]), "4111111111111111")
	require.NotContains(t, eventValues(events[0]), `"cvv":"123"`)
}

func TestGatewayRouteEmitsTimeoutUsageEvent(t *testing.T) {
	usageEvents := billing.NewInMemoryUsageEventStore()
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(75 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstreamServer.Close)
	router := newGatewayTestRouter(t, gatewayTestConfig{
		upstreamBaseURL: upstreamServer.URL,
		timeoutMs:       10,
		usageEvents:     usageEvents,
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://api.example.test/accounts", nil)
	req.Header.Set("Authorization", "ApiKey gw_live_tenant_1.secret")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusGatewayTimeout, rec.Code)
	events := listUsageEvents(t, usageEvents)
	require.Len(t, events, 1)
	require.Equal(t, billing.StatusTimeout, events[0].Status)
	require.Equal(t, http.StatusGatewayTimeout, events[0].HTTPStatus)
	require.True(t, events[0].Billable)
}

func TestGatewayRouteBlocksIPPolicy(t *testing.T) {
	upstreamHits := atomic.Int64{}
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstreamServer.Close)
	allowlist, err := policy.NewIPAllowlistPolicy("10.0.0.0/8")
	require.NoError(t, err)
	router := newGatewayTestRouter(t, gatewayTestConfig{
		upstreamBaseURL: upstreamServer.URL,
		policies:        policy.NewPipeline(allowlist),
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://api.example.test/accounts", nil)
	req.RemoteAddr = "192.168.1.10:1234"
	req.Header.Set("Authorization", "ApiKey gw_live_tenant_1.secret")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.JSONEq(t, `{"error":{"code":"blocked_ip","message":"IP address is not allowed"}}`, rec.Body.String())
	require.Equal(t, int64(0), upstreamHits.Load())
}

func TestGatewayRouteRejectsOversizedRequestPolicy(t *testing.T) {
	router := newGatewayTestRouter(t, gatewayTestConfig{
		policies: policy.NewPipeline(policy.NewRequestSizeLimitPolicy(3)),
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://api.example.test/accounts", strings.NewReader("abcd"))
	req.Header.Set("Authorization", "ApiKey gw_live_tenant_1.secret")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	require.JSONEq(t, `{"error":{"code":"request_too_large","message":"Request is too large"}}`, rec.Body.String())
}

func TestGatewayRouteRateLimitPolicy(t *testing.T) {
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstreamServer.Close)
	router := newGatewayTestRouter(t, gatewayTestConfig{
		upstreamBaseURL: upstreamServer.URL,
		policies: policy.NewPipeline(policy.NewRateLimitPolicy(
			policy.NewInMemoryRateLimiter(1, time.Minute),
		)),
	})

	for idx := 0; idx < 2; idx++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "http://api.example.test/accounts", nil)
		req.Header.Set("Authorization", "ApiKey gw_live_tenant_1.secret")
		router.ServeHTTP(rec, req)
		if idx == 0 {
			require.Equal(t, http.StatusOK, rec.Code)
			continue
		}
		require.Equal(t, http.StatusTooManyRequests, rec.Code)
		require.JSONEq(t, `{"error":{"code":"rate_limited","message":"Rate limit exceeded"}}`, rec.Body.String())
	}
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

func listUsageEvents(t *testing.T, store billing.UsageEventStore) []billing.UsageEvent {
	t.Helper()

	events, err := store.List(context.Background(), billing.UsageEventFilter{})
	require.NoError(t, err)
	return events
}

func eventValues(event billing.UsageEvent) string {
	return strings.Join([]string{
		event.EventID,
		event.TenantID,
		event.ConsumerID,
		event.APIProductID,
		event.RouteID,
		event.SourceProtocol,
		event.TargetProtocol,
		event.Status,
		event.UpstreamStatus,
	}, " ")
}

func newTestAdapterRegistry(t *testing.T) *protocol.Registry {
	t.Helper()

	registry := protocol.NewRegistry()
	adapter := restprotocol.NewAdapter(http.DefaultClient)
	require.NoError(t, registry.RegisterProtocol(adapter))
	require.NoError(t, registry.RegisterUpstream(adapter))
	return registry
}

func newTestAdapterRegistryWithISO(t *testing.T, profiles iso8583.ProfileStore) *protocol.Registry {
	t.Helper()

	registry := newTestAdapterRegistry(t)
	isoAdapter := iso8583.NewAdapter(nil, profiles, nil)
	require.NoError(t, registry.RegisterUpstream(isoAdapter))
	return registry
}

func newTestAdapterRegistryWithSOAP(t *testing.T) *protocol.Registry {
	t.Helper()

	registry := newTestAdapterRegistry(t)
	require.NoError(t, registry.RegisterUpstream(soapxml.NewAdapter(http.DefaultClient)))
	return registry
}

func gatewayISOTransformTemplate() transform.Template {
	return transform.Template{
		ID:             "template_1",
		TenantID:       "tenant_1",
		APIProductID:   "product_1",
		Name:           "rest-to-iso8583-card-auth",
		SourceProtocol: restprotocol.Name,
		TargetProtocol: iso8583.Name,
		Version:        1,
		Status:         transform.StatusPublished,
		Request: transform.Section{
			Fields: map[string]string{
				"2":  "$.fields.pan",
				"3":  "'000000'",
				"4":  "formatAmount($.fields.amount)",
				"7":  "nowMMddHHmmss()",
				"11": "generateStan()",
				"41": "$.fields.terminalId",
				"49": "currencyNumeric($.fields.currency)",
			},
			Sensitive: []string{"2"},
		},
		Response: transform.Section{
			Fields: map[string]string{
				"responseCode":      "$.fields.39",
				"authorizationCode": "$.fields.38",
				"stan":              "$.fields.11",
			},
		},
	}
}

func signHMACRequest(t *testing.T, secret, method, path, query, timestamp, nonce, body string) string {
	t.Helper()
	bodyHash := sha256.Sum256([]byte(body))
	canonical := strings.Join([]string{
		method,
		path,
		query,
		timestamp,
		nonce,
		hex.EncodeToString(bodyHash[:]),
	}, "\n")
	mac := hmac.New(sha256.New, []byte(secret))
	_, err := mac.Write([]byte(canonical))
	require.NoError(t, err)
	return "hmac-sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func gatewaySOAPTransformTemplate() transform.Template {
	return transform.Template{
		ID:             "template_soap",
		TenantID:       "tenant_1",
		APIProductID:   "product_1",
		Name:           "rest-to-soap-card-auth",
		SourceProtocol: restprotocol.Name,
		TargetProtocol: soapxml.Name,
		Version:        1,
		Status:         transform.StatusPublished,
		Request: transform.Section{
			Fields: map[string]string{
				"pan":        "$.fields.pan",
				"amount":     "formatAmount($.fields.amount)",
				"currency":   "$.fields.currency",
				"terminalId": "$.fields.terminalId",
			},
			Sensitive: []string{"pan"},
		},
		Response: transform.Section{
			Fields: map[string]string{
				"responseCode":      "$.fields.responseCode",
				"authorizationCode": "$.fields.authorizationCode",
				"rrn":               "$.fields.rrn",
			},
		},
	}
}

func gatewayISOProfile() iso8583.Profile {
	return iso8583.Profile{
		ID:           "profile_1",
		MTI:          "0100",
		ResponseMTI:  "0110",
		LengthHeader: true,
		Fields: map[int]iso8583.FieldSpec{
			2:  {ID: 2, Type: iso8583.FieldLLVAR, Length: 19, Sensitive: true},
			3:  {ID: 3, Type: iso8583.FieldFixed, Length: 6},
			4:  {ID: 4, Type: iso8583.FieldFixed, Length: 12},
			7:  {ID: 7, Type: iso8583.FieldFixed, Length: 10},
			11: {ID: 11, Type: iso8583.FieldFixed, Length: 6},
			38: {ID: 38, Type: iso8583.FieldFixed, Length: 6},
			39: {ID: 39, Type: iso8583.FieldFixed, Length: 2},
			41: {ID: 41, Type: iso8583.FieldFixed, Length: 8},
			49: {ID: 49, Type: iso8583.FieldFixed, Length: 3},
		},
		SensitiveKeys: []string{"2"},
	}
}
