package runtimeconfig

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPingerReadyWhenConfigLoadedAndNoErrors(t *testing.T) {
	manager := NewManager(nil, Applier{}, nil)
	manager.current = Snapshot{Version: 1}
	pinger := NewPinger(manager)
	require.NoError(t, pinger.Ping(context.Background()))
}

func TestPingerNotReadyWhenConfigMissing(t *testing.T) {
	pinger := NewPinger(NewManager(nil, Applier{}, nil))
	err := pinger.Ping(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "has not been loaded")
}

func TestPingerNotReadyWhenLastReloadFailed(t *testing.T) {
	manager := NewManager(nil, Applier{}, nil)
	manager.current = Snapshot{Version: 1}
	manager.lastErr = errors.New("load failed")
	pinger := NewPinger(manager)
	err := pinger.Ping(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "last reload failed")
}
