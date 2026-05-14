package policy

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"backend/internal/runtime/state"
)

func TestDistributedRateLimiterFixedWindowSharedCounter(t *testing.T) {
	store := state.NewInMemoryStore(state.Namespacer{Environment: "test", Version: "v1"})
	limiterA := NewDistributedRateLimiter(store)
	limiterB := NewDistributedRateLimiter(store)
	cfg := RateLimitConfig{LimitCount: 2, WindowSeconds: 60, Algorithm: "fixed_window"}
	now := time.Date(2026, 5, 13, 10, 0, 0, 0, time.UTC)

	allowed, err := limiterA.Allow(context.Background(), "tenant:consumer:route", now, cfg)
	require.NoError(t, err)
	require.True(t, allowed)
	allowed, err = limiterB.Allow(context.Background(), "tenant:consumer:route", now, cfg)
	require.NoError(t, err)
	require.True(t, allowed)
	allowed, err = limiterA.Allow(context.Background(), "tenant:consumer:route", now, cfg)
	require.NoError(t, err)
	require.False(t, allowed)
}

func TestDistributedQuotaCheckerTenantScopedPeriodAware(t *testing.T) {
	store := state.NewInMemoryStore(state.Namespacer{Environment: "test", Version: "v1"})
	checker := NewDistributedQuotaChecker(store)
	cfg := QuotaConfig{QuotaCount: 2, Period: "monthly", ExceededBehavior: "block"}

	reqA := Request{TenantID: "tenant_a", ConsumerID: "consumer_1", RouteID: "route_1", Now: time.Date(2026, 5, 13, 10, 0, 0, 0, time.UTC)}
	reqB := Request{TenantID: "tenant_b", ConsumerID: "consumer_1", RouteID: "route_1", Now: reqA.Now}

	allow, err := checker.Allow(context.Background(), reqA, cfg)
	require.NoError(t, err)
	require.True(t, allow)
	allow, err = checker.Allow(context.Background(), reqA, cfg)
	require.NoError(t, err)
	require.True(t, allow)
	allow, err = checker.Allow(context.Background(), reqA, cfg)
	require.NoError(t, err)
	require.False(t, allow)

	allow, err = checker.Allow(context.Background(), reqB, cfg)
	require.NoError(t, err)
	require.True(t, allow)

	reqA.Now = reqA.Now.AddDate(0, 1, 0)
	allow, err = checker.Allow(context.Background(), reqA, cfg)
	require.NoError(t, err)
	require.True(t, allow)
}
