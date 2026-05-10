package upstream

import (
	"context"
	"errors"
	"sync"
)

var ErrNotFound = errors.New("upstream not found")

type Store interface {
	Find(ctx context.Context, tenantID string, upstreamID string) (Upstream, error)
}

type InMemoryStore struct {
	mu        sync.RWMutex
	upstreams map[string]Upstream
}

func NewInMemoryStore(upstreams ...Upstream) *InMemoryStore {
	store := &InMemoryStore{upstreams: map[string]Upstream{}}
	for _, upstream := range upstreams {
		store.upstreams[key(upstream.TenantID, upstream.ID)] = upstream
	}
	return store
}

func (s *InMemoryStore) Find(ctx context.Context, tenantID string, upstreamID string) (Upstream, error) {
	if err := ctx.Err(); err != nil {
		return Upstream{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	upstream, ok := s.upstreams[key(tenantID, upstreamID)]
	if !ok {
		return Upstream{}, ErrNotFound
	}
	return upstream, nil
}

func key(tenantID string, upstreamID string) string {
	return tenantID + "/" + upstreamID
}
