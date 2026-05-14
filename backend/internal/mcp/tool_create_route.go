package mcp

import (
	"net/http"

	"backend/internal/controlplane"
	"backend/pkg/ids"
)

func (s *server) createRoute(w http.ResponseWriter, r *http.Request) {
	identity, ok := IdentityFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "MCP identity missing")
		return
	}
	var req struct {
		TenantID         string `json:"tenantId"`
		APIProductID     string `json:"apiProductId"`
		Name             string `json:"name"`
		InboundProtocol  string `json:"inboundProtocol"`
		OutboundProtocol string `json:"outboundProtocol"`
		Host             string `json:"host"`
		Method           string `json:"method"`
		Path             string `json:"path"`
		UpstreamID       string `json:"upstreamId"`
		Priority         int    `json:"priority"`
		TimeoutMs        int    `json:"timeoutMs"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "Invalid request body")
		return
	}
	if req.TenantID == "" || req.APIProductID == "" || req.Name == "" || req.InboundProtocol == "" ||
		req.OutboundProtocol == "" || req.Host == "" || req.Method == "" || req.Path == "" || req.UpstreamID == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "tenantId, apiProductId, name, protocols, host, method, path, and upstreamId are required")
		return
	}
	if !s.canAccessTenant(identity, req.TenantID) {
		writeError(w, http.StatusForbidden, "forbidden", "Tenant access denied")
		return
	}
	if _, err := s.store.GetTenant(r.Context(), req.TenantID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Tenant not found")
		return
	}

	now := s.now()
	route := controlplane.Route{
		ID:                ids.New(),
		TenantID:          req.TenantID,
		APIProductID:      req.APIProductID,
		Name:              req.Name,
		InboundProtocol:   req.InboundProtocol,
		OutboundProtocol:  req.OutboundProtocol,
		Host:              req.Host,
		Method:            req.Method,
		Path:              req.Path,
		UpstreamID:        req.UpstreamID,
		Priority:          req.Priority,
		TimeoutMs:         req.TimeoutMs,
		Status:            controlplane.StatusDraft,
		CreatedAt:         now,
		UpdatedAt:         now,
		ReplayWindowSec:   300,
		IdempotencyTTLSec: 86400,
	}
	if err := s.store.CreateRoute(r.Context(), route); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create route")
		return
	}
	s.audit(r.Context(), identity, req.TenantID, "mcp.create_route", "route", route.ID)

	writeJSON(w, http.StatusCreated, ToolResponse{
		Tool:   "createRoute",
		Result: route,
	})
}
