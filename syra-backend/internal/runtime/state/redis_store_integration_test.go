package state

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"syra-backend/internal/observability"
	storageredis "syra-backend/internal/storage/redis"
)

func TestRedisStoreIntegration(t *testing.T) {
	ctx := context.Background()
	addr := startRedisContainer(t, ctx)
	client, err := storageredis.Open(ctx, storageredis.Config{Addr: addr, Timeout: 2 * time.Second}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	store := NewRedisStore(client, Namespacer{Environment: "it", Version: "v1"}, observability.NewMetrics(), nil)
	key := Key{TenantID: "tenant_1", Feature: "ratelimit", Name: "counter"}

	require.NoError(t, store.Set(ctx, key, "5", 0))
	value, found, err := store.Get(ctx, key)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "5", value)

	next, err := store.Increment(ctx, key, 3, 50*time.Millisecond)
	require.NoError(t, err)
	require.Equal(t, int64(8), next)

	ok, err := store.CompareAndSet(ctx, key, "8", "9", time.Second)
	require.NoError(t, err)
	require.True(t, ok)
	value, found, err = store.Get(ctx, key)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "9", value)

	require.NoError(t, store.Expire(ctx, key, 20*time.Millisecond))
	time.Sleep(30 * time.Millisecond)
	_, found, err = store.Get(ctx, key)
	require.NoError(t, err)
	require.False(t, found)

	require.NoError(t, store.Set(ctx, key, "1", 0))
	require.NoError(t, store.Delete(ctx, key))
	_, found, err = store.Get(ctx, key)
	require.NoError(t, err)
	require.False(t, found)
}

func startRedisContainer(t *testing.T, ctx context.Context) string {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Skipf("redis testcontainer unavailable: %v", recovered)
		}
	}()
	req := testcontainers.ContainerRequest{
		Image:        "redis:8-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForListeningPort("6379/tcp"),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Skipf("redis testcontainer unavailable: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil && !errors.Is(err, context.Canceled) {
			t.Logf("terminate redis container: %v", err)
		}
	})
	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "6379")
	require.NoError(t, err)
	return host + ":" + port.Port()
}
