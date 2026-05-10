package policy

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPipelineStopsInOrder(t *testing.T) {
	var calls []string
	pipeline := NewPipeline(
		stubPolicy{name: "first", calls: &calls},
		stubPolicy{name: "second", calls: &calls, err: ErrBlockedIP},
		stubPolicy{name: "third", calls: &calls},
	)

	err := pipeline.Evaluate(context.Background(), Request{})

	require.ErrorIs(t, err, ErrBlockedIP)
	require.Equal(t, []string{"first", "second"}, calls)
}

func TestIPAllowlistBlocksRemoteAddress(t *testing.T) {
	p, err := NewIPAllowlistPolicy("10.0.0.0/8")
	require.NoError(t, err)

	err = p.Evaluate(context.Background(), Request{RemoteAddr: "192.168.1.10:1234"})

	require.ErrorIs(t, err, ErrBlockedIP)
}

func TestRequestSizeLimit(t *testing.T) {
	p := NewRequestSizeLimitPolicy(10)

	require.NoError(t, p.Evaluate(context.Background(), Request{SizeBytes: 10}))
	require.ErrorIs(t, p.Evaluate(context.Background(), Request{SizeBytes: 11}), ErrRequestTooLarge)
}

func TestInMemoryRateLimiter(t *testing.T) {
	limiter := NewInMemoryRateLimiter(1, time.Minute)
	p := NewRateLimitPolicy(limiter)
	req := Request{TenantID: "t1", ConsumerID: "c1", RouteID: "r1", Now: time.Unix(1, 0)}

	require.NoError(t, p.Evaluate(context.Background(), req))
	require.ErrorIs(t, p.Evaluate(context.Background(), req), ErrRateLimited)

	req.Now = req.Now.Add(time.Minute)
	require.NoError(t, p.Evaluate(context.Background(), req))
}

func TestQuotaPolicy(t *testing.T) {
	p := NewQuotaPolicy(stubQuota{allow: false})

	err := p.Evaluate(context.Background(), Request{})

	require.ErrorIs(t, err, ErrQuotaExceeded)
}

type stubPolicy struct {
	name  string
	calls *[]string
	err   error
}

func (s stubPolicy) Name() string { return s.name }

func (s stubPolicy) Evaluate(context.Context, Request) error {
	*s.calls = append(*s.calls, s.name)
	if s.err != nil {
		return s.err
	}
	return nil
}

type stubQuota struct {
	allow bool
	err   error
}

func (s stubQuota) Allow(context.Context, Request) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	if errors.Is(s.err, context.Canceled) {
		return false, s.err
	}
	return s.allow, nil
}
