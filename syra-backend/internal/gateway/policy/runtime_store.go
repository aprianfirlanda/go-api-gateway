package policy

import "sync"

type RateLimitConfig struct {
	ID            string
	TenantID      string
	Scope         string
	LimitCount    int
	WindowSeconds int
	BurstCount    int
	Algorithm     string
	Status        string
}

type QuotaConfig struct {
	ID               string
	TenantID         string
	Scope            string
	Period           string
	QuotaCount       int64
	ExceededBehavior string
	Status           string
}

type APIProductBinding struct {
	ID                string
	TenantID          string
	RateLimitPolicyID string
	QuotaPolicyID     string
}

type RuntimePolicyStore struct {
	mu         sync.RWMutex
	rateLimits map[string]RateLimitConfig
	quotas     map[string]QuotaConfig
	products   map[string]APIProductBinding
}

func NewRuntimePolicyStore() *RuntimePolicyStore {
	return &RuntimePolicyStore{
		rateLimits: map[string]RateLimitConfig{},
		quotas:     map[string]QuotaConfig{},
		products:   map[string]APIProductBinding{},
	}
}

func (s *RuntimePolicyStore) ReplaceRateLimits(items ...RateLimitConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := map[string]RateLimitConfig{}
	for _, item := range items {
		next[item.TenantID+"/"+item.ID] = item
	}
	s.rateLimits = next
}

func (s *RuntimePolicyStore) ReplaceQuotas(items ...QuotaConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := map[string]QuotaConfig{}
	for _, item := range items {
		next[item.TenantID+"/"+item.ID] = item
	}
	s.quotas = next
}

func (s *RuntimePolicyStore) ReplaceProducts(items ...APIProductBinding) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := map[string]APIProductBinding{}
	for _, item := range items {
		next[item.TenantID+"/"+item.ID] = item
	}
	s.products = next
}

func (s *RuntimePolicyStore) RateLimit(tenantID, id string) (RateLimitConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.rateLimits[tenantID+"/"+id]
	return item, ok
}

func (s *RuntimePolicyStore) Quota(tenantID, id string) (QuotaConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.quotas[tenantID+"/"+id]
	return item, ok
}

func (s *RuntimePolicyStore) Product(tenantID, id string) (APIProductBinding, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.products[tenantID+"/"+id]
	return item, ok
}
