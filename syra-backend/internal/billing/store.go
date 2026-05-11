package billing

import (
	"context"
	"sync"
	"time"
)

type UsageEventStore interface {
	Save(ctx context.Context, event UsageEvent) error
	List(ctx context.Context, filter UsageEventFilter) ([]UsageEvent, error)
}

type UsageEventFilter struct {
	TenantID string
	From     *time.Time
	To       *time.Time
}

type InMemoryUsageEventStore struct {
	mu     sync.RWMutex
	events []UsageEvent
}

func NewInMemoryUsageEventStore(events ...UsageEvent) *InMemoryUsageEventStore {
	store := &InMemoryUsageEventStore{}
	store.events = append(store.events, events...)
	return store
}

func (s *InMemoryUsageEventStore) Save(ctx context.Context, event UsageEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}

func (s *InMemoryUsageEventStore) List(ctx context.Context, filter UsageEventFilter) ([]UsageEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	events := make([]UsageEvent, 0, len(s.events))
	for _, event := range s.events {
		if filter.TenantID != "" && event.TenantID != filter.TenantID {
			continue
		}
		if filter.From != nil && event.OccurredAt.Before(*filter.From) {
			continue
		}
		if filter.To != nil && !event.OccurredAt.Before(*filter.To) {
			continue
		}
		events = append(events, event)
	}
	return events, nil
}
