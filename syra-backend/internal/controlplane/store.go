package controlplane

import (
	"context"
	"errors"
	"sync"
)

var ErrNotFound = errors.New("resource not found")

type Store struct {
	mu          sync.RWMutex
	tenants     map[string]Tenant
	products    map[string]APIProduct
	upstreams   map[string]Upstream
	routes      map[string]Route
	consumers   map[string]Consumer
	credentials map[string]Credential
	templates   map[string]TransformationTemplate
	auditEvents []AuditEvent
}

func NewStore() *Store {
	return &Store{
		tenants:     map[string]Tenant{},
		products:    map[string]APIProduct{},
		upstreams:   map[string]Upstream{},
		routes:      map[string]Route{},
		consumers:   map[string]Consumer{},
		credentials: map[string]Credential{},
		templates:   map[string]TransformationTemplate{},
	}
}

func (s *Store) CreateTenant(ctx context.Context, tenant Tenant) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tenants[tenant.ID] = tenant
	return nil
}

func (s *Store) ListTenants(ctx context.Context) ([]Tenant, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Tenant, 0, len(s.tenants))
	for _, value := range s.tenants {
		out = append(out, value)
	}
	return out, nil
}

func (s *Store) GetTenant(ctx context.Context, tenantID string) (Tenant, error) {
	if err := ctx.Err(); err != nil {
		return Tenant{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.tenants[tenantID]
	if !ok {
		return Tenant{}, ErrNotFound
	}
	return value, nil
}

func (s *Store) UpdateTenant(ctx context.Context, tenant Tenant) error {
	return s.CreateTenant(ctx, tenant)
}

func (s *Store) CreateAPIProduct(ctx context.Context, product APIProduct) error {
	return put(ctx, s, s.products, tenantKey(product.TenantID, product.ID), product)
}

func (s *Store) ListAPIProducts(ctx context.Context, tenantID string) ([]APIProduct, error) {
	return listTenantValues(ctx, s, s.products, tenantID)
}

func (s *Store) GetAPIProduct(ctx context.Context, tenantID, id string) (APIProduct, error) {
	return get(ctx, s, s.products, tenantKey(tenantID, id))
}

func (s *Store) UpdateAPIProduct(ctx context.Context, product APIProduct) error {
	return s.CreateAPIProduct(ctx, product)
}

func (s *Store) CreateUpstream(ctx context.Context, upstream Upstream) error {
	return put(ctx, s, s.upstreams, tenantKey(upstream.TenantID, upstream.ID), upstream)
}

func (s *Store) ListUpstreams(ctx context.Context, tenantID string) ([]Upstream, error) {
	return listTenantValues(ctx, s, s.upstreams, tenantID)
}

func (s *Store) GetUpstream(ctx context.Context, tenantID, id string) (Upstream, error) {
	return get(ctx, s, s.upstreams, tenantKey(tenantID, id))
}

func (s *Store) UpdateUpstream(ctx context.Context, upstream Upstream) error {
	return s.CreateUpstream(ctx, upstream)
}

func (s *Store) CreateRoute(ctx context.Context, route Route) error {
	return put(ctx, s, s.routes, tenantKey(route.TenantID, route.ID), route)
}

func (s *Store) ListRoutes(ctx context.Context, tenantID string) ([]Route, error) {
	return listTenantValues(ctx, s, s.routes, tenantID)
}

func (s *Store) GetRoute(ctx context.Context, tenantID, id string) (Route, error) {
	return get(ctx, s, s.routes, tenantKey(tenantID, id))
}

func (s *Store) UpdateRoute(ctx context.Context, route Route) error {
	return s.CreateRoute(ctx, route)
}

func (s *Store) CreateConsumer(ctx context.Context, consumer Consumer) error {
	return put(ctx, s, s.consumers, tenantKey(consumer.TenantID, consumer.ID), consumer)
}

func (s *Store) GetConsumer(ctx context.Context, tenantID, id string) (Consumer, error) {
	return get(ctx, s, s.consumers, tenantKey(tenantID, id))
}

func (s *Store) CreateCredential(ctx context.Context, credential Credential) error {
	return put(ctx, s, s.credentials, tenantKey(credential.TenantID, credential.ID), credential)
}

func (s *Store) GetCredential(ctx context.Context, tenantID, id string) (Credential, error) {
	return get(ctx, s, s.credentials, tenantKey(tenantID, id))
}

func (s *Store) UpdateCredential(ctx context.Context, credential Credential) error {
	return s.CreateCredential(ctx, credential)
}

func (s *Store) CreateTemplate(ctx context.Context, template TransformationTemplate) error {
	return put(ctx, s, s.templates, tenantKey(template.TenantID, template.ID), template)
}

func (s *Store) ListTemplates(ctx context.Context, tenantID string) ([]TransformationTemplate, error) {
	return listTenantValues(ctx, s, s.templates, tenantID)
}

func (s *Store) GetTemplate(ctx context.Context, tenantID, id string) (TransformationTemplate, error) {
	return get(ctx, s, s.templates, tenantKey(tenantID, id))
}

func (s *Store) UpdateTemplate(ctx context.Context, template TransformationTemplate) error {
	return s.CreateTemplate(ctx, template)
}

func (s *Store) AppendAudit(ctx context.Context, event AuditEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.auditEvents = append(s.auditEvents, event)
	return nil
}

func (s *Store) ListAuditEvents(ctx context.Context) ([]AuditEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]AuditEvent(nil), s.auditEvents...), nil
}

func put[T any](ctx context.Context, s *Store, values map[string]T, key string, value T) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	values[key] = value
	return nil
}

func get[T any](ctx context.Context, s *Store, values map[string]T, key string) (T, error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := values[key]
	if !ok {
		return zero, ErrNotFound
	}
	return value, nil
}

type tenantResource interface {
	GetTenantID() string
}

func listTenantValues[T tenantResource](ctx context.Context, s *Store, values map[string]T, tenantID string) ([]T, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []T{}
	for _, value := range values {
		if value.GetTenantID() == tenantID {
			out = append(out, value)
		}
	}
	return out, nil
}

func tenantKey(tenantID, id string) string {
	return tenantID + "/" + id
}

func (p APIProduct) GetTenantID() string             { return p.TenantID }
func (u Upstream) GetTenantID() string               { return u.TenantID }
func (r Route) GetTenantID() string                  { return r.TenantID }
func (t TransformationTemplate) GetTenantID() string { return t.TenantID }
