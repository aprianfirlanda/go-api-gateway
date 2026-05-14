package protocol

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegistryReturnsRegisteredProtocolAdapter(t *testing.T) {
	registry := NewRegistry()
	adapter := stubProtocolAdapter{name: "rest"}

	require.NoError(t, registry.RegisterProtocol(adapter))

	got, ok := registry.Protocol("REST")
	require.True(t, ok)
	require.Equal(t, adapter, got)
}

func TestRegistryReturnsRegisteredUpstreamAdapter(t *testing.T) {
	registry := NewRegistry()
	adapter := stubUpstreamAdapter{name: "rest"}

	require.NoError(t, registry.RegisterUpstream(adapter))

	got, ok := registry.Upstream("REST")
	require.True(t, ok)
	require.Equal(t, adapter, got)
}

func TestRegistryRejectsNilAdapters(t *testing.T) {
	registry := NewRegistry()

	require.Error(t, registry.RegisterProtocol(nil))
	require.Error(t, registry.RegisterUpstream(nil))
}

type stubProtocolAdapter struct {
	name string
}

func (s stubProtocolAdapter) Name() string {
	return s.name
}

func (s stubProtocolAdapter) Decode(context.Context, InboundRequest) (CanonicalMessage, error) {
	return CanonicalMessage{}, nil
}

func (s stubProtocolAdapter) Encode(context.Context, CanonicalMessage) (OutboundResponse, error) {
	return OutboundResponse{}, nil
}

type stubUpstreamAdapter struct {
	name string
}

func (s stubUpstreamAdapter) Name() string {
	return s.name
}

func (s stubUpstreamAdapter) Call(context.Context, UpstreamTarget, CanonicalMessage) (CanonicalMessage, error) {
	return CanonicalMessage{}, nil
}
