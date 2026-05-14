package controlplane

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"backend/internal/auth"
	"backend/internal/billing"
	"backend/pkg/ids"
)

type RouterConfig struct {
	AdminToken         string
	AdminAuthenticator AdminAuthenticator
	Store              Repository
	UsageEvents        billing.UsageEventStore
	Now                func() time.Time
}

type Handler struct {
	store              Repository
	usageEvents        billing.UsageEventStore
	adminAuthenticator AdminAuthenticator
	now                func() time.Time
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
	if cfg.AdminAuthenticator == nil {
		cfg.AdminAuthenticator = StaticAdminAuthenticator{BootstrapToken: cfg.AdminToken}
	}
	h := &Handler{store: cfg.Store, usageEvents: cfg.UsageEvents, adminAuthenticator: cfg.AdminAuthenticator, now: cfg.Now}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(h.adminAuth)
	r.Route("/admin/v1", func(r chi.Router) {
		r.With(h.requirePlatformAdmin).Post("/billing-plans", h.createBillingPlan)
		r.With(h.requirePlatformAdmin).Get("/billing-plans", h.listBillingPlans)
		r.With(h.requirePlatformAdmin).Patch("/billing-plans/{billingPlanId}", h.updateBillingPlan)
		r.With(h.requirePlatformAdmin).Post("/tenants", h.createTenant)
		r.With(h.requirePlatformAdmin).Get("/tenants", h.listTenants)
		r.With(h.requirePlatformAdmin).Get("/audit-logs", h.listAuditLogs)
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

			r.Post("/rate-limit-policies", h.createRateLimitPolicy)
			r.Get("/rate-limit-policies", h.listRateLimitPolicies)
			r.Patch("/rate-limit-policies/{policyId}", h.updateRateLimitPolicy)
			r.Post("/quota-policies", h.createQuotaPolicy)
			r.Get("/quota-policies", h.listQuotaPolicies)
			r.Patch("/quota-policies/{policyId}", h.updateQuotaPolicy)

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

			r.Get("/usage", h.listUsage)
			r.Get("/billing-summaries/{billingPeriod}", h.getBillingSummary)
			r.Post("/billing-summaries/{billingPeriod}/recalculate", h.recalculateBillingSummary)
			r.Post("/billing-summaries/{billingPeriod}/finalize", h.finalizeBillingSummary)
			r.Get("/billing-summaries/{billingPeriod}/export", h.exportBillingSummary)
			r.Get("/audit-logs", h.listAuditLogs)
		})
	})
	return r
}

func (h *Handler) adminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, err := h.adminAuthenticator.Authenticate(r)
		if err != nil {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", "Admin authentication required")
			return
		}
		next.ServeHTTP(w, r.WithContext(contextWithAdminIdentity(r.Context(), identity)))
	})
}

func (h *Handler) requireTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := adminIdentityFromContext(r.Context())
		if !ok {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", "Admin authentication required")
			return
		}
		tenantID := chi.URLParam(r, "tenantId")
		if identity.Role == RoleTenantAdmin && identity.TenantID != tenantID {
			writeError(w, r, http.StatusForbidden, "forbidden", "Tenant admin cannot access another tenant")
			return
		}
		if _, err := h.store.GetTenant(r.Context(), chi.URLParam(r, "tenantId")); err != nil {
			writeStoreError(w, r, err)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func actorFromContext(ctx context.Context) string {
	identity, ok := adminIdentityFromContext(ctx)
	if !ok || identity.ActorID == "" {
		return "unknown"
	}
	return identity.ActorID
}

func (h *Handler) requirePlatformAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := adminIdentityFromContext(r.Context())
		if !ok || identity.Role != RolePlatformAdmin {
			writeError(w, r, http.StatusForbidden, "forbidden", "Platform admin required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) canAccessTenant(ctx context.Context, tenantID string) bool {
	identity, ok := adminIdentityFromContext(ctx)
	if !ok {
		return false
	}
	if identity.Role == RolePlatformAdmin {
		return true
	}
	return identity.Role == RoleTenantAdmin && identity.TenantID == tenantID
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
	tenantID := chi.URLParam(r, "tenantId")
	if !h.canAccessTenant(r.Context(), tenantID) {
		writeError(w, r, http.StatusForbidden, "forbidden", "Tenant admin cannot access another tenant")
		return
	}
	tenant, err := h.store.GetTenant(r.Context(), tenantID)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, tenant)
}

func (h *Handler) updateTenant(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantId")
	if !h.canAccessTenant(r.Context(), tenantID) {
		writeError(w, r, http.StatusForbidden, "forbidden", "Tenant admin cannot access another tenant")
		return
	}
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
		Name              string `json:"name"`
		Slug              string `json:"slug"`
		Description       string `json:"description"`
		RateLimitPolicyID string `json:"rateLimitPolicyId"`
		QuotaPolicyID     string `json:"quotaPolicyId"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Name == "" || req.Slug == "" {
		writeError(w, r, http.StatusBadRequest, "validation_error", "name and slug are required")
		return
	}
	now := h.now()
	product := APIProduct{ID: ids.New(), TenantID: tenantID, Name: req.Name, Slug: req.Slug, Description: req.Description, RateLimitPolicyID: req.RateLimitPolicyID, QuotaPolicyID: req.QuotaPolicyID, Status: StatusActive, CreatedAt: now, UpdatedAt: now}
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
		Name              *string `json:"name"`
		Description       *string `json:"description"`
		RateLimitPolicyID *string `json:"rateLimitPolicyId"`
		QuotaPolicyID     *string `json:"quotaPolicyId"`
		Status            *string `json:"status"`
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
	if req.RateLimitPolicyID != nil {
		item.RateLimitPolicyID = *req.RateLimitPolicyID
	}
	if req.QuotaPolicyID != nil {
		item.QuotaPolicyID = *req.QuotaPolicyID
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
	if err := validateUpstreamConfig(req.Protocol, req.Config); err != nil {
		writeError(w, r, http.StatusBadRequest, "validation_error", err.Error())
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
	if err := validateUpstreamConfig(item.Protocol, item.Config); err != nil {
		writeError(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	item.UpdatedAt = h.now()
	if err := h.store.UpdateUpstream(r.Context(), item); err != nil {
		writeInternal(w, r, err)
		return
	}
	h.audit(r.Context(), tenantID, "upstream.update", "upstream", item.ID)
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) createRateLimitPolicy(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantId")
	var req RateLimitPolicy
	if !decode(w, r, &req) {
		return
	}
	if req.Name == "" || req.Scope == "" || req.LimitCount <= 0 || req.WindowSeconds <= 0 {
		writeError(w, r, http.StatusBadRequest, "validation_error", "name, scope, limitCount, and windowSeconds are required")
		return
	}
	if req.Status == "" {
		req.Status = StatusActive
	}
	if req.Algorithm == "" {
		req.Algorithm = "fixed_window"
	}
	now := h.now()
	req.ID = ids.New()
	req.TenantID = tenantID
	req.CreatedAt = now
	req.UpdatedAt = now
	if err := h.store.CreateRateLimitPolicy(r.Context(), req); err != nil {
		writeInternal(w, r, err)
		return
	}
	h.audit(r.Context(), tenantID, "rate_limit_policy.create", "rate_limit_policy", req.ID)
	writeJSON(w, http.StatusCreated, req)
}

func (h *Handler) listRateLimitPolicies(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListRateLimitPolicies(r.Context(), chi.URLParam(r, "tenantId"))
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[RateLimitPolicy]{Data: items})
}

func (h *Handler) updateRateLimitPolicy(w http.ResponseWriter, r *http.Request) {
	tenantID, id := chi.URLParam(r, "tenantId"), chi.URLParam(r, "policyId")
	item, err := h.store.GetRateLimitPolicy(r.Context(), tenantID, id)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	var req RateLimitPolicy
	if !decode(w, r, &req) {
		return
	}
	if req.Name != "" {
		item.Name = req.Name
	}
	if req.Scope != "" {
		item.Scope = req.Scope
	}
	if req.LimitCount > 0 {
		item.LimitCount = req.LimitCount
	}
	if req.WindowSeconds > 0 {
		item.WindowSeconds = req.WindowSeconds
	}
	if req.BurstCount != 0 {
		item.BurstCount = req.BurstCount
	}
	if req.Algorithm != "" {
		item.Algorithm = req.Algorithm
	}
	if req.Status != "" {
		item.Status = req.Status
	}
	item.UpdatedAt = h.now()
	if err := h.store.UpdateRateLimitPolicy(r.Context(), item); err != nil {
		writeInternal(w, r, err)
		return
	}
	h.audit(r.Context(), tenantID, "rate_limit_policy.update", "rate_limit_policy", item.ID)
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) createQuotaPolicy(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantId")
	var req QuotaPolicy
	if !decode(w, r, &req) {
		return
	}
	if req.Name == "" || req.Scope == "" || req.Period == "" || req.QuotaCount <= 0 || req.ExceededBehavior == "" {
		writeError(w, r, http.StatusBadRequest, "validation_error", "name, scope, period, quotaCount, and exceededBehavior are required")
		return
	}
	if req.Status == "" {
		req.Status = StatusActive
	}
	now := h.now()
	req.ID = ids.New()
	req.TenantID = tenantID
	req.CreatedAt = now
	req.UpdatedAt = now
	if err := h.store.CreateQuotaPolicy(r.Context(), req); err != nil {
		writeInternal(w, r, err)
		return
	}
	h.audit(r.Context(), tenantID, "quota_policy.create", "quota_policy", req.ID)
	writeJSON(w, http.StatusCreated, req)
}

func (h *Handler) listQuotaPolicies(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListQuotaPolicies(r.Context(), chi.URLParam(r, "tenantId"))
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[QuotaPolicy]{Data: items})
}

func (h *Handler) updateQuotaPolicy(w http.ResponseWriter, r *http.Request) {
	tenantID, id := chi.URLParam(r, "tenantId"), chi.URLParam(r, "policyId")
	item, err := h.store.GetQuotaPolicy(r.Context(), tenantID, id)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	var req QuotaPolicy
	if !decode(w, r, &req) {
		return
	}
	if req.Name != "" {
		item.Name = req.Name
	}
	if req.Scope != "" {
		item.Scope = req.Scope
	}
	if req.Period != "" {
		item.Period = req.Period
	}
	if req.QuotaCount > 0 {
		item.QuotaCount = req.QuotaCount
	}
	if req.ExceededBehavior != "" {
		item.ExceededBehavior = req.ExceededBehavior
	}
	if req.Status != "" {
		item.Status = req.Status
	}
	item.UpdatedAt = h.now()
	if err := h.store.UpdateQuotaPolicy(r.Context(), item); err != nil {
		writeInternal(w, r, err)
		return
	}
	h.audit(r.Context(), tenantID, "quota_policy.update", "quota_policy", item.ID)
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
	if err := validateRouteProtocols(req.InboundProtocol, req.OutboundProtocol); err != nil {
		writeError(w, r, http.StatusBadRequest, "validation_error", err.Error())
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
	if err := validateRouteProtocols(item.InboundProtocol, item.OutboundProtocol); err != nil {
		writeError(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
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
	if status == StatusActive {
		if err := validateRouteProtocols(item.InboundProtocol, item.OutboundProtocol); err != nil {
			writeError(w, r, http.StatusBadRequest, "validation_error", err.Error())
			return
		}
		up, err := h.store.GetUpstream(r.Context(), tenantID, item.UpstreamID)
		if err != nil {
			writeStoreError(w, r, err)
			return
		}
		if err := validateRouteUpstreamCompatibility(item, up); err != nil {
			writeError(w, r, http.StatusBadRequest, "validation_error", err.Error())
			return
		}
	}
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

func (h *Handler) createBillingPlan(w http.ResponseWriter, r *http.Request) {
	var req billing.BillingPlan
	if !decode(w, r, &req) {
		return
	}
	if req.Name == "" || req.Slug == "" || req.Currency == "" {
		writeError(w, r, http.StatusBadRequest, "validation_error", "name, slug, and currency are required")
		return
	}
	if req.Status == "" {
		req.Status = billing.PlanStatusActive
	}
	req.ID = ids.New()
	if err := h.store.CreateBillingPlan(r.Context(), req); err != nil {
		writeInternal(w, r, err)
		return
	}
	h.audit(r.Context(), "", "billing_plan.create", "billing_plan", req.ID)
	writeJSON(w, http.StatusCreated, req)
}

func (h *Handler) listBillingPlans(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListBillingPlans(r.Context())
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[billing.BillingPlan]{Data: items})
}

func (h *Handler) updateBillingPlan(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "billingPlanId")
	plan, err := h.store.GetBillingPlan(r.Context(), id)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	var req struct {
		Name             *string  `json:"name"`
		Slug             *string  `json:"slug"`
		MonthlyFee       *float64 `json:"monthlyFee"`
		IncludedRequests *int64   `json:"includedRequests"`
		OveragePrice     *float64 `json:"overagePrice"`
		Currency         *string  `json:"currency"`
		Status           *string  `json:"status"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Name != nil {
		plan.Name = *req.Name
	}
	if req.Slug != nil {
		plan.Slug = *req.Slug
	}
	if req.MonthlyFee != nil {
		plan.MonthlyFee = *req.MonthlyFee
	}
	if req.IncludedRequests != nil {
		plan.IncludedRequests = *req.IncludedRequests
	}
	if req.OveragePrice != nil {
		plan.OveragePrice = *req.OveragePrice
	}
	if req.Currency != nil {
		plan.Currency = *req.Currency
	}
	if req.Status != nil {
		plan.Status = *req.Status
	}
	if err := h.store.UpdateBillingPlan(r.Context(), plan); err != nil {
		writeInternal(w, r, err)
		return
	}
	h.audit(r.Context(), "", "billing_plan.update", "billing_plan", plan.ID)
	writeJSON(w, http.StatusOK, plan)
}

func (h *Handler) listUsage(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantId")
	if h.usageEvents == nil {
		writeJSON(w, http.StatusOK, listResponse[billing.UsageEvent]{Data: []billing.UsageEvent{}})
		return
	}
	filter := billing.UsageEventFilter{
		TenantID:       tenantID,
		RouteID:        r.URL.Query().Get("routeId"),
		ConsumerID:     r.URL.Query().Get("consumerId"),
		Status:         r.URL.Query().Get("status"),
		SourceProtocol: r.URL.Query().Get("sourceProtocol"),
		TargetProtocol: r.URL.Query().Get("targetProtocol"),
	}
	if billableRaw := r.URL.Query().Get("billable"); billableRaw != "" {
		parsed := billableRaw == "true"
		filter.Billable = &parsed
	}
	if fromRaw := r.URL.Query().Get("from"); fromRaw != "" {
		from, err := time.Parse(time.RFC3339, fromRaw)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "validation_error", "invalid from")
			return
		}
		filter.From = &from
	}
	if toRaw := r.URL.Query().Get("to"); toRaw != "" {
		to, err := time.Parse(time.RFC3339, toRaw)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "validation_error", "invalid to")
			return
		}
		filter.To = &to
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, r, http.StatusBadRequest, "validation_error", "invalid limit")
			return
		}
		limit = parsed
	}
	page, err := h.usageEvents.ListPage(r.Context(), filter, limit, r.URL.Query().Get("cursor"))
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[billing.UsageEvent]{Data: page.Data, NextCursor: page.NextCursor})
}

func (h *Handler) getBillingSummary(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantId")
	period := chi.URLParam(r, "billingPeriod")
	summary, err := h.store.GetBillingSummary(r.Context(), tenantID, period)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (h *Handler) recalculateBillingSummary(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantId")
	period := chi.URLParam(r, "billingPeriod")
	if h.usageEvents == nil {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "usage events store unavailable")
		return
	}
	tenant, err := h.store.GetTenant(r.Context(), tenantID)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	plan, err := h.store.GetBillingPlan(r.Context(), tenant.BillingPlanID)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	periodStart, periodEnd, err := parseBillingPeriod(period)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	agg := billing.NewAggregator(h.usageEvents)
	summary, err := agg.Summarize(r.Context(), tenantID, plan, periodStart, periodEnd)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	summary.BillingPeriod = period
	summary.Status = "draft"
	summary.CalculatedAt = h.now()
	if err := h.store.UpsertBillingSummary(r.Context(), summary); err != nil {
		writeInternal(w, r, err)
		return
	}
	h.audit(r.Context(), tenantID, "billing_summary.recalculate", "billing_summary", period)
	writeJSON(w, http.StatusOK, summary)
}

func (h *Handler) finalizeBillingSummary(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantId")
	period := chi.URLParam(r, "billingPeriod")
	summary, err := h.store.GetBillingSummary(r.Context(), tenantID, period)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	summary.Status = "finalized"
	summary.CalculatedAt = h.now()
	if err := h.store.UpsertBillingSummary(r.Context(), summary); err != nil {
		writeInternal(w, r, err)
		return
	}
	h.audit(r.Context(), tenantID, "billing_summary.finalize", "billing_summary", period)
	writeJSON(w, http.StatusOK, summary)
}

func (h *Handler) exportBillingSummary(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantId")
	period := chi.URLParam(r, "billingPeriod")
	format := strings.ToLower(r.URL.Query().Get("format"))
	if format == "" {
		format = "json"
	}
	summary, err := h.store.GetBillingSummary(r.Context(), tenantID, period)
	if err != nil {
		writeStoreError(w, r, err)
		return
	}
	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv")
		writer := csv.NewWriter(w)
		_ = writer.Write([]string{"tenant_id", "billing_period", "billing_plan", "request_count", "failure_count", "rejected_count", "timeout_count", "billable_count", "included_quota", "billable_overage", "monthly_fee", "overage_amount", "estimated_amount", "currency", "status", "calculated_at"})
		_ = writer.Write([]string{
			summary.TenantID, summary.BillingPeriod, summary.PlanID,
			strconv.FormatInt(summary.TotalRequests, 10),
			strconv.FormatInt(summary.FailedRequests, 10),
			strconv.FormatInt(summary.RejectedRequests, 10),
			strconv.FormatInt(summary.TimeoutRequests, 10),
			strconv.FormatInt(summary.BillableRequests, 10),
			strconv.FormatInt(summary.IncludedRequests, 10),
			strconv.FormatInt(summary.OverageRequests, 10),
			fmt.Sprintf("%.4f", summary.MonthlyFee),
			fmt.Sprintf("%.4f", summary.OverageAmount),
			fmt.Sprintf("%.4f", summary.EstimatedAmount),
			summary.Currency, summary.Status, summary.CalculatedAt.Format(time.RFC3339),
		})
		writer.Flush()
	case "json":
		writeJSON(w, http.StatusOK, map[string]any{
			"summary":     summary,
			"generatedAt": h.now(),
		})
	default:
		writeError(w, r, http.StatusBadRequest, "validation_error", "unsupported format")
		return
	}
	h.audit(r.Context(), tenantID, "billing_summary.export", "billing_summary", period)
}

func (h *Handler) listAuditLogs(w http.ResponseWriter, r *http.Request) {
	filter := AuditFilter{
		TenantID: chi.URLParam(r, "tenantId"),
		ActorID:  r.URL.Query().Get("actorId"),
		Action:   r.URL.Query().Get("action"),
		Resource: r.URL.Query().Get("resource"),
	}
	if fromRaw := r.URL.Query().Get("from"); fromRaw != "" {
		from, err := time.Parse(time.RFC3339, fromRaw)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "validation_error", "invalid from")
			return
		}
		filter.From = &from
	}
	if toRaw := r.URL.Query().Get("to"); toRaw != "" {
		to, err := time.Parse(time.RFC3339, toRaw)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "validation_error", "invalid to")
			return
		}
		filter.To = &to
	}
	identity, _ := adminIdentityFromContext(r.Context())
	if filter.TenantID == "" && identity.Role == RoleTenantAdmin {
		filter.TenantID = identity.TenantID
	}
	items, err := h.store.ListAuditEvents(r.Context(), filter)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[AuditEvent]{Data: items})
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

func parseBillingPeriod(period string) (time.Time, time.Time, error) {
	start, err := time.Parse("2006-01", period)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid billing period")
	}
	end := start.AddDate(0, 1, 0)
	return start.UTC(), end.UTC(), nil
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
