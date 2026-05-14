package rest

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"backend/internal/protocol"
)

func TestDecodeMapsHTTPRequestToCanonicalMessage(t *testing.T) {
	adapter := NewAdapter(nil)
	req := httptest.NewRequest(http.MethodPost, "http://api.example.test/payments?trace=1", strings.NewReader("body"))
	req.Header.Set("X-Trace", "trace-1")

	msg, err := adapter.Decode(context.Background(), protocol.InboundRequest{
		HTTPRequest:    req,
		TenantID:       "tenant_1",
		ConsumerID:     "consumer_1",
		CredentialID:   "credential_1",
		APIProductID:   "product_1",
		RouteID:        "route_1",
		SourceProtocol: Name,
		TargetProtocol: Name,
		Operation:      "proxy",
	})

	require.NoError(t, err)
	require.Equal(t, "tenant_1", msg.TenantID)
	require.Equal(t, http.MethodPost, msg.Method)
	require.Equal(t, "/payments", msg.Path)
	require.Equal(t, "trace=1", msg.RawQuery)
	require.Equal(t, "trace-1", msg.Headers.Get("X-Trace"))
	require.NotNil(t, msg.Body)
}

func TestEncodeMapsCanonicalMessageToOutboundResponse(t *testing.T) {
	adapter := NewAdapter(nil)

	resp, err := adapter.Encode(context.Background(), protocol.CanonicalMessage{
		StatusCode: http.StatusCreated,
		Headers:    http.Header{"X-Trace": []string{"trace-1"}},
		Body:       io.NopCloser(strings.NewReader("created")),
	})

	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Equal(t, "trace-1", resp.Headers.Get("X-Trace"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "created", string(body))
}

func TestCallMapsCanonicalMessageToRESTUpstream(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotQuery string
	var gotTrace string
	var gotAuth string
	var gotBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotTrace = r.Header.Get("X-Trace")
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)

		w.Header().Set("X-Upstream", "ok")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("accepted"))
	}))
	t.Cleanup(server.Close)

	adapter := NewAdapter(http.DefaultClient)
	msg, err := adapter.Call(context.Background(), protocol.UpstreamTarget{
		ID:       "upstream_1",
		TenantID: "tenant_1",
		Protocol: Name,
		BaseURL:  server.URL + "/base",
	}, protocol.CanonicalMessage{
		Method:   http.MethodPost,
		Path:     "/payments",
		RawQuery: "trace=1",
		Headers: http.Header{
			"X-Trace":       []string{"trace-1"},
			"Authorization": []string{"ApiKey secret"},
		},
		Body: io.NopCloser(strings.NewReader("request-body")),
	})

	require.NoError(t, err)
	require.Equal(t, http.MethodPost, gotMethod)
	require.Equal(t, "/base/payments", gotPath)
	require.Equal(t, "trace=1", gotQuery)
	require.Equal(t, "trace-1", gotTrace)
	require.Empty(t, gotAuth)
	require.Equal(t, "request-body", gotBody)
	require.Equal(t, http.StatusAccepted, msg.StatusCode)
	require.Equal(t, "ok", msg.Headers.Get("X-Upstream"))
	responseBody, err := io.ReadAll(msg.Body)
	require.NoError(t, err)
	require.Equal(t, "accepted", string(responseBody))
}
