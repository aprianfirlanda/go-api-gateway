package health

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubPinger struct {
	err error
}

func (s stubPinger) Ping(ctx context.Context) error {
	return s.err
}

func TestReadinessWithoutPingerIsHealthy(t *testing.T) {
	service := NewService(nil)
	require.NoError(t, service.Readiness(context.Background()))
}

func TestReadinessReturnsPingerError(t *testing.T) {
	expected := errors.New("unavailable")
	service := NewService(NewMultiPinger(stubPinger{err: expected}))
	require.ErrorIs(t, service.Readiness(context.Background()), expected)
}
