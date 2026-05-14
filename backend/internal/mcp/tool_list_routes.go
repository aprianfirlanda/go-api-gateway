package mcp

import (
	"net/http"
)

func (s *server) listRoutes(w http.ResponseWriter, r *http.Request) {
	identity, ok := IdentityFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "MCP identity missing")
		return
	}
	tenantID := r.URL.Query().Get("tenantId")
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "tenantId is required")
		return
	}
	if !s.canAccessTenant(identity, tenantID) {
		writeError(w, http.StatusForbidden, "forbidden", "Tenant access denied")
		return
	}
	if _, err := s.store.GetTenant(r.Context(), tenantID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Tenant not found")
		return
	}

	items, err := s.store.ListRoutes(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list routes")
		return
	}
	s.audit(r.Context(), identity, tenantID, "mcp.list_routes", "route", "")
	writeJSON(w, http.StatusOK, ToolResponse{
		Tool:   "listRoutes",
		Result: items,
	})
}
