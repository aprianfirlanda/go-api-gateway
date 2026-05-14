package upstream

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInMemoryStoreFindsTenantUpstream(t *testing.T) {
	store := NewInMemoryStore(Upstream{
		ID:       "upstream_1",
		TenantID: "tenant_1",
		Protocol: ProtocolREST,
		BaseURL:  "http://example.test",
	})

	got, err := store.Find(context.Background(), "tenant_1", "upstream_1")

	require.NoError(t, err)
	require.Equal(t, "http://example.test", got.BaseURL)
}

func TestInMemoryStoreDoesNotCrossTenantFind(t *testing.T) {
	store := NewInMemoryStore(Upstream{
		ID:       "upstream_1",
		TenantID: "tenant_1",
		Protocol: ProtocolREST,
		BaseURL:  "http://example.test",
	})

	_, err := store.Find(context.Background(), "tenant_2", "upstream_1")

	require.ErrorIs(t, err, ErrNotFound)
}
