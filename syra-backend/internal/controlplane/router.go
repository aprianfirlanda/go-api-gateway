package controlplane

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"syra-backend/internal/auth"
	"syra-backend/pkg/ids"
)

const actorPlatformAdmin = "platform_admin"

type RouterConfig struct {
	AdminToken string
	Store      Repository
	Now        func() time.Time
}

type Handler struct {
	store      Repository
	adminToken string
	now        func() time.Time
}

func NewRouter(cfg RouterConfig) http.Handler {
	if cfg.Store == nil {
		cfg.Store = NewStore()
	}
	if cfg.AdminToken == "" {
		cfg.AdminToken = "dev-admin-token"
	}
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now().UTC() }
	}
	h := &Handler{store: cfg.Store, adminToken: cfg.AdminToken, now: cfg.Now}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(h.adminAuth)
	r.Route("/admin/v1", func(r chi.Router) {
		r.Post("/tenants", h.createTenant)
		r.Get("/tenants", h.listTenants)
		r.Get("/tenants/{tenantId}", h.getTenant)
		r.Patch("/tenants/{tenantId}", h.updateTenant)
		r.Route("/tenants/{tenantId}", func(r chi.Router) {
			r.Use(h.requireTenant)
			r.Post("/api-products", h.createAPIProduct)
			r.Get("/api-products", h.listAPIProducts)
			r.Get("/api-products/{apiProductId}", h.getAPIProduct)
			r.Patch("/api-products/{apiProductId}", h.updateAPIProduct)

			r.Post("/upstreams", h.createUpstream)
			r.Get("/upstreams", h.listUpstreams)
			r.Patch("/upstreams/{upstreamId}", h.updateUpstream)

			r.Post("/routes", h.createRoute)
			r.Get("/routes", h.listRoutes)
			r.Get("/routes/{routeId}", h.getRoute)
			r.Patch("/routes/{routeId}", h.updateRoute)
			r.Post("/routes/{routeId}/publish", h.publishRoute)
			r.Post("/routes/{routeId}/disable", h.disableRoute)

			r.Post("/consumers", h.createConsumer)
			r.Post("/consumers/{consumerId}/credentials", h.createCredential)
			r.Post("/credentials/{credentialId}/rotate", h.rotateCredential)
			r.Post("/credentials/{credentialId}/revoke", h.revokeCredential)

			r.Post("/transformation-templates", h.createTemplate)
			r.Get("/transformation-templates", h.listTemplates)
			r.Post("/transformation-templates/{templateId}/publish", h.publishTemplate)
		})
	})
	return r
}

func (h *Handler) adminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		value := r.Header.Get("Authorization")
		if !strings.HasPrefix(value, prefix) || strings.TrimPrefix(value, prefix) != h.adminToken {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", "Admin authentication required")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), actorContextKey{}, actorPlatformAdmin)))
	})
}

func (h *Handler) requireTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := h.store.GetTenant(r.Context(), chi.URLParam(r, "tenantId")); err != nil {
			writeStoreError(w, r, err)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type actorContextKey struct{}

func actorFromContext(ctx context.Context) string {
	actor, _ := ctx.Value(actorContextKey{}).(string)
	if actor == "" {
		return "unknown"
	}
	return actor
}

func (h *Handler) createTenant(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name          string         `json:"name"`
		Slug          string         `json:"slug"`
		BillingPlanID string         `json:"billingPlanId"`
		Metadata      map[string]any `json:"metadata"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Name == "" || req.Slug == "" {
		writeError(w, r, http.StatusBadRequest, "validation_error", "name and slug are required")
		return
	}
	now := h.now()
	tenant := Tenant{ID: ids.New(), Name: req.Name, Slug: req.Slug, Status: StatusActive, BillingPlanID: req.BillingPlanID, Metadata: req.Metadata, CreatedAt: now, UpdatedAt: now}
	if err := h.store.CreateTenant(r.Context(), tenant); err != nil {
		writeInternal(w, r, err)
		return
	}
	h.audit(r.Context(), tenant.ID, "tenant.create", "tenant", tenant.ID)
	writeJSON(w, http.StatusCreated, tenant)
}

func (h *Handler) listTenants(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListTenants(r.Context())
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[Tenant]{Data: items})
}

func (h *Handler) getTenant(w http.ResponseWriter, r *http.Request) {
	tenant, err := h.store.GetTenant(r.Context(), chi.URLParam(r, "tenantId"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, tenant)
}

func (h *Handler) updateTenant(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantId")
	tenant, err := h.store.GetTenant(r.Context(), tenantID)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	var req struct {
		Name          *string `json:"name"`
		Status        *string `json:"status"`
		BillingPlanID *string `json:"billingPlanId"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Name != nil {
		tenant.Name = *req.Name
	}
	if req.Status != nil {
		tenant.Status = *req.Status
	}
	if req.BillingPlanID != nil {
		tenant.BillingPlanID = *req.BillingPlanID
	}
	tenant.UpdatedAt = h.now()
	if err := h.store.UpdateTenant(r.Context(), tenant); err != nil {
		writeInternal(w, r, err)
		return
	}
	h.audit(r.Context(), tenant.ID, "tenant.update", "tenant", tenant.ID)
	writeJSON(w, http.StatusOK, tenant)
}

func (h *Handler) createAPIProduct(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantId")
	var req struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Description string `json:"description"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Name == "" || req.Slug == "" {
		writeError(w, r, http.StatusBadRequest, "validation_error", "name and slug are required")
		return
	}
	now := h.now()
	product := APIProduct{ID: ids.New(), TenantID: tenantID, Name: req.Name, Slug: req.Slug, Description: req.Description, Status: StatusActive, CreatedAt: now, UpdatedAt: now}
	if err := h.store.CreateAPIProduct(r.Context(), product); err != nil {
		writeInternal(w, r, err)
		return
	}
	h.audit(r.Context(), tenantID, "api_product.create", "api_product", product.ID)
	writeJSON(w, http.StatusCreated, product)
}

func (h *Handler) listAPIProducts(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListAPIProducts(r.Context(), chi.URLParam(r, "tenantId"))
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[APIProduct]{Data: items})
}

func (h *Handler) getAPIProduct(w http.ResponseWriter, r *http.Request) {
	item, err := h.store.GetAPIProduct(r.Context(), chi.URLParam(r, "tenantId"), chi.URLParam(r, "apiProductId"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) updateAPIProduct(w http.ResponseWriter, r *http.Request) {
	tenantID, id := chi.URLParam(r, "tenantId"), chi.URLParam(r, "apiProductId")
	item, err := h.store.GetAPIProduct(r.Context(), tenantID, id)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Status      *string `json:"status"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Name != nil {
		item.Name = *req.Name
	}
	if req.Description != nil {
		item.Description = *req.Description
	}
	if req.Status != nil {
		item.Status = *req.Status
	}
	item.UpdatedAt = h.now()
	if err := h.store.UpdateAPIProduct(r.Context(), item); err != nil {
		writeInternal(w, r, err)
		return
	}
	h.audit(r.Context(), tenantID, "api_product.update", "api_product", item.ID)
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) createUpstream(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantId")
	var req struct {
		Name      string          `json:"name"`
		Protocol  string          `json:"protocol"`
		Config    json.RawMessage `json:"config"`
		SecretRef *string         `json:"secretRef"`
		Status    string          `json:"status"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Name == "" || req.Protocol == "" {
		writeError(w, r, http.StatusBadRequest, "validation_error", "name and protocol are required")
		return
	}
	if req.Status == "" {
		req.Status = StatusActive
	}
	now := h.now()
	item := Upstream{ID: ids.New(), TenantID: tenantID, Name: req.Name, Protocol: req.Protocol, Config: req.Config, SecretRef: req.SecretRef, Status: req.Status, CreatedAt: now, UpdatedAt: now}
	if err := h.store.CreateUpstream(r.Context(), item); err != nil {
		writeInternal(w, r, err)
		return
	}
	h.audit(r.Context(), tenantID, "upstream.create", "upstream", item.ID)
	writeJSON(w, http.StatusCreated, item)
}

func (h *Handler) listUpstreams(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListUpstreams(r.Context(), chi.URLParam(r, "tenantId"))
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[Upstream]{Data: items})
}

func (h *Handler) updateUpstream(w http.ResponseWriter, r *http.Request) {
	tenantID, id := chi.URLParam(r, "tenantId"), chi.URLParam(r, "upstreamId")
	item, err := h.store.GetUpstream(r.Context(), tenantID, id)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	var req struct {
		Name      *string          `json:"name"`
		Protocol  *string          `json:"protocol"`
		Config    *json.RawMessage `json:"config"`
		SecretRef *string          `json:"secretRef"`
		Status    *string          `json:"status"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Name != nil {
		item.Name = *req.Name
	}
	if req.Protocol != nil {
		item.Protocol = *req.Protocol
	}
	if req.Config != nil {
		item.Config = *req.Config
	}
	if req.SecretRef != nil {
		item.SecretRef = req.SecretRef
	}
	if req.Status != nil {
		item.Status = *req.Status
	}
	item.UpdatedAt = h.now()
	if err := h.store.UpdateUpstream(r.Context(), item); err != nil {
		writeInternal(w, r, err)
		return
	}
	h.audit(r.Context(), tenantID, "upstream.update", "upstream", item.ID)
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) createRoute(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantId")
	var req Route
	if !decode(w, r, &req) {
		return
	}
	if req.APIProductID == "" || req.Name == "" || req.InboundProtocol == "" || req.OutboundProtocol == "" || req.Host == "" || req.Method == "" || req.Path == "" || req.UpstreamID == "" {
		writeError(w, r, http.StatusBadRequest, "validation_error", "apiProductId, name, protocols, host, method, path, and upstreamId are required")
		return
	}
	now := h.now()
	req.ID = ids.New()
	req.TenantID = tenantID
	if req.Status == "" {
		req.Status = StatusDraft
	}
	if req.ReplayWindowSec == 0 {
		req.ReplayWindowSec = 300
	}
	if req.IdempotencyTTLSec == 0 {
		req.IdempotencyTTLSec = 86400
	}
	req.CreatedAt = now
	req.UpdatedAt = now
	if err := h.store.CreateRoute(r.Context(), req); err != nil {
		writeInternal(w, r, err)
		return
	}
	h.audit(r.Context(), tenantID, "route.create", "route", req.ID)
	writeJSON(w, http.StatusCreated, req)
}

func (h *Handler) listRoutes(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListRoutes(r.Context(), chi.URLParam(r, "tenantId"))
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	status := r.URL.Query().Get("status")
	if status != "" {
		filtered := []Route{}
		for _, item := range items {
			if item.Status == status {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	writeJSON(w, http.StatusOK, listResponse[Route]{Data: items})
}

func (h *Handler) getRoute(w http.ResponseWriter, r *http.Request) {
	item, err := h.store.GetRoute(r.Context(), chi.URLParam(r, "tenantId"), chi.URLParam(r, "routeId"))
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) updateRoute(w http.ResponseWriter, r *http.Request) {
	tenantID, id := chi.URLParam(r, "tenantId"), chi.URLParam(r, "routeId")
	item, err := h.store.GetRoute(r.Context(), tenantID, id)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	var req Route
	if !decode(w, r, &req) {
		return
	}
	patchRoute(&item, req)
	item.UpdatedAt = h.now()
	if err := h.store.UpdateRoute(r.Context(), item); err != nil {
		writeInternal(w, r, err)
		return
	}
	h.audit(r.Context(), tenantID, "route.update", "route", item.ID)
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) publishRoute(w http.ResponseWriter, r *http.Request) {
	h.setRouteStatus(w, r, StatusActive, "route.publish")
}

func (h *Handler) disableRoute(w http.ResponseWriter, r *http.Request) {
	h.setRouteStatus(w, r, StatusDisabled, "route.disable")
}

func (h *Handler) setRouteStatus(w http.ResponseWriter, r *http.Request, status string, action string) {
	tenantID, id := chi.URLParam(r, "tenantId"), chi.URLParam(r, "routeId")
	item, err := h.store.GetRoute(r.Context(), tenantID, id)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	item.Status = status
	item.UpdatedAt = h.now()
	if err := h.store.UpdateRoute(r.Context(), item); err != nil {
		writeInternal(w, r, err)
		return
	}
	h.audit(r.Context(), tenantID, action, "route", item.ID)
	writeJSON(w, http.StatusOK, item)
}

func patchRoute(item *Route, req Route) {
	if req.APIProductID != "" {
		item.APIProductID = req.APIProductID
	}
	if req.Name != "" {
		item.Name = req.Name
	}
	if req.InboundProtocol != "" {
		item.InboundProtocol = req.InboundProtocol
	}
	if req.OutboundProtocol != "" {
		item.OutboundProtocol = req.OutboundProtocol
	}
	if req.Host != "" {
		item.Host = req.Host
	}
	if req.Method != "" {
		item.Method = req.Method
	}
	if req.Path != "" {
		item.Path = req.Path
	}
	if req.UpstreamID != "" {
		item.UpstreamID = req.UpstreamID
	}
	if req.TransformationTemplateID != "" {
		item.TransformationTemplateID = req.TransformationTemplateID
	}
	if req.RateLimitPolicyID != "" {
		item.RateLimitPolicyID = req.RateLimitPolicyID
	}
	if req.QuotaPolicyID != "" {
		item.QuotaPolicyID = req.QuotaPolicyID
	}
	if req.Priority != 0 {
		item.Priority = req.Priority
	}
	if req.TimeoutMs != 0 {
		item.TimeoutMs = req.TimeoutMs
	}
	if req.RequiredScopes != nil {
		item.RequiredScopes = append([]string(nil), req.RequiredScopes...)
	}
	item.HMACEnabled = req.HMACEnabled || item.HMACEnabled
	if req.HMACSecret != "" {
		item.HMACSecret = req.HMACSecret
	}
	if req.ReplayWindowSec != 0 {
		item.ReplayWindowSec = req.ReplayWindowSec
	}
	item.IdempotencyEnabled = req.IdempotencyEnabled || item.IdempotencyEnabled
	if req.IdempotencyTTLSec != 0 {
		item.IdempotencyTTLSec = req.IdempotencyTTLSec
	}
	if req.Status != "" {
		item.Status = req.Status
	}
}

func (h *Handler) createConsumer(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantId")
	var req struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		OwnerUserID string `json:"ownerUserId"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Name == "" || req.Slug == "" {
		writeError(w, r, http.StatusBadRequest, "validation_error", "name and slug are required")
		return
	}
	now := h.now()
	item := Consumer{ID: ids.New(), TenantID: tenantID, Name: req.Name, Slug: req.Slug, OwnerUserID: req.OwnerUserID, Status: StatusActive, CreatedAt: now, UpdatedAt: now}
	if err := h.store.CreateConsumer(r.Context(), item); err != nil {
		writeInternal(w, r, err)
		return
	}
	h.audit(r.Context(), tenantID, "consumer.create", "consumer", item.ID)
	writeJSON(w, http.StatusCreated, item)
}

func (h *Handler) createCredential(w http.ResponseWriter, r *http.Request) {
	tenantID, consumerID := chi.URLParam(r, "tenantId"), chi.URLParam(r, "consumerId")
	if _, err := h.store.GetConsumer(r.Context(), tenantID, consumerID); err != nil {
		writeStoreError(w, r, err)
		return
	}
	var req struct {
		Type      string     `json:"type"`
		Scopes    []string   `json:"scopes"`
		ExpiresAt *time.Time `json:"expiresAt"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Type == "" {
		req.Type = "api_key"
	}
	if req.Type != "api_key" {
		writeError(w, r, http.StatusBadRequest, "validation_error", "only api_key credentials are supported")
		return
	}
	keyPrefix, secret, err := newAPIKeyParts()
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	secretHash, err := auth.HashSecret(secret)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	now := h.now()
	item := Credential{ID: ids.New(), TenantID: tenantID, ConsumerID: consumerID, Type: req.Type, KeyPrefix: keyPrefix, SecretHash: secretHash, Scopes: req.Scopes, Status: StatusActive, ExpiresAt: req.ExpiresAt, CreatedAt: now, UpdatedAt: now}
	if err := h.store.CreateCredential(r.Context(), item); err != nil {
		writeInternal(w, r, err)
		return
	}
	h.audit(r.Context(), tenantID, "credential.create", "credential", item.ID)
	writeJSON(w, http.StatusCreated, CredentialCreateResponse{ID: item.ID, Type: item.Type, KeyPrefix: item.KeyPrefix, APIKey: keyPrefix + "." + secret, Status: item.Status, ExpiresAt: item.ExpiresAt})
}

func (h *Handler) rotateCredential(w http.ResponseWriter, r *http.Request) {
	tenantID, id := chi.URLParam(r, "tenantId"), chi.URLParam(r, "credentialId")
	item, err := h.store.GetCredential(r.Context(), tenantID, id)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	keyPrefix, secret, err := newAPIKeyParts()
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	secretHash, err := auth.HashSecret(secret)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	item.KeyPrefix = keyPrefix
	item.SecretHash = secretHash
	item.Status = StatusActive
	item.UpdatedAt = h.now()
	if err := h.store.UpdateCredential(r.Context(), item); err != nil {
		writeInternal(w, r, err)
		return
	}
	h.audit(r.Context(), tenantID, "credential.rotate", "credential", item.ID)
	writeJSON(w, http.StatusOK, CredentialCreateResponse{ID: item.ID, Type: item.Type, KeyPrefix: item.KeyPrefix, APIKey: keyPrefix + "." + secret, Status: item.Status, ExpiresAt: item.ExpiresAt})
}

func (h *Handler) revokeCredential(w http.ResponseWriter, r *http.Request) {
	tenantID, id := chi.URLParam(r, "tenantId"), chi.URLParam(r, "credentialId")
	item, err := h.store.GetCredential(r.Context(), tenantID, id)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	item.Status = StatusRevoked
	item.UpdatedAt = h.now()
	if err := h.store.UpdateCredential(r.Context(), item); err != nil {
		writeInternal(w, r, err)
		return
	}
	h.audit(r.Context(), tenantID, "credential.revoke", "credential", item.ID)
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) createTemplate(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantId")
	var req struct {
		APIProductID   string          `json:"apiProductId"`
		Name           string          `json:"name"`
		SourceProtocol string          `json:"sourceProtocol"`
		TargetProtocol string          `json:"targetProtocol"`
		TemplateBody   json.RawMessage `json:"templateBody"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.APIProductID == "" || req.Name == "" || req.SourceProtocol == "" || req.TargetProtocol == "" || len(req.TemplateBody) == 0 {
		writeError(w, r, http.StatusBadRequest, "validation_error", "apiProductId, name, protocols, and templateBody are required")
		return
	}
	now := h.now()
	item := TransformationTemplate{ID: ids.New(), TenantID: tenantID, APIProductID: req.APIProductID, Name: req.Name, SourceProtocol: req.SourceProtocol, TargetProtocol: req.TargetProtocol, TemplateBody: req.TemplateBody, Version: 1, Status: StatusDraft, CreatedAt: now, UpdatedAt: now}
	if err := h.store.CreateTemplate(r.Context(), item); err != nil {
		writeInternal(w, r, err)
		return
	}
	h.audit(r.Context(), tenantID, "transformation_template.create", "transformation_template", item.ID)
	writeJSON(w, http.StatusCreated, item)
}

func (h *Handler) listTemplates(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListTemplates(r.Context(), chi.URLParam(r, "tenantId"))
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[TransformationTemplate]{Data: items})
}

func (h *Handler) publishTemplate(w http.ResponseWriter, r *http.Request) {
	tenantID, id := chi.URLParam(r, "tenantId"), chi.URLParam(r, "templateId")
	item, err := h.store.GetTemplate(r.Context(), tenantID, id)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	now := h.now()
	item.Status = "published"
	item.PublishedAt = &now
	item.UpdatedAt = now
	if err := h.store.UpdateTemplate(r.Context(), item); err != nil {
		writeInternal(w, r, err)
		return
	}
	h.audit(r.Context(), tenantID, "transformation_template.publish", "transformation_template", item.ID)
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) audit(ctx context.Context, tenantID string, action string, resource string, resourceID string) {
	event := AuditEvent{ID: ids.New(), ActorID: actorFromContext(ctx), TenantID: tenantID, Action: action, Resource: resource, ResourceID: resourceID, OccurredAt: h.now()}
	_ = h.store.AppendAudit(context.WithoutCancel(ctx), event)
}

func newAPIKeyParts() (string, string, error) {
	prefixBytes := make([]byte, 9)
	secretBytes := make([]byte, 24)
	if _, err := rand.Read(prefixBytes); err != nil {
		return "", "", err
	}
	if _, err := rand.Read(secretBytes); err != nil {
		return "", "", err
	}
	prefix := "gw_live_" + base64.RawURLEncoding.EncodeToString(prefixBytes)
	secret := base64.RawURLEncoding.EncodeToString(secretBytes)
	return prefix, secret, nil
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, r, http.StatusBadRequest, "validation_error", "Invalid request body")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeStoreError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "not_found", "Resource not found")
		return
	}
	writeInternal(w, r, err)
}

func writeInternal(w http.ResponseWriter, r *http.Request, err error) {
	writeError(w, r, http.StatusInternalServerError, "internal_error", "Internal server error")
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code string, message string) {
	requestID := middleware.GetReqID(r.Context())
	body := map[string]any{
		"error": map[string]any{
			"code":      code,
			"message":   message,
			"requestId": requestID,
		},
	}
	writeJSON(w, status, body)
}
