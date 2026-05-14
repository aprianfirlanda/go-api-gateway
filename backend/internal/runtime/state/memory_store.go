package state

import (
	"context"
	"strconv"
	"sync"
	"time"
)

type memoryEntry struct {
	value     string
	expiresAt time.Time
}

type InMemoryStore struct {
	mu         sync.RWMutex
	namespacer Namespacer
	values     map[string]memoryEntry
}

func NewInMemoryStore(namespacer Namespacer) *InMemoryStore {
	return &InMemoryStore{
		namespacer: namespacer,
		values:     map[string]memoryEntry{},
	}
}

func (s *InMemoryStore) Set(ctx context.Context, key Key, value string, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[s.namespacer.Format(key)] = memoryEntry{value: value, expiresAt: expiry(ttl)}
	return nil
}

func (s *InMemoryStore) Get(ctx context.Context, key Key) (string, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getLocked(s.namespacer.Format(key))
}

func (s *InMemoryStore) Increment(ctx context.Context, key Key, delta int64, ttl time.Duration) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fullKey := s.namespacer.Format(key)
	raw, ok, _ := s.getLocked(fullKey)
	var current int64
	if ok {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return 0, err
		}
		current = parsed
	}
	current += delta
	expiresAt := time.Time{}
	if ok {
		expiresAt = s.values[fullKey].expiresAt
	} else {
		expiresAt = expiry(ttl)
	}
	s.values[fullKey] = memoryEntry{value: strconv.FormatInt(current, 10), expiresAt: expiresAt}
	return current, nil
}

func (s *InMemoryStore) Expire(ctx context.Context, key Key, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fullKey := s.namespacer.Format(key)
	entry, ok := s.values[fullKey]
	if !ok || isExpired(entry.expiresAt) {
		delete(s.values, fullKey)
		return nil
	}
	entry.expiresAt = expiry(ttl)
	s.values[fullKey] = entry
	return nil
}

func (s *InMemoryStore) Delete(ctx context.Context, key Key) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.values, s.namespacer.Format(key))
	return nil
}

func (s *InMemoryStore) CompareAndSet(ctx context.Context, key Key, expected, next string, ttl time.Duration) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fullKey := s.namespacer.Format(key)
	current, ok, _ := s.getLocked(fullKey)
	if !ok {
		current = ""
	}
	if current != expected {
		return false, nil
	}
	expiresAt := expiry(ttl)
	if ttl <= 0 {
		if entry, exists := s.values[fullKey]; exists {
			expiresAt = entry.expiresAt
		}
	}
	s.values[fullKey] = memoryEntry{value: next, expiresAt: expiresAt}
	return true, nil
}

func (s *InMemoryStore) getLocked(fullKey string) (string, bool, error) {
	entry, ok := s.values[fullKey]
	if !ok {
		return "", false, nil
	}
	if isExpired(entry.expiresAt) {
		delete(s.values, fullKey)
		return "", false, nil
	}
	return entry.value, true, nil
}

func expiry(ttl time.Duration) time.Time {
	if ttl <= 0 {
		return time.Time{}
	}
	return time.Now().UTC().Add(ttl)
}

func isExpired(expiresAt time.Time) bool {
	if expiresAt.IsZero() {
		return false
	}
	return time.Now().UTC().After(expiresAt)
}
