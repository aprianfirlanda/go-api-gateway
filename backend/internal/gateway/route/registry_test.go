package route

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInMemoryRegistryMatchesTenantHostMethodAndPath(t *testing.T) {
	registry := NewInMemoryRegistry(Route{
		ID:           "route_1",
		TenantID:     "tenant_1",
		APIProductID: "product_1",
		Host:         "api.example.test",
		Method:       "POST",
		Path:         "/payments",
		Status:       StatusActive,
	})

	got, err := registry.Match(context.Background(), MatchRequest{
		TenantID: "tenant_1",
		Host:     "api.example.test:443",
		Method:   "post",
		Path:     "/payments",
	})

	require.NoError(t, err)
	require.Equal(t, "route_1", got.ID)
}

func TestInMemoryRegistryDoesNotCrossTenantMatch(t *testing.T) {
	registry := NewInMemoryRegistry(Route{
		ID:       "route_1",
		TenantID: "tenant_1",
		Host:     "api.example.test",
		Method:   "GET",
		Path:     "/accounts",
		Status:   StatusActive,
	})

	_, err := registry.Match(context.Background(), MatchRequest{
		TenantID: "tenant_2",
		Host:     "api.example.test",
		Method:   "GET",
		Path:     "/accounts",
	})

	require.ErrorIs(t, err, ErrNotFound)
}

func TestInMemoryRegistryDoesNotMatchDisabledRoute(t *testing.T) {
	registry := NewInMemoryRegistry(Route{
		ID:       "route_1",
		TenantID: "tenant_1",
		Host:     "api.example.test",
		Method:   "GET",
		Path:     "/accounts",
		Status:   StatusDisabled,
	})

	_, err := registry.Match(context.Background(), MatchRequest{
		TenantID: "tenant_1",
		Host:     "api.example.test",
		Method:   "GET",
		Path:     "/accounts",
	})

	require.ErrorIs(t, err, ErrNotFound)
}
