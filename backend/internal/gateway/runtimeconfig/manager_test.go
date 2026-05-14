package runtimeconfig

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"backend/internal/auth"
	"backend/internal/gateway/route"
	"backend/internal/gateway/upstream"
	"backend/internal/protocol/rest"
)

func TestManagerReloadAppliesValidSnapshot(t *testing.T) {
	routes := route.NewInMemoryRegistry()
	upstreams := upstream.NewInMemoryStore()
	credentials := auth.NewInMemoryCredentialStore()
	snapshot := Snapshot{
		Version: 1,
		Routes: []route.Route{{
			ID:               "route_1",
			TenantID:         "tenant_1",
			InboundProtocol:  rest.Name,
			OutboundProtocol: rest.Name,
			Host:             "api.example.test",
			Method:           "GET",
			Path:             "/accounts",
			UpstreamRef:      "upstream_1",
			Status:           route.StatusActive,
		}},
		Upstreams: []upstream.Upstream{{
			ID:       "upstream_1",
			TenantID: "tenant_1",
			Protocol: upstream.ProtocolREST,
			BaseURL:  "https://core.example.test",
		}},
		Credentials: []auth.APIKeyCredential{{
			ID:         "credential_1",
			TenantID:   "tenant_1",
			ConsumerID: "consumer_1",
			KeyPrefix:  "gw_live_test",
			SecretHash: "hash",
			Status:     auth.StatusActive,
		}},
	}
	manager := NewManager(StaticSource{Snapshot: snapshot}, Applier{
		Routes:      routes,
		Upstreams:   upstreams,
		Credentials: credentials,
	}, slog.Default())

	require.NoError(t, manager.Reload(context.Background()))
	loadedRoute, err := routes.Match(context.Background(), route.MatchRequest{TenantID: "tenant_1", Host: "api.example.test", Method: "GET", Path: "/accounts"})
	require.NoError(t, err)
	require.Equal(t, "route_1", loadedRoute.ID)
	loadedUpstream, err := upstreams.Find(context.Background(), "tenant_1", "upstream_1")
	require.NoError(t, err)
	require.Equal(t, "https://core.example.test", loadedUpstream.BaseURL)
	loadedCredential, err := credentials.FindByPrefix(context.Background(), "gw_live_test")
	require.NoError(t, err)
	require.Equal(t, "credential_1", loadedCredential.ID)
}

func TestManagerRejectsInvalidSnapshotAndKeepsLastKnownGood(t *testing.T) {
	routes := route.NewInMemoryRegistry()
	upstreams := upstream.NewInMemoryStore()
	valid := Snapshot{
		Version: 1,
		Routes: []route.Route{{
			ID:               "route_good",
			TenantID:         "tenant_1",
			InboundProtocol:  rest.Name,
			OutboundProtocol: rest.Name,
			Host:             "api.example.test",
			Method:           "GET",
			Path:             "/accounts",
			UpstreamRef:      "upstream_1",
			Status:           route.StatusActive,
		}},
		Upstreams: []upstream.Upstream{{ID: "upstream_1", TenantID: "tenant_1", Protocol: upstream.ProtocolREST}},
	}
	manager := NewManager(StaticSource{Snapshot: valid}, Applier{Routes: routes, Upstreams: upstreams}, slog.Default())
	require.NoError(t, manager.Reload(context.Background()))

	manager.source = StaticSource{Snapshot: Snapshot{
		Version: 2,
		Routes: []route.Route{{
			ID:               "route_bad",
			TenantID:         "tenant_1",
			InboundProtocol:  rest.Name,
			OutboundProtocol: rest.Name,
			Host:             "api.example.test",
			Method:           "GET",
			Path:             "/accounts",
			UpstreamRef:      "missing_upstream",
			Status:           route.StatusActive,
		}},
	}}
	require.Error(t, manager.Reload(context.Background()))

	loadedRoute, err := routes.Match(context.Background(), route.MatchRequest{TenantID: "tenant_1", Host: "api.example.test", Method: "GET", Path: "/accounts"})
	require.NoError(t, err)
	require.Equal(t, "route_good", loadedRoute.ID)
	require.Equal(t, int64(1), manager.Current().Version)
	require.NotNil(t, manager.LastError())
	_, rejects := manager.Stats()
	require.Equal(t, int64(1), rejects)
}
