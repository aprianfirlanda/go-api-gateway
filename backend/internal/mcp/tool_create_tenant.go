package mcp

import (
	"net/http"

	"backend/internal/controlplane"
	"backend/pkg/ids"
)

func (s *server) createTenant(w http.ResponseWriter, r *http.Request) {
	identity, ok := IdentityFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "MCP identity missing")
		return
	}
	if identity.Role != "platform_admin" {
		writeError(w, http.StatusForbidden, "forbidden", "Platform admin required")
		return
	}
	var req struct {
		Name          string         `json:"name"`
		Slug          string         `json:"slug"`
		BillingPlanID string         `json:"billingPlanId"`
		Metadata      map[string]any `json:"metadata"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "Invalid request body")
		return
	}
	if req.Name == "" || req.Slug == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "name and slug are required")
		return
	}

	now := s.now()
	tenant := controlplane.Tenant{
		ID:            ids.New(),
		Name:          req.Name,
		Slug:          req.Slug,
		Status:        controlplane.StatusActive,
		BillingPlanID: req.BillingPlanID,
		Metadata:      req.Metadata,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.store.CreateTenant(r.Context(), tenant); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create tenant")
		return
	}
	s.audit(r.Context(), identity, tenant.ID, "mcp.create_tenant", "tenant", tenant.ID)

	writeJSON(w, http.StatusCreated, ToolResponse{
		Tool:   "createTenant",
		Result: tenant,
	})
}
