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
	store.Replace(upstreams...)
	return store
}

func (s *InMemoryStore) Replace(upstreams ...Upstream) {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := map[string]Upstream{}
	for _, upstream := range upstreams {
		next[key(upstream.TenantID, upstream.ID)] = upstream
	}
	s.upstreams = next
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
