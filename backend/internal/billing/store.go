package billing

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"sync"
	"time"
)

type UsageEventStore interface {
	Save(ctx context.Context, event UsageEvent) error
	List(ctx context.Context, filter UsageEventFilter) ([]UsageEvent, error)
	ListPage(ctx context.Context, filter UsageEventFilter, limit int, cursor string) (UsageEventPage, error)
}

type UsageEventFilter struct {
	TenantID       string
	RouteID        string
	ConsumerID     string
	Status         string
	SourceProtocol string
	TargetProtocol string
	Billable       *bool
	From           *time.Time
	To             *time.Time
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
		if !matchesFilter(event, filter) {
			continue
		}
		events = append(events, event)
	}
	return events, nil
}

func (s *InMemoryUsageEventStore) ListPage(ctx context.Context, filter UsageEventFilter, limit int, cursor string) (UsageEventPage, error) {
	items, err := s.List(ctx, filter)
	if err != nil {
		return UsageEventPage{}, err
	}
	offset, err := decodeCursor(cursor)
	if err != nil {
		return UsageEventPage{}, err
	}
	if offset < 0 || offset >= len(items) {
		return UsageEventPage{Data: []UsageEvent{}}, nil
	}
	if limit <= 0 {
		limit = 50
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	page := UsageEventPage{
		Data: append([]UsageEvent(nil), items[offset:end]...),
	}
	if end < len(items) {
		next := encodeCursor(end)
		page.NextCursor = &next
	}
	return page, nil
}

func matchesFilter(event UsageEvent, filter UsageEventFilter) bool {
	if filter.TenantID != "" && event.TenantID != filter.TenantID {
		return false
	}
	if filter.RouteID != "" && event.RouteID != filter.RouteID {
		return false
	}
	if filter.ConsumerID != "" && event.ConsumerID != filter.ConsumerID {
		return false
	}
	if filter.Status != "" && event.Status != filter.Status {
		return false
	}
	if filter.SourceProtocol != "" && event.SourceProtocol != filter.SourceProtocol {
		return false
	}
	if filter.TargetProtocol != "" && event.TargetProtocol != filter.TargetProtocol {
		return false
	}
	if filter.Billable != nil && event.Billable != *filter.Billable {
		return false
	}
	if filter.From != nil && event.OccurredAt.Before(*filter.From) {
		return false
	}
	if filter.To != nil && !event.OccurredAt.Before(*filter.To) {
		return false
	}
	return true
}

func decodeCursor(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid cursor")
	}
	offset, err := strconv.Atoi(string(decoded))
	if err != nil {
		return 0, fmt.Errorf("invalid cursor")
	}
	return offset, nil
}

func encodeCursor(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}
