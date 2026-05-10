package iso8583

import (
	"context"
	"errors"
	"sync"
)

var ErrProfileNotFound = errors.New("iso8583 profile not found")

type ProfileStore interface {
	Find(ctx context.Context, profileID string) (Profile, error)
}

type InMemoryProfileStore struct {
	mu       sync.RWMutex
	profiles map[string]Profile
}

func NewInMemoryProfileStore(profiles ...Profile) *InMemoryProfileStore {
	store := &InMemoryProfileStore{profiles: map[string]Profile{}}
	for _, profile := range profiles {
		store.profiles[profile.ID] = profile
	}
	return store
}

func (s *InMemoryProfileStore) Find(ctx context.Context, profileID string) (Profile, error) {
	if err := ctx.Err(); err != nil {
		return Profile{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	profile, ok := s.profiles[profileID]
	if !ok {
		return Profile{}, ErrProfileNotFound
	}
	return profile, nil
}
