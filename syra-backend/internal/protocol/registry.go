package protocol

import (
	"fmt"
	"strings"
	"sync"
)

type Registry struct {
	mu        sync.RWMutex
	protocols map[string]ProtocolAdapter
	upstreams map[string]UpstreamAdapter
}

func NewRegistry() *Registry {
	return &Registry{
		protocols: map[string]ProtocolAdapter{},
		upstreams: map[string]UpstreamAdapter{},
	}
}

func (r *Registry) RegisterProtocol(adapter ProtocolAdapter) error {
	if adapter == nil {
		return fmt.Errorf("protocol adapter is nil")
	}
	name := normalizeName(adapter.Name())
	if name == "" {
		return fmt.Errorf("protocol adapter name is empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.protocols[name] = adapter
	return nil
}

func (r *Registry) RegisterUpstream(adapter UpstreamAdapter) error {
	if adapter == nil {
		return fmt.Errorf("upstream adapter is nil")
	}
	name := normalizeName(adapter.Name())
	if name == "" {
		return fmt.Errorf("upstream adapter name is empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.upstreams[name] = adapter
	return nil
}

func (r *Registry) Protocol(name string) (ProtocolAdapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	adapter, ok := r.protocols[normalizeName(name)]
	return adapter, ok
}

func (r *Registry) Upstream(name string) (UpstreamAdapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	adapter, ok := r.upstreams[normalizeName(name)]
	return adapter, ok
}

func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
