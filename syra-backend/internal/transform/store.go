package transform

import (
	"context"
	"errors"
	"sync"
)

var ErrTemplateNotFound = errors.New("transformation template not found")

type Store interface {
	Find(ctx context.Context, tenantID string, templateID string) (Template, error)
}

type InMemoryStore struct {
	mu        sync.RWMutex
	templates map[string]Template
}

func NewInMemoryStore(templates ...Template) *InMemoryStore {
	store := &InMemoryStore{templates: map[string]Template{}}
	for _, template := range templates {
		store.templates[key(template.TenantID, template.ID)] = template
	}
	return store
}

func (s *InMemoryStore) Find(ctx context.Context, tenantID string, templateID string) (Template, error) {
	if err := ctx.Err(); err != nil {
		return Template{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	template, ok := s.templates[key(tenantID, templateID)]
	if !ok {
		return Template{}, ErrTemplateNotFound
	}
	return template, nil
}

func key(tenantID string, templateID string) string {
	return tenantID + "/" + templateID
}
