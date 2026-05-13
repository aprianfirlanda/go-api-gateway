package state

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestInMemoryStoreSetGetIncrementExpireDeleteAndCAS(t *testing.T) {
	store := NewInMemoryStore(Namespacer{Environment: "test", Version: "v1"})
	key := Key{TenantID: "tenant_1", Feature: "quota", Name: "counter"}

	require.NoError(t, store.Set(context.Background(), key, "10", 0))
	value, found, err := store.Get(context.Background(), key)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "10", value)

	next, err := store.Increment(context.Background(), key, 2, time.Second)
	require.NoError(t, err)
	require.Equal(t, int64(12), next)

	ok, err := store.CompareAndSet(context.Background(), key, "12", "15", 0)
	require.NoError(t, err)
	require.True(t, ok)

	value, found, err = store.Get(context.Background(), key)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "15", value)

	require.NoError(t, store.Expire(context.Background(), key, 20*time.Millisecond))
	time.Sleep(30 * time.Millisecond)
	_, found, err = store.Get(context.Background(), key)
	require.NoError(t, err)
	require.False(t, found)

	require.NoError(t, store.Set(context.Background(), key, "1", 0))
	require.NoError(t, store.Delete(context.Background(), key))
	_, found, err = store.Get(context.Background(), key)
	require.NoError(t, err)
	require.False(t, found)
}

func TestNamespacerFormat(t *testing.T) {
	full := Namespacer{Environment: "local", Version: "v3"}.Format(Key{
		TenantID: "tenant_2",
		Feature:  "replay",
		Name:     "nonce-1",
	})
	require.Equal(t, "local:tenant:tenant_2:feature:replay:version:v3:nonce-1", full)
}
