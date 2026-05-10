package route

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
)

var ErrNotFound = errors.New("route not found")

type Registry interface {
	Match(ctx context.Context, req MatchRequest) (Route, error)
}

type MatchRequest struct {
	TenantID string
	Host     string
	Method   string
	Path     string
}

type InMemoryRegistry struct {
	mu     sync.RWMutex
	routes []Route
}

func NewInMemoryRegistry(routes ...Route) *InMemoryRegistry {
	registry := &InMemoryRegistry{}
	registry.Replace(routes...)
	return registry
}

func (r *InMemoryRegistry) Replace(routes ...Route) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.routes = append([]Route(nil), routes...)
}

func (r *InMemoryRegistry) Match(ctx context.Context, req MatchRequest) (Route, error) {
	if err := ctx.Err(); err != nil {
		return Route{}, err
	}

	host := normalizeHost(req.Host)
	method := strings.ToUpper(req.Method)

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, candidate := range r.routes {
		if candidate.Status != StatusActive {
			continue
		}
		if candidate.TenantID != req.TenantID {
			continue
		}
		if normalizeHost(candidate.Host) != host {
			continue
		}
		if strings.ToUpper(candidate.Method) != method {
			continue
		}
		if candidate.Path != req.Path {
			continue
		}

		return candidate, nil
	}

	return Route{}, ErrNotFound
}

func normalizeHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return host
	}

	withoutPort, _, err := net.SplitHostPort(host)
	if err == nil {
		return withoutPort
	}

	return host
}
